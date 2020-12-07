package memory

import (
	"context"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
)

type Memory struct {
	mu      *sync.RWMutex
	targets map[uuid.UUID]datastore.Target
	jobs    map[uuid.UUID]datastore.Job
	runners map[uuid.UUID]datastore.Runner
}

// New create map
func New() (*Memory, error) {
	m := &sync.RWMutex{}
	t := map[uuid.UUID]datastore.Target{}
	j := map[uuid.UUID]datastore.Job{}
	r := map[uuid.UUID]datastore.Runner{}

	return &Memory{
		mu:      m,
		targets: t,
		jobs:    j,
		runners: r,
	}, nil
}

func (m *Memory) CreateTarget(ctx context.Context, target datastore.Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.targets[target.UUID] = target
	return nil
}

func (m *Memory) GetTarget(ctx context.Context, id uuid.UUID) (*datastore.Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.targets[id]
	if !ok {
		return nil, datastore.ErrNotFound
	}
	return &t, nil
}

func (m *Memory) GetTargetByScope(ctx context.Context, gheDomain, scope string) (*datastore.Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var inputValid bool
	if gheDomain == "" {
		inputValid = false
	} else {
		inputValid = true
	}

	for _, t := range m.targets {
		if t.Scope == scope {
			if t.GHEDomain.Valid == inputValid && t.GHEDomain.String == gheDomain {
				// found
				return &t, nil
			}
		}
	}

	return nil, datastore.ErrNotFound
}

func (m *Memory) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.targets, id)
	return nil
}

func (m *Memory) EnqueueJob(ctx context.Context, job datastore.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.jobs[job.UUID] = job
	return nil
}

func (m *Memory) GetJob(ctx context.Context) ([]datastore.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var jobs []datastore.Job

	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}

	return jobs, nil
}

func (m *Memory) DeleteJob(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.jobs, id)
	return nil
}

func (m *Memory) CreateRunner(ctx context.Context, runner datastore.Runner) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.runners[runner.UUID] = runner

	return nil
}

func (m *Memory) DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Ticker) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.runners, id)
	return nil
}
