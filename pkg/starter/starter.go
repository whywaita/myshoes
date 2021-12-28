package starter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/runner"
	"github.com/whywaita/myshoes/pkg/shoes"
	"github.com/whywaita/myshoes/pkg/starter/safety"
)

var (
	// CountRunning is count of running semaphore
	CountRunning = 0
	// CountWaiting is count of waiting job
	CountWaiting = 0

	inProgress = sync.Map{}
)

// Starter is dispatcher for running job
type Starter struct {
	ds     datastore.Datastore
	safety safety.Safety
}

// New create starter instance
func New(ds datastore.Datastore, s safety.Safety) *Starter {
	return &Starter{
		ds:     ds,
		safety: s,
	}
}

// Loop is main loop for starter
func (s *Starter) Loop(ctx context.Context) error {
	logger.Logf(false, "start starter loop")
	ch := make(chan datastore.Job)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := s.run(ctx, ch); err != nil {
			return fmt.Errorf("faied to start processor: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.dispatcher(ctx, ch); err != nil {
					logger.Logf(false, "failed to starter: %+v", err)
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to errgroup wait: %w", err)
	}
	return nil
}

func (s *Starter) dispatcher(ctx context.Context, ch chan datastore.Job) error {
	logger.Logf(true, "start to check starter")
	jobs, err := s.ds.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get jobs: %w", err)
	}

	for _, j := range jobs {
		// send to processor
		ch <- j
	}

	return nil
}

func (s *Starter) run(ctx context.Context, ch chan datastore.Job) error {
	sem := semaphore.NewWeighted(config.Config.MaxConnectionsToBackend)

	// Processor
	for {
		select {
		case job := <-ch:
			// receive job from dispatcher

			if _, ok := inProgress.Load(job.UUID); ok {
				// this job is in progress, skip
				continue
			}

			logger.Logf(true, "found new job: %s", job.UUID)
			CountWaiting++
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("failed to Acquire: %w", err)
			}
			CountWaiting--
			CountRunning++

			inProgress.Store(job.UUID, struct{}{})

			go func(job datastore.Job) {
				defer func() {
					sem.Release(1)
					inProgress.Delete(job.UUID)
					CountRunning--
				}()

				if err := s.processJob(ctx, job); err != nil {
					logger.Logf(false, "failed to process job: %+v\n", err)
				}
			}(job)

		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Starter) processJob(ctx context.Context, job datastore.Job) error {
	logger.Logf(false, "start job (job id: %s)\n", job.UUID.String())

	isOK, err := s.safety.Check(&job)
	if err != nil {
		return fmt.Errorf("failed to check safety: %w", err)
	}
	if !isOK {
		// is not ok, save job
		return nil
	}
	if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusRunning, fmt.Sprintf("job id: %s", job.UUID)); err != nil {
		return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
	}

	target, err := s.ds.GetTarget(ctx, job.TargetID)
	if err != nil {
		return fmt.Errorf("failed to retrieve relational target: (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
	}

	cctx, cancel := context.WithTimeout(ctx, runner.MustRunningTime)
	defer cancel()
	cloudID, ipAddress, shoesType, err := s.bung(cctx, job.UUID, *target)
	if err != nil {
		logger.Logf(false, "failed to bung (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

		if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("failed to create an instance (job ID: %s)", job.UUID)); err != nil {
			return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
		}

		return fmt.Errorf("failed to bung (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
	}

	runnerName := runner.ToName(job.UUID.String())
	if config.Config.Strict {
		if err := s.checkRegisteredRunner(ctx, runnerName, *target); err != nil {
			logger.Logf(false, "failed to check to register runner (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

			if err := deleteInstance(ctx, cloudID); err != nil {
				logger.Logf(false, "failed to delete an instance that not registered instance (target ID: %s, cloud ID: %s): %+v\n", job.TargetID, cloudID, err)
				// not return, need to update target status if err.
			}

			if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("cannot register runner to GitHub (job ID: %s)", job.UUID)); err != nil {
				return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
			}

			return fmt.Errorf("failed to check to register runner (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
		}
	}

	r := datastore.Runner{
		UUID:           job.UUID,
		ShoesType:      shoesType,
		IPAddress:      ipAddress,
		TargetID:       job.TargetID,
		CloudID:        cloudID,
		ResourceType:   target.ResourceType,
		RunnerUser:     target.RunnerUser,
		RunnerVersion:  target.RunnerVersion,
		ProviderURL:    target.ProviderURL,
		RepositoryURL:  job.RepoURL(),
		RequestWebhook: job.CheckEventJSON,
	}
	if err := s.ds.CreateRunner(ctx, r); err != nil {
		logger.Logf(false, "failed to save runner to datastore (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

		if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("job id: %s", job.UUID)); err != nil {
			return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
		}

		return fmt.Errorf("failed to save runner to datastore (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
	}

	if err := s.ds.DeleteJob(ctx, job.UUID); err != nil {
		logger.Logf(false, "failed to delete job: %+v\n", err)

		if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("job id: %s", job.UUID)); err != nil {
			return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
		}

		return fmt.Errorf("failed to delete job: %w", err)
	}

	return nil
}

// bung is start runner, like a pistol! :)
func (s *Starter) bung(ctx context.Context, jobUUID uuid.UUID, target datastore.Target) (string, string, string, error) {
	logger.Logf(false, "start create instance (job: %s)", jobUUID)
	runnerName := runner.ToName(jobUUID.String())

	script, err := s.getSetupScript(ctx, target, runnerName)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get setup scripts: %w", err)
	}

	client, teardown, err := shoes.GetClient()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	cloudID, ipAddress, shoesType, err := client.AddInstance(ctx, runnerName, script, target.ResourceType)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to add instance: %w", err)
	}

	logger.Logf(false, "instance create successfully! (job: %s, cloud ID: %s)", jobUUID, cloudID)

	return cloudID, ipAddress, shoesType, nil
}

func deleteInstance(ctx context.Context, cloudID string) error {
	client, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	if err := client.DeleteInstance(ctx, cloudID); err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	logger.Logf(false, "successfully delete instance that not registered (cloud ID: %s)", cloudID)
	return nil
}

// checkRegisteredRunner check to register runner to GitHub
func (s *Starter) checkRegisteredRunner(ctx context.Context, runnerName string, target datastore.Target) error {
	client, err := gh.NewClient(ctx, target.GitHubToken, target.GHEDomain.String)
	if err != nil {
		return fmt.Errorf("failed to create github client: %w", err)
	}
	owner, repo := gh.DivideScope(target.Scope)

	cctx, cancel := context.WithTimeout(ctx, runner.MustRunningTime)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	count := 0
	for {
		select {
		case <-cctx.Done():
			// timeout
			return fmt.Errorf("faied to to check existing runner in GitHub: timeout in %s", runner.MustRunningTime)
		case <-ticker.C:
			if _, err := gh.ExistGitHubRunner(cctx, client, owner, repo, runnerName); err == nil {
				// success to register runner to GitHub
				return nil
			} else if !errors.Is(err, gh.ErrNotFound) {
				// not retryable error
				return fmt.Errorf("failed to check existing runner in GitHub: %w", err)
			}
			count++
			logger.Logf(true, "%s is not found in GitHub, will retry... (second: %ds)", runnerName, count)
		}
	}
}
