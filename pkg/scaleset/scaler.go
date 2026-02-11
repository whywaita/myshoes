package scaleset

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/actions/scaleset"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/runner"
	"github.com/whywaita/myshoes/pkg/shoes"
)

type runnerInfo struct {
	runnerID  uuid.UUID
	cloudID   string
	labels    []string
	createdAt time.Time
}

// targetScaler implements listener.Scaler interface
type targetScaler struct {
	ds            datastore.Datastore
	target        datastore.Target
	client        *scaleset.Client
	scaleSetID    int
	cfg           ManagerConfig
	activeRunners sync.Map // runner name -> runnerInfo
}

// HandleDesiredRunnerCount is called when scale set controller wants to scale up/down
func (ts *targetScaler) HandleDesiredRunnerCount(ctx context.Context, count int) (int, error) {
	current := ts.getActiveRunnerCount()
	logger.Logf(false, "[scaleset] HandleDesiredRunnerCount: target=%s, desired=%d, current=%d", ts.target.Scope, count, current)

	metricScaleSetDesiredRunners.WithLabelValues(ts.target.Scope).Set(float64(count))

	if count > current {
		// Scale up
		toProvision := count - current
		for i := 0; i < toProvision; i++ {
			if err := ts.provisionRunner(ctx); err != nil {
				logger.Logf(false, "[scaleset] failed to provision runner: %+v", err)
				metricScaleSetProvisionErrorsTotal.WithLabelValues(ts.target.Scope).Inc()
				// Continue provisioning other runners even if one fails
			}
		}
	}
	// Scale down is handled by HandleJobCompleted (ephemeral runners terminate automatically)

	actualCount := ts.getActiveRunnerCount()
	metricScaleSetActiveRunners.WithLabelValues(ts.target.Scope).Set(float64(actualCount))

	return actualCount, nil
}

// HandleJobStarted is called when a job starts on a runner
func (ts *targetScaler) HandleJobStarted(ctx context.Context, event *scaleset.JobStarted) error {
	logger.Logf(false, "[scaleset] HandleJobStarted: target=%s, runner=%s, job=%s",
		ts.target.Scope, event.RunnerName, event.JobID)

	// Update metrics and log only
	return nil
}

// HandleJobCompleted is called when a job completes on a runner
func (ts *targetScaler) HandleJobCompleted(ctx context.Context, event *scaleset.JobCompleted) error {
	logger.Logf(false, "[scaleset] HandleJobCompleted: target=%s, runner=%s, job=%s",
		ts.target.Scope, event.RunnerName, event.JobID)

	metricScaleSetJobsCompletedTotal.WithLabelValues(ts.target.Scope).Inc()

	// Find runner info by name
	runnerInfoRaw, ok := ts.activeRunners.Load(event.RunnerName)
	if !ok {
		logger.Logf(false, "[scaleset] runner not found in active runners: %s", event.RunnerName)
		return nil
	}

	info := runnerInfoRaw.(runnerInfo)

	// Delete instance via shoes plugin
	shoesClient, cleanup, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get shoes client: %w", err)
	}
	defer cleanup()

	if err := shoesClient.DeleteInstance(ctx, info.cloudID, info.labels); err != nil {
		// Preserve runner state on deletion failure to allow retry
		// Without this, transient cloud/plugin errors leave orphaned instances
		logger.Logf(false, "[scaleset] failed to delete instance (preserving state for retry): %+v", err)
		return fmt.Errorf("failed to delete instance %s: %w", info.cloudID, err)
	}

	// Only update datastore and remove from tracking after successful deletion
	if err := ts.ds.DeleteRunner(ctx, info.runnerID, time.Now(), datastore.RunnerStatusCompleted); err != nil {
		logger.Logf(false, "[scaleset] failed to delete runner from datastore: %+v", err)
		// Continue even if datastore update fails - instance is already deleted
	}

	// Remove from active runners
	ts.activeRunners.Delete(event.RunnerName)

	// Update metrics
	actualCount := ts.getActiveRunnerCount()
	metricScaleSetActiveRunners.WithLabelValues(ts.target.Scope).Set(float64(actualCount))

	return nil
}

func (ts *targetScaler) provisionRunner(ctx context.Context) error {
	runnerID := uuid.NewV4()
	runnerName := runner.ToName(runnerID.String())

	logger.Logf(false, "[scaleset] provisioning runner: %s", runnerName)

	// Generate JIT config
	setting := &scaleset.RunnerScaleSetJitRunnerSetting{
		Name:       runnerName,
		WorkFolder: ts.cfg.RunnerBaseDir,
	}
	jitConfig, err := ts.client.GenerateJitRunnerConfig(ctx, setting, ts.scaleSetID)
	if err != nil {
		return fmt.Errorf("failed to generate JIT config: %w", err)
	}

	// Generate setup script with JIT config
	setupScript, err := GetJITSetupScript(
		jitConfig.EncodedJITConfig,
		ts.cfg.RunnerVersion,
		ts.cfg.RunnerUser,
		ts.cfg.RunnerBaseDir,
	)
	if err != nil {
		return fmt.Errorf("failed to generate JIT setup script: %w", err)
	}

	// Create labels based on target scope
	var labels []string
	labels = append(labels, "scaleset")
	labels = append(labels, ts.target.Scope)

	// Provision instance via shoes plugin
	shoesClient, cleanup, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get shoes client: %w", err)
	}
	defer cleanup()

	cloudID, ipAddress, shoesType, resourceType, err := shoesClient.AddInstance(
		ctx,
		runnerName,
		setupScript,
		ts.target.ResourceType,
		labels,
	)
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}

	// Store runner info in datastore
	r := datastore.Runner{
		UUID:          runnerID,
		ShoesType:     shoesType,
		IPAddress:     ipAddress,
		TargetID:      ts.target.UUID,
		CloudID:       cloudID,
		Deleted:       false,
		Status:        datastore.RunnerStatusCreated,
		ResourceType:  resourceType,
		RunnerUser:    sql.NullString{String: ts.cfg.RunnerUser, Valid: true},
		ProviderURL:   ts.target.ProviderURL,
		RepositoryURL: fmt.Sprintf("%s/%s", ts.cfg.GitHubURL, ts.target.Scope),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := ts.ds.CreateRunner(ctx, r); err != nil {
		logger.Logf(false, "[scaleset] failed to create runner in datastore: %+v", err)
	}

	// Store in active runners
	ts.activeRunners.Store(runnerName, runnerInfo{
		runnerID:  runnerID,
		cloudID:   cloudID,
		labels:    labels,
		createdAt: time.Now(),
	})

	logger.Logf(false, "[scaleset] runner provisioned: %s (cloud_id=%s)", runnerName, cloudID)

	return nil
}

func (ts *targetScaler) getActiveRunnerCount() int {
	count := 0
	ts.activeRunners.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
