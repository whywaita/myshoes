package starter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/whywaita/myshoes/pkg/runner"

	"github.com/whywaita/myshoes/pkg/logger"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/shoes"
	"github.com/whywaita/myshoes/pkg/starter/safety"
)

var (
	// PistolInterval is interval of bung (a.k.a. create instance)
	PistolInterval = 10 * time.Second
	// DefaultRunnerVersion is default value of actions/runner
	DefaultRunnerVersion = "v2.275.1"
)

// Starter is dispatcher for running job
type Starter struct {
	ds     datastore.Datastore
	safety safety.Safety
}

// New is create starter instance
func New(ds datastore.Datastore, s safety.Safety) *Starter {
	return &Starter{
		ds:     ds,
		safety: s,
	}
}

// Loop is main loop for starter
func (s *Starter) Loop(ctx context.Context) error {
	logger.Logf(false, "start starter loop")

	ticker := time.NewTicker(PistolInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.do(ctx); err != nil {
				logger.Logf(false, "failed to starter: %+v", err)
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Starter) do(ctx context.Context) error {
	logger.Logf(true, "start to check starter")
	jobs, err := s.ds.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get jobs: %w", err)
	}

	logger.Logf(true, "found %d jobs", len(jobs))
	wg := &sync.WaitGroup{}
	for _, j := range jobs {
		wg.Add(1)
		job := j

		go func() {
			defer wg.Done()
			logger.Logf(false, "start job (job id: %s)\n", job.UUID.String())

			isOK, err := s.safety.Check(&job)
			if err != nil {
				logger.Logf(false, "failed to check safety: %+v\n", err)
				return
			}
			if !isOK {
				// is not ok, save job
				return
			}
			if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusRunning, fmt.Sprintf("job id: %s", job.UUID)); err != nil {
				logger.Logf(false, "failed to update target status (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)
				return
			}

			if err := s.bung(ctx, job); err != nil {
				logger.Logf(false, "failed to bung: %+v\n", err)

				if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("job id: %s", job.UUID)); err != nil {
					logger.Logf(false, "failed to update target status (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)
					return
				}

				return
			}
			if err := s.ds.DeleteJob(ctx, job.UUID); err != nil {
				logger.Logf(false, "failed to delete job: %+v\n", err)

				if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("job id: %s", job.UUID)); err != nil {
					logger.Logf(false, "failed to update target status (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)
					return
				}

				return
			}
		}()
	}

	wg.Wait()

	return nil
}

// bung is start runner, like a pistol! :)
func (s *Starter) bung(ctx context.Context, job datastore.Job) error {
	logger.Logf(false, "start create instance (job: %s)", job.UUID)
	client, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	target, err := s.ds.GetTarget(ctx, job.TargetID)
	if err != nil {
		return fmt.Errorf("failed to retrieve relational target (job: %s, target: %s): %w", job.UUID, job.TargetID, err)
	}

	script, err := s.getSetupScript(*target)
	if err != nil {
		return fmt.Errorf("failed to get setup scripts: %w", err)
	}

	runnerName := runner.ToName(job.UUID.String())
	cloudID, ipAddress, shoesType, err := client.AddInstance(ctx, runnerName, script, target.ResourceType)
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}

	logger.Logf(false, "instance create successfully! (job: %s, cloud ID: %s)", job.UUID, cloudID)

	r := datastore.Runner{
		UUID:      job.UUID,
		ShoesType: shoesType,
		IPAddress: ipAddress,
		TargetID:  target.UUID,
		CloudID:   cloudID,
	}
	if err := s.ds.CreateRunner(ctx, r); err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	return nil
}
