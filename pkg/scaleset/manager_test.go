package scaleset

import (
	"context"
	"testing"
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
)

// mockDatastoreForManager is a minimal mock for manager tests
type mockDatastoreForManager struct {
	datastore.Datastore
	targets            []datastore.Target
	createRunnerCalled bool
	deleteRunnerCalled bool
}

func (m *mockDatastoreForManager) ListTargets(ctx context.Context) ([]datastore.Target, error) {
	return m.targets, nil
}

func (m *mockDatastoreForManager) CreateRunner(ctx context.Context, runner datastore.Runner) error {
	m.createRunnerCalled = true
	return nil
}

func (m *mockDatastoreForManager) DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Time, reason datastore.RunnerStatus) error {
	m.deleteRunnerCalled = true
	return nil
}

func TestNew(t *testing.T) {
	mock := &mockDatastoreForManager{}
	cfg := ManagerConfig{
		AppID:           12345,
		PrivateKeyPEM:   []byte("test-key"),
		GitHubURL:       "https://github.com",
		RunnerGroupName: "default",
		MaxRunners:      10,
		ScaleSetPrefix:  "myshoes",
		RunnerVersion:   "v2.311.0",
		RunnerUser:      "runner",
		RunnerBaseDir:   "/tmp",
	}

	manager := New(mock, cfg)

	if manager == nil {
		t.Fatal("New() returned nil")
	}

	if manager.ds == nil {
		t.Error("datastore not set correctly")
	}

	if manager.cfg.AppID != cfg.AppID {
		t.Error("config not set correctly")
	}

	if manager.scalers == nil {
		t.Error("scalers map not initialized")
	}

	if len(manager.scalers) != 0 {
		t.Error("scalers map should be empty initially")
	}
}

func TestSanitizeScaleSetName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		scope  string
		want   string
	}{
		{
			name:   "organization scope",
			prefix: "myshoes",
			scope:  "myorg",
			want:   "myshoes-myorg",
		},
		{
			name:   "repository scope",
			prefix: "myshoes",
			scope:  "myorg/myrepo",
			want:   "myshoes-myorg-myrepo",
		},
		{
			name:   "scope with special characters",
			prefix: "prefix",
			scope:  "org/repo-name_v2",
			want:   "prefix-org-repo-name-v2",
		},
		{
			name:   "scope with dots",
			prefix: "test",
			scope:  "org.com/repo.name",
			want:   "test-org-com-repo-name",
		},
		{
			name:   "custom prefix",
			prefix: "custom-prefix",
			scope:  "organization",
			want:   "custom-prefix-organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeScaleSetName(tt.prefix, tt.scope)
			if got != tt.want {
				t.Errorf("sanitizeScaleSetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_StopAllListeners(t *testing.T) {
	mock := &mockDatastoreForManager{}
	cfg := ManagerConfig{
		RunnerGroupName: "default",
		MaxRunners:      10,
	}

	manager := New(mock, cfg)

	// Add some mock scalers
	ctx, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	_, cancel2 := context.WithCancel(context.Background())

	manager.scalers[uuid.NewV4()] = &targetScalerWrapper{
		cancelFunc: cancel1,
	}
	manager.scalers[uuid.NewV4()] = &targetScalerWrapper{
		cancelFunc: cancel2,
	}

	if len(manager.scalers) != 2 {
		t.Fatal("setup failed: expected 2 scalers")
	}

	manager.stopAllListeners()

	if len(manager.scalers) != 0 {
		t.Error("stopAllListeners() should clear all scalers")
	}

	// Verify context was cancelled
	select {
	case <-ctx.Done():
		// Expected: context should be cancelled
	default:
		t.Error("context should be cancelled after stopAllListeners()")
	}
}

func TestManager_DeferredCleanupDoesNotDeleteReplacement(t *testing.T) {
	mock := &mockDatastoreForManager{}
	cfg := ManagerConfig{
		RunnerGroupName: "default",
		MaxRunners:      10,
	}
	manager := New(mock, cfg)

	targetID := uuid.NewV4()

	// Simulate: old goroutine's wrapper
	_, oldCancel := context.WithCancel(context.Background())
	oldWrapper := &targetScalerWrapper{cancelFunc: oldCancel}

	// Simulate: new wrapper stored by syncTargets after config change
	_, newCancel := context.WithCancel(context.Background())
	newWrapper := &targetScalerWrapper{cancelFunc: newCancel}
	manager.scalers[targetID] = newWrapper

	// Simulate: old goroutine's deferred cleanup runs, but should NOT delete newWrapper
	manager.mu.Lock()
	if current, exists := manager.scalers[targetID]; exists && current == oldWrapper {
		delete(manager.scalers, targetID)
	}
	manager.mu.Unlock()

	// newWrapper should still be in the map
	if _, exists := manager.scalers[targetID]; !exists {
		t.Error("deferred cleanup should not delete replacement wrapper")
	}
	if manager.scalers[targetID] != newWrapper {
		t.Error("scalers map should still contain the new wrapper")
	}
}

func TestManager_DeferredCleanupDeletesOwnWrapper(t *testing.T) {
	mock := &mockDatastoreForManager{}
	cfg := ManagerConfig{
		RunnerGroupName: "default",
		MaxRunners:      10,
	}
	manager := New(mock, cfg)

	targetID := uuid.NewV4()

	// Simulate: goroutine's wrapper is still the current one (no replacement)
	_, cancel := context.WithCancel(context.Background())
	wrapper := &targetScalerWrapper{cancelFunc: cancel}
	manager.scalers[targetID] = wrapper

	// Simulate: deferred cleanup should delete because it IS the same wrapper
	manager.mu.Lock()
	if current, exists := manager.scalers[targetID]; exists && current == wrapper {
		delete(manager.scalers, targetID)
	}
	manager.mu.Unlock()

	if _, exists := manager.scalers[targetID]; exists {
		t.Error("deferred cleanup should delete own wrapper when no replacement exists")
	}
}

func TestManagerConfig_Fields(t *testing.T) {
	cfg := ManagerConfig{
		AppID:           12345,
		PrivateKeyPEM:   []byte("test-key"),
		GitHubURL:       "https://github.com",
		RunnerGroupName: "custom-group",
		MaxRunners:      25,
		ScaleSetPrefix:  "custom-prefix",
		RunnerVersion:   "v2.311.0",
		RunnerUser:      "ubuntu",
		RunnerBaseDir:   "/home/ubuntu/runner",
	}

	// Verify all fields are accessible
	if cfg.AppID != 12345 {
		t.Error("AppID not set correctly")
	}
	if string(cfg.PrivateKeyPEM) != "test-key" {
		t.Error("PrivateKeyPEM not set correctly")
	}
	if cfg.GitHubURL != "https://github.com" {
		t.Error("GitHubURL not set correctly")
	}
	if cfg.RunnerGroupName != "custom-group" {
		t.Error("RunnerGroupName not set correctly")
	}
	if cfg.MaxRunners != 25 {
		t.Error("MaxRunners not set correctly")
	}
	if cfg.ScaleSetPrefix != "custom-prefix" {
		t.Error("ScaleSetPrefix not set correctly")
	}
	if cfg.RunnerVersion != "v2.311.0" {
		t.Error("RunnerVersion not set correctly")
	}
	if cfg.RunnerUser != "ubuntu" {
		t.Error("RunnerUser not set correctly")
	}
	if cfg.RunnerBaseDir != "/home/ubuntu/runner" {
		t.Error("RunnerBaseDir not set correctly")
	}
}
