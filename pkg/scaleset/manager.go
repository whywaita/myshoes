package scaleset

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

// Manager manages scale set lifecycle for all targets
type Manager struct {
	ds      datastore.Datastore
	cfg     ManagerConfig
	scalers map[uuid.UUID]*targetScalerWrapper
	mu      sync.RWMutex
}

// ManagerConfig contains configuration for scale set manager
type ManagerConfig struct {
	AppID           int64
	PrivateKeyPEM   []byte
	GitHubURL       string
	RunnerGroupName string
	MaxRunners      int
	ScaleSetPrefix  string
	RunnerVersion   string
	RunnerUser      string
	RunnerBaseDir   string
}

type targetScalerWrapper struct {
	scaler     *targetScaler
	cancelFunc context.CancelFunc
}

// New creates a new scale set manager
func New(ds datastore.Datastore, cfg ManagerConfig) *Manager {
	return &Manager{
		ds:      ds,
		cfg:     cfg,
		scalers: make(map[uuid.UUID]*targetScalerWrapper),
	}
}

// Loop periodically syncs targets and manages scale set listeners
func (m *Manager) Loop(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial sync
	if err := m.syncTargets(ctx); err != nil {
		logger.Logf(false, "[scaleset] initial sync failed: %+v", err)
	}

	for {
		select {
		case <-ctx.Done():
			logger.Logf(false, "[scaleset] manager loop stopped")
			m.stopAllListeners()
			return ctx.Err()
		case <-ticker.C:
			if err := m.syncTargets(ctx); err != nil {
				logger.Logf(false, "[scaleset] sync failed: %+v", err)
			}
		}
	}
}

func (m *Manager) syncTargets(ctx context.Context) error {
	targets, err := datastore.ListTargets(ctx, m.ds)
	if err != nil {
		return fmt.Errorf("failed to list targets: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Track active target IDs
	activeTargets := make(map[uuid.UUID]bool)

	for _, target := range targets {
		activeTargets[target.UUID] = true

		// Skip if listener already running
		if _, exists := m.scalers[target.UUID]; exists {
			continue
		}

		// Start new listener
		if err := m.startListener(ctx, target); err != nil {
			logger.Logf(false, "[scaleset] failed to start listener for target %s: %+v", target.Scope, err)
			continue
		}
	}

	// Stop listeners for deleted targets
	for targetID, wrapper := range m.scalers {
		if !activeTargets[targetID] {
			logger.Logf(false, "[scaleset] stopping listener for deleted target %s", targetID)
			wrapper.cancelFunc()
			delete(m.scalers, targetID)
		}
	}

	return nil
}

func (m *Manager) startListener(ctx context.Context, target datastore.Target) error {
	logger.Logf(false, "[scaleset] starting listener for target: %s", target.Scope)

	// Resolve installation ID
	installationID, err := gh.IsInstalledGitHubApp(ctx, target.Scope)
	if err != nil {
		return fmt.Errorf("failed to get installation ID: %w", err)
	}

	// Create scaleset client
	client, err := scaleset.NewClientWithGitHubApp(
		scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: fmt.Sprintf("%s/%s", m.cfg.GitHubURL, target.Scope),
			GitHubAppAuth: scaleset.GitHubAppAuth{
				ClientID:       strconv.FormatInt(m.cfg.AppID, 10),
				InstallationID: installationID,
				PrivateKey:     string(m.cfg.PrivateKeyPEM),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create scaleset client: %w", err)
	}

	// Get or create scale set
	scaleSetName := sanitizeScaleSetName(m.cfg.ScaleSetPrefix, target.Scope)
	scaleSetID, err := m.ensureScaleSet(ctx, client, scaleSetName, target)
	if err != nil {
		return fmt.Errorf("failed to ensure scale set: %w", err)
	}

	// Create scaler
	scaler := &targetScaler{
		ds:         m.ds,
		target:     target,
		client:     client,
		scaleSetID: scaleSetID,
		cfg:        m.cfg,
	}

	// Create message session
	session, err := client.MessageSessionClient(ctx, scaleSetID, "myshoes-manager")
	if err != nil {
		return fmt.Errorf("failed to create message session: %w", err)
	}

	// Start listener in background
	listenerCtx, cancel := context.WithCancel(context.Background())
	wrapper := &targetScalerWrapper{
		scaler:     scaler,
		cancelFunc: cancel,
	}
	m.scalers[target.UUID] = wrapper

	go func() {
		logger.Logf(false, "[scaleset] listener started for target: %s", target.Scope)
		metricScaleSetListenerRunning.WithLabelValues(target.Scope).Set(1)

		listenerConfig := listener.Config{
			ScaleSetID: scaleSetID,
			MaxRunners: m.cfg.MaxRunners,
		}
		l, err := listener.New(session, listenerConfig)
		if err != nil {
			logger.Logf(false, "[scaleset] failed to create listener for target %s: %+v", target.Scope, err)
			return
		}

		if err := l.Run(listenerCtx, scaler); err != nil && listenerCtx.Err() == nil {
			logger.Logf(false, "[scaleset] listener error for target %s: %+v", target.Scope, err)
		}

		logger.Logf(false, "[scaleset] listener stopped for target: %s", target.Scope)
		metricScaleSetListenerRunning.WithLabelValues(target.Scope).Set(0)
	}()

	return nil
}

func (m *Manager) ensureScaleSet(ctx context.Context, client *scaleset.Client, name string, target datastore.Target) (int, error) {
	// Get runner group by name
	runnerGroup, err := client.GetRunnerGroupByName(ctx, m.cfg.RunnerGroupName)
	if err != nil {
		return 0, fmt.Errorf("failed to get runner group %s: %w", m.cfg.RunnerGroupName, err)
	}

	// Try to get existing scale set
	scaleSet, err := client.GetRunnerScaleSet(ctx, runnerGroup.ID, name)
	if err == nil && scaleSet != nil {
		logger.Logf(false, "[scaleset] found existing scale set: %s (id=%d)", name, scaleSet.ID)
		return scaleSet.ID, nil
	}

	// Create new scale set
	logger.Logf(false, "[scaleset] creating new scale set: %s", name)

	// Generate labels based on target scope
	var labels []scaleset.Label
	labels = append(labels, scaleset.Label{Type: "scaleset", Name: "scaleset"})
	labels = append(labels, scaleset.Label{Type: "scope", Name: target.Scope})

	scaleSet, err = client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
		Name:            name,
		RunnerGroupID:   runnerGroup.ID,
		RunnerGroupName: m.cfg.RunnerGroupName,
		Labels:          labels,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create scale set: %w", err)
	}

	logger.Logf(false, "[scaleset] created scale set: %s (id=%d)", name, scaleSet.ID)
	return scaleSet.ID, nil
}

func (m *Manager) stopAllListeners() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for targetID, wrapper := range m.scalers {
		logger.Logf(false, "[scaleset] stopping listener for target: %s", targetID)
		wrapper.cancelFunc()
	}
	m.scalers = make(map[uuid.UUID]*targetScalerWrapper)
}

// sanitizeScaleSetName creates a valid scale set name from prefix and scope
// Format: {prefix}-{sanitized-scope}
// Example: myshoes-myorg, myshoes-myorg-myrepo
func sanitizeScaleSetName(prefix, scope string) string {
	// Replace / and other invalid characters with -
	sanitized := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(scope, "-")
	sanitized = strings.Trim(sanitized, "-")
	return fmt.Sprintf("%s-%s", prefix, sanitized)
}
