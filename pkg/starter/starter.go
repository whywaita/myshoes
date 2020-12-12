package starter

import (
	"context"
	"fmt"
	"math/rand"
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
	logger.Logf("start starter loop")

	ticker := time.NewTicker(PistolInterval)

	for {
		select {
		case <-ticker.C:
			if err := s.do(ctx); err != nil {
				logger.Logf("failed to start do: %+v", err)
			}
		}
	}
}

func (s *Starter) do(ctx context.Context) error {
	jobs, err := s.ds.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get jobs: %w", err)
	}

	wg := &sync.WaitGroup{}
	for _, j := range jobs {
		wg.Add(1)
		job := j

		// self-hosted runner has a problem like a race condition. So wait a few random seconds.
		// ref: https://github.com/actions/runner/issues/510
		rand.Seed(time.Now().UnixNano())
		randTime := rand.Int63n(10)
		randomizeSleepTime, err := time.ParseDuration(fmt.Sprintf("%ds", randTime))
		if err != nil {
			logger.Logf("failed to parse random time (%ds): %+v\n", randTime, err)
			wg.Done()
		}

		go func() {
			defer wg.Done()
			time.Sleep(randomizeSleepTime)
			logger.Logf("start job (job id: %s)\n", job.UUID.String())

			isOK, err := s.safety.Check(&job)
			if err != nil {
				logger.Logf("failed to check safety: %+v\n", err)
				return
			}
			if !isOK {
				// is not ok, save job
				return
			}

			if err := s.bung(ctx, job); err != nil {
				logger.Logf("failed to bung: %+v\n", err)
				return
			}
			if err := s.ds.DeleteJob(ctx, job.UUID); err != nil {
				logger.Logf("failed to delete job: %+v\n", err)
				return
			}
		}()
	}

	wg.Wait()

	return nil
}

// bung is start runner, like a pistol! :)
func (s *Starter) bung(ctx context.Context, job datastore.Job) error {
	logger.Logf("start create instance (job: %s)", job.UUID)
	client, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	target, err := s.ds.GetTarget(ctx, job.TargetID)
	if err != nil {
		return fmt.Errorf("failed to retrieve relational target (job: %s, target: %s): %w", job.UUID, job.TargetID, err)
	}

	script := s.getSetupScript(*target)
	runnerName := runner.ToName(job.UUID.String())
	cloudID, ipAddress, shoesType, err := client.AddInstance(ctx, runnerName, script)
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}

	logger.Logf("instance create successfully! (job: %s, cloud ID: %s)", job.UUID, cloudID)

	now := time.Now()
	r := datastore.Runner{
		UUID:      job.UUID,
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
