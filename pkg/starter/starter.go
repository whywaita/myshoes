package starter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/google/go-github/v47/github"
	uuid "github.com/satori/go.uuid"
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
	CountRunning atomic.Int64
	// CountWaiting is count of waiting job
	CountWaiting atomic.Int64

	// CountRecovered is count of recovered job per target
	CountRecovered = sync.Map{}

	inProgress = sync.Map{}

	reQueuedJobs = sync.Map{}
)

// Starter is dispatcher for running job
type Starter struct {
	ds              datastore.Datastore
	safety          safety.Safety
	runnerVersion   string
	notifyEnqueueCh <-chan struct{}
}

// New create starter instance
func New(ds datastore.Datastore, s safety.Safety, runnerVersion string, notifyEnqueueCh <-chan struct{}) *Starter {
	return &Starter{
		ds:              ds,
		safety:          s,
		runnerVersion:   runnerVersion,
		notifyEnqueueCh: notifyEnqueueCh,
	}
}

// Loop is main loop for starter
func (s *Starter) Loop(ctx context.Context) error {
	logger.Logf(false, "start starter loop")
	ch := make(chan datastore.Job)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		s.reRunWorkflow(ctx)
		return nil
	})

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
			case <-s.notifyEnqueueCh:
				ticker.Reset(10 * time.Second)
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
			CountWaiting.Add(1)
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("failed to Acquire: %w", err)
			}
			CountWaiting.Add(-1)
			CountRunning.Add(1)

			inProgress.Store(job.UUID, struct{}{})

			go func(job datastore.Job) {
				defer func() {
					sem.Release(1)
					inProgress.Delete(job.UUID)
					CountRunning.Add(-1)
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
	if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusRunning, ""); err != nil {
		return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
	}

	target, err := s.ds.GetTarget(ctx, job.TargetID)
	if err != nil {
		return fmt.Errorf("failed to retrieve relational target: (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
	}

	CountRecovered.LoadOrStore(target.Scope, 0)

	cctx, cancel := context.WithTimeout(ctx, runner.MustRunningTime)
	defer cancel()
	cloudID, ipAddress, shoesType, resourceType, err := s.bung(cctx, job, *target)
	if err != nil {
		logger.Logf(false, "failed to bung (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

		if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("failed to create an instance (job ID: %s)", job.UUID)); err != nil {
			return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
		}

		return fmt.Errorf("failed to bung (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
	}
	if resourceType == datastore.ResourceTypeUnknown {
		resourceType = target.ResourceType
	}

	runnerName := runner.ToName(job.UUID.String())
	if config.Config.Strict {
		if err := s.checkRegisteredRunner(ctx, runnerName, *target); err != nil {
			logger.Logf(false, "failed to check to register runner (target ID: %s, job ID: %s): %+v\n", job.TargetID, job.UUID, err)

			if err := deleteInstance(ctx, cloudID, job.CheckEventJSON); err != nil {
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
		UUID:         job.UUID,
		ShoesType:    shoesType,
		IPAddress:    ipAddress,
		TargetID:     job.TargetID,
		CloudID:      cloudID,
		ResourceType: resourceType,
		RunnerUser: sql.NullString{
			String: config.Config.RunnerUser,
			Valid:  true,
		},
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
func (s *Starter) bung(ctx context.Context, job datastore.Job, target datastore.Target) (string, string, string, datastore.ResourceType, error) {
	logger.Logf(false, "start create instance (job: %s)", job.UUID)
	runnerName := runner.ToName(job.UUID.String())

	script, err := s.getSetupScript(ctx, target, runnerName)
	if err != nil {
		return "", "", "", datastore.ResourceTypeUnknown, fmt.Errorf("failed to get setup scripts: %w", err)
	}

	client, teardown, err := shoes.GetClient()
	if err != nil {
		return "", "", "", datastore.ResourceTypeUnknown, fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	labels, err := gh.ExtractRunsOnLabels([]byte(job.CheckEventJSON))
	if err != nil {
		return "", "", "", datastore.ResourceTypeUnknown, fmt.Errorf("failed to extract labels: %w", err)
	}

	cloudID, ipAddress, shoesType, resourceType, err := client.AddInstance(ctx, runnerName, script, target.ResourceType, labels)
	if err != nil {
		return "", "", "", datastore.ResourceTypeUnknown, fmt.Errorf("failed to add instance: %w", err)
	}

	logger.Logf(false, "instance create successfully! (job: %s, cloud ID: %s)", job.UUID, cloudID)

	return cloudID, ipAddress, shoesType, resourceType, nil
}

func deleteInstance(ctx context.Context, cloudID, checkEventJSON string) error {
	client, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	labels, err := gh.ExtractRunsOnLabels([]byte(checkEventJSON))
	if err != nil {
		return fmt.Errorf("failed to extract labels: %w", err)
	}

	if err := client.DeleteInstance(ctx, cloudID, labels); err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	logger.Logf(false, "successfully delete instance that not registered (cloud ID: %s)", cloudID)
	return nil
}

// checkRegisteredRunner check to register runner to GitHub
func (s *Starter) checkRegisteredRunner(ctx context.Context, runnerName string, target datastore.Target) error {
	client, err := gh.NewClient(target.GitHubToken)
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

func (s *Starter) reRunWorkflow(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			gh.PendingRuns.Range(func(key, value any) bool {
				installationID := key.(int64)
				run := value.(*github.WorkflowRun)
				client, err := gh.NewClientInstallation(installationID)
				if err != nil {
					logger.Logf(false, "failed to create GitHub client: %+v", err)
					return true
				}

				owner := run.GetRepository().GetOwner().GetLogin()
				repo := run.GetRepository().GetName()
				repoName := run.GetRepository().GetFullName()

				jobs, _, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, run.GetID(), &github.ListWorkflowJobsOptions{
					Filter: "latest",
				})
				if err != nil {
					logger.Logf(false, "failed to get workflow jobs: %+v", err)
					return true
				}

				for _, j := range jobs.Jobs {
					if value, ok := reQueuedJobs.Load(j.GetID()); ok {
						expired := value.(time.Time)
						if time.Until(expired) <= 0 {
							reQueuedJobs.Delete(j.GetID())
						}
						continue
					}
					if !slices.Contains(j.Labels, "self-hosted") && !slices.Contains(j.Labels, "myshoes") {
						continue
					}
					if j.GetStatus() == "queued" {
						repoURL := run.GetRepository().GetHTMLURL()
						u, err := url.Parse(repoURL)
						if err != nil {
							logger.Logf(false, "failed to parse repository url from event: %+v", err)
							continue
						}
						var domain string
						gheDomain := ""
						if u.Host != "github.com" {
							gheDomain = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
							domain = gheDomain
						} else {
							domain = "https://github.com"
						}

						logger.Logf(false, "receive webhook repository: %s/%s", domain, repoName)
						target, err := datastore.SearchRepo(ctx, s.ds, repoName)
						if err != nil {
							logger.Logf(false, "failed to search registered target: %+v", err)
							continue
						}
						jobID := uuid.NewV4()
						jobJSON, _ := json.Marshal(j)
						job := datastore.Job{
							UUID:           jobID,
							TargetID:       target.UUID,
							Repository:     repoName,
							CheckEventJSON: string(jobJSON),
						}
						if err := s.ds.EnqueueJob(ctx, job); err != nil {
							logger.Logf(false, "failed to enqueue job: %+v", err)
							continue
						}
						reQueuedJobs.Store(j.GetID(), time.Now().Add(30*time.Minute))
						countRecovered, _ := CountRecovered.LoadOrStore(target.Scope, 0)
						CountRecovered.Store(target.Scope, countRecovered.(int)+1)
					}
				}
				gh.PendingRuns.Delete(installationID)
				gh.ClearRunsCache(owner, repo)
				return true
			})
		}
	}
}
