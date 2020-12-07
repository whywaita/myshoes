package starter

import (
	"context"
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/logger"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/shoes"
	"github.com/whywaita/myshoes/pkg/starter/safety"
)

var (
	PistolInterval = 10 * time.Second
)

// Starter is dispatcher for running job
type Starter struct {
	ds     datastore.Datastore
	safety safety.Safety
}

func New(ds datastore.Datastore, s safety.Safety) *Starter {
	return &Starter{
		ds:     ds,
		safety: s,
	}
}

func (s *Starter) Loop() error {
	logger.Logf("start starter loop")

	ctx := context.Background()
	ticker := time.NewTicker(PistolInterval)

	for {
		select {
		case <-ticker.C:
			if err := s.do(ctx); err != nil {
				logger.Logf("%+v", err)
			}
		}
	}
}

func (s *Starter) do(ctx context.Context) error {
	jobs, err := s.ds.GetJob(ctx)
	if err != nil {
		return fmt.Errorf("failed to get jobs: %w", err)
	}

	for _, j := range jobs {
		logger.Logf("start job (job id: %s)\n", j.UUID.String())

		isOK, err := s.safety.Check(&j)
		if err != nil {
			logger.Logf("failed to check safery: %+v\n", err)
			continue
		}
		if !isOK {
			// is not ok, save job
			continue
		}

		if err := s.bung(ctx, j); err != nil {
			logger.Logf("failed to bung: %+v\n", err)
			continue
		}
		if err := s.ds.DeleteJob(ctx, j.UUID); err != nil {
			logger.Logf("failed to delete job: %+v\n", err)
			continue
		}
	}

	return nil
}

// bung is start runner, like a pistol! :)
func (s *Starter) bung(ctx context.Context, job datastore.Job) error {
	client, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}

	target, err := s.ds.GetTarget(ctx, job.TargetID)
	if err != nil {
		return fmt.Errorf("failed to retrieve relational target (job: %s, target: %s): %w", job.UUID, job.TargetID, err)
	}

	script, err := s.getSetupScript(ctx, *target)
	if err != nil {
		return fmt.Errorf("failed to get setup script: %w", err)
	}

	cloudID, ipAddress, shoesType, err := client.AddInstance(ctx, job.UUID.String(), script)
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}
	teardown()

	runnerID := uuid.NewV4()
	now := time.Now()
	r := datastore.Runner{
		UUID:      runnerID,
		ShoesType: shoesType,
		IPAddress: ipAddress,
		TargetID:  target.UUID,
		CloudID:   cloudID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.ds.CreateRunner(ctx, r); err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	return nil
}
