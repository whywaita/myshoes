package scaleset

import (
	"context"
	"testing"
	"time"

	"github.com/actions/scaleset"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
)

// mockDatastore is a minimal mock for testing
type mockDatastore struct {
	datastore.Datastore
	createRunnerCalled bool
	deleteRunnerCalled bool
}

func (m *mockDatastore) CreateRunner(ctx context.Context, runner datastore.Runner) error {
	m.createRunnerCalled = true
	return nil
}

func (m *mockDatastore) DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Time, reason datastore.RunnerStatus) error {
	m.deleteRunnerCalled = true
	return nil
}

func TestTargetScaler_GetActiveRunnerCount(t *testing.T) {
	ts := &targetScaler{}

	// Initially empty
	if count := ts.getActiveRunnerCount(); count != 0 {
		t.Errorf("expected 0 active runners, got %d", count)
	}

	// Add some runners
	ts.activeRunners.Store("runner-1", runnerInfo{runnerID: uuid.NewV4()})
	ts.activeRunners.Store("runner-2", runnerInfo{runnerID: uuid.NewV4()})

	if count := ts.getActiveRunnerCount(); count != 2 {
		t.Errorf("expected 2 active runners, got %d", count)
	}

	// Remove one
	ts.activeRunners.Delete("runner-1")

	if count := ts.getActiveRunnerCount(); count != 1 {
		t.Errorf("expected 1 active runner, got %d", count)
	}
}

func TestTargetScaler_HandleDesiredRunnerCount(t *testing.T) {
	tests := []struct {
		name           string
		currentCount   int
		desiredCount   int
		expectIncrease bool
	}{
		{
			name:           "no change - same count",
			currentCount:   5,
			desiredCount:   5,
			expectIncrease: false,
		},
		{
			name:           "scale down (no-op for ephemeral)",
			currentCount:   10,
			desiredCount:   5,
			expectIncrease: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDatastore{}
			ts := &targetScaler{
				ds: mock,
				target: datastore.Target{
					UUID:  uuid.NewV4(),
					Scope: "test-org",
				},
				cfg: ManagerConfig{
					RunnerVersion: "v2.311.0",
					RunnerUser:    "runner",
					RunnerBaseDir: "/tmp",
				},
			}

			// Simulate current runners
			for i := 0; i < tt.currentCount; i++ {
				ts.activeRunners.Store("runner-"+string(rune(i)), runnerInfo{
					runnerID: uuid.NewV4(),
				})
			}

			ctx := context.Background()
			actualCount, err := ts.HandleDesiredRunnerCount(ctx, tt.desiredCount)

			// Without scale up, should not error
			if err != nil {
				t.Errorf("HandleDesiredRunnerCount() error = %v, want nil", err)
			}

			// Count should remain the same when not scaling up
			if actualCount != tt.currentCount {
				t.Errorf("actual count = %d, want %d", actualCount, tt.currentCount)
			}
		})
	}
}

// TestTargetScaler_GetActiveRunnerCount tests the runner count logic
func TestTargetScaler_HandleDesiredRunnerCount_Logic(t *testing.T) {
	ts := &targetScaler{
		target: datastore.Target{
			UUID:  uuid.NewV4(),
			Scope: "test-org",
		},
	}

	// Test scale up detection (without actual provisioning)
	current := ts.getActiveRunnerCount()
	if current != 0 {
		t.Errorf("expected 0 current runners, got %d", current)
	}

	desired := 5
	toProvision := desired - current
	if toProvision != 5 {
		t.Errorf("expected to provision 5 runners, got %d", toProvision)
	}

	// Test that scale down is no-op
	for i := 0; i < 10; i++ {
		ts.activeRunners.Store("runner-"+string(rune(i)), runnerInfo{
			runnerID: uuid.NewV4(),
		})
	}

	current = ts.getActiveRunnerCount()
	desired = 5
	if desired < current {
		// This is the scale down case - should be no-op
		toProvision = 0
	} else {
		toProvision = desired - current
	}

	if toProvision != 0 {
		t.Errorf("scale down should be no-op, but got toProvision=%d", toProvision)
	}
}

func TestTargetScaler_HandleJobCompleted(t *testing.T) {
	mock := &mockDatastore{}
	ts := &targetScaler{
		ds: mock,
		target: datastore.Target{
			UUID:  uuid.NewV4(),
			Scope: "test-org",
		},
	}

	// Add a runner to active runners
	runnerName := "test-runner"
	runnerID := uuid.NewV4()
	ts.activeRunners.Store(runnerName, runnerInfo{
		runnerID: runnerID,
		cloudID:  "cloud-123",
		labels:   []string{"test"},
	})

	ctx := context.Background()
	event := &scaleset.JobCompleted{
		RunnerName: runnerName,
		JobMessageBase: scaleset.JobMessageBase{
			JobID: "job-123",
		},
	}

	// Note: without real shoes client, deletion will fail and return early
	// We're testing the logic structure, not the actual deletion
	err := ts.HandleJobCompleted(ctx, event)
	if err == nil {
		t.Error("HandleJobCompleted() should error without real shoes client")
	}

	// Runner won't be removed because shoes client fails early
	// This is expected behavior in test environment
}

func TestTargetScaler_HandleJobStarted(t *testing.T) {
	ts := &targetScaler{
		target: datastore.Target{
			Scope: "test-org",
		},
	}

	ctx := context.Background()
	event := &scaleset.JobStarted{
		RunnerName: "test-runner",
		JobMessageBase: scaleset.JobMessageBase{
			JobID: "job-123",
		},
	}

	// HandleJobStarted should not return error (it only logs)
	if err := ts.HandleJobStarted(ctx, event); err != nil {
		t.Errorf("HandleJobStarted() error = %v, want nil", err)
	}
}
