package starter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/runner"
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

			target, err := s.ds.GetTarget(ctx, job.TargetID)
			if err != nil {
				logger.Logf(false, "failed to retrieve relational target: (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)
				return
			}

			cloudID, ipAddress, shoesType, err := s.bung(ctx, job.UUID, *target)
			if err != nil {
				logger.Logf(false, "failed to bung (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

				if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("failed to create an instance (job ID: %s)", job.UUID)); err != nil {
					logger.Logf(false, "failed to update target status (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)
					return
				}

				return
			}

			if err := s.checkRegisteredRunner(ctx, cloudID, *target); err != nil {
				logger.Logf(false, "failed to check to register runner (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

				if err := deleteInstance(ctx, cloudID); err != nil {
					logger.Logf(false, "failed to delete an instance that not registered instance (target ID: %s, cloud ID: %s): %+v\n", job.TargetID, cloudID, err)
					// not return, need to update target status if err.
				}

				if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("cannot register runner to GitHub (job ID: %s)", job.UUID)); err != nil {
					logger.Logf(false, "failed to update target status (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)
					return
				}

				return
			}

			r := datastore.Runner{
				UUID:           job.UUID,
				ShoesType:      shoesType,
				IPAddress:      ipAddress,
				TargetID:       job.TargetID,
				CloudID:        cloudID,
				RepositoryURL:  job.RepoURL(),
				RequestWebhook: job.CheckEventJSON,
			}
			if err := s.ds.CreateRunner(ctx, r); err != nil {
				logger.Logf(false, "failed to save runner to datastore (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

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
func (s *Starter) bung(ctx context.Context, jobUUID uuid.UUID, target datastore.Target) (string, string, string, error) {
	logger.Logf(false, "start create instance (job: %s)", jobUUID)
	client, teardown, err := shoes.GetClient()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	script, err := s.getSetupScript(target)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get setup scripts: %w", err)
	}

	runnerName := runner.ToName(jobUUID.String())
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
func (s *Starter) checkRegisteredRunner(ctx context.Context, cloudID string, target datastore.Target) error {
	client, err := gh.NewClient(ctx, target.GitHubToken, target.GHEDomain.String)
	if err != nil {
		return fmt.Errorf("failed to create github client: %w", err)
	}

	owner, repo := gh.DivideScope(target.Scope)

	for i := 0; float64(i) < runner.MustRunningTime.Seconds(); i++ {
		if _, err := gh.ExistGitHubRunner(ctx, client, owner, repo, cloudID); err == nil {
			// success to register runner to GitHub
			return nil
		} else if !errors.Is(err, gh.ErrNotFound) {
			// not retryable error
			return fmt.Errorf("failed to check existing runner in GitHub: %w", err)
		}

		logger.Logf(true, "%s is not found in GitHub, will retry... (second: %ds)", cloudID, i)
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("faied to to check existing runner in GitHub: timeout in %s", runner.MustRunningTime)
}
