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

	"github.com/google/go-github/v80/github"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/internal/util"
	"github.com/whywaita/myshoes/pkg/config"
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

	// CountRescued is count of rescued job per target
	CountRescued = sync.Map{}

	inProgress = sync.Map{}

	// AddInstanceRetryCount is count of retry to add instance
	AddInstanceRetryCount = sync.Map{}
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
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.reRunWorkflow(ctx)
			case <-ctx.Done():
				return nil
			}
		}
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
			c, _ := AddInstanceRetryCount.LoadOrStore(job.UUID, 0)
			count, _ := c.(int)

			runID, jobID, err := extractWorkflowIDs(job)
			if err != nil {
				logger.Logf(true, "found new job: %s (repo: %s)", job.UUID, job.Repository)
			} else {
				logger.Logf(true, "found new job: %s (gh_run_id: %d, gh_job_id: %d, repo: %s)", job.UUID, runID, jobID, job.Repository)
			}
			CountWaiting.Add(1)
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("failed to Acquire: %w", err)
			}
			CountWaiting.Add(-1)
			CountRunning.Add(1)

			inProgress.Store(job.UUID, struct{}{})

			sleep := util.CalcRetryTime(count)
			if count > 0 {
				AddInstanceRetryTotal.WithLabelValues(job.UUID.String()).Inc()
				AddInstanceBackoffDuration.WithLabelValues(job.UUID.String()).Observe(sleep.Seconds())
			}
			go func(job datastore.Job, sleep time.Duration) {
				defer func() {
					sem.Release(1)
					inProgress.Delete(job.UUID)
					CountRunning.Add(-1)
				}()

				time.Sleep(sleep)
				if err := s.ProcessJob(ctx, job); err != nil {
					AddInstanceRetryCount.Store(job.UUID, count+1)
					logger.Logf(false, "failed to process job: %+v\n", err)
				} else {
					AddInstanceRetryCount.Delete(job.UUID)
				}
			}(job, sleep)

		case <-ctx.Done():
			return nil
		}
	}
}

// extractWorkflowIDs extracts GitHub workflow run ID and job ID from a datastore.Job
func extractWorkflowIDs(job datastore.Job) (runID int64, jobID int64, err error) {
	webhookEvent, err := github.ParseWebHook("workflow_job", []byte(job.CheckEventJSON))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse webhook: %w", err)
	}

	workflowJob, ok := webhookEvent.(*github.WorkflowJobEvent)
	if !ok {
		return 0, 0, fmt.Errorf("failed to cast to WorkflowJobEvent")
	}

	if workflowJob.GetWorkflowJob() == nil {
		return 0, 0, fmt.Errorf("workflow job is nil")
	}

	return workflowJob.GetWorkflowJob().GetRunID(), workflowJob.GetWorkflowJob().GetID(), nil
}

// ProcessJob is process job
func (s *Starter) ProcessJob(ctx context.Context, job datastore.Job) error {
	runID, jobID, err := extractWorkflowIDs(job)
	if err != nil {
		logger.Logf(false, "start job (job id: %s, repo: %s)\n", job.UUID.String(), job.Repository)
	} else {
		logger.Logf(false, "start job (job id: %s, gh_run_id: %d, gh_job_id: %d, repo: %s)\n", job.UUID.String(), runID, jobID, job.Repository)
	}

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

	cctx, cancel := context.WithTimeout(ctx, runner.MustRunningTime)
	defer cancel()
	cloudID, ipAddress, shoesType, resourceType, err := s.bung(cctx, job, *target)
	if err != nil {
		runID2, jobID2, extractErr := extractWorkflowIDs(job)
		if extractErr != nil {
			logger.Logf(false, "failed to bung (target ID: %s, job ID: %s): %+v", job.TargetID, job.UUID, err)
		} else {
			logger.Logf(false, "failed to bung (target ID: %s, job ID: %s, gh_run_id: %d, gh_job_id: %d): %+v", job.TargetID, job.UUID, runID2, jobID2, err)
		}

		if errors.Is(err, ErrInvalidLabel) {
			if extractErr != nil {
				logger.Logf(false, "invalid argument. so will delete (job ID: %s)", job.UUID)
			} else {
				logger.Logf(false, "invalid argument. so will delete (job ID: %s, gh_run_id: %d, gh_job_id: %d)", job.UUID, runID2, jobID2)
			}
			if err := s.ds.DeleteJob(ctx, job.UUID); err != nil {
				logger.Logf(false, "failed to delete job: %+v\n", err)

				if err := datastore.UpdateTargetStatus(ctx, s.ds, job.TargetID, datastore.TargetStatusErr, fmt.Sprintf("job id: %s", job.UUID)); err != nil {
					return fmt.Errorf("failed to update target status (target ID: %s, job ID: %s): %w", job.TargetID, job.UUID, err)
				}

				return fmt.Errorf("failed to delete job: %w", err)
			}
			if err := incrementDeleteJobMap(job); err != nil {
				return fmt.Errorf("failed to increment delete metrics: %w", err)
			}
			return nil
		}

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
	runID, jobID, extractErr := extractWorkflowIDs(job)
	if extractErr != nil {
		logger.Logf(false, "start create instance (job: %s)", job.UUID)
	} else {
		logger.Logf(false, "start create instance (job: %s, gh_run_id: %d, gh_job_id: %d)", job.UUID, runID, jobID)
	}
	runnerName := runner.ToName(job.UUID.String())

	targetScope := getTargetScope(target, job)
	script, err := s.getSetupScript(ctx, targetScope, runnerName)
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
		if stat, _ := status.FromError(err); stat.Code() == codes.InvalidArgument {
			return "", "", "", datastore.ResourceTypeUnknown, NewInvalidLabel(err)
		}
		return "", "", "", datastore.ResourceTypeUnknown, fmt.Errorf("failed to add instance: %w", err)
	}

	if extractErr != nil {
		logger.Logf(false, "instance create successfully! (job: %s, cloud ID: %s)", job.UUID, cloudID)
	} else {
		logger.Logf(false, "instance create successfully! (job: %s, cloud ID: %s, gh_run_id: %d, gh_job_id: %d)", job.UUID, cloudID, runID, jobID)
	}

	return cloudID, ipAddress, shoesType, resourceType, nil
}

// getTargetScope from target, but receive from job if datastore.target.Scope is empty
// this function is for datastore that don't store target.
func getTargetScope(target datastore.Target, job datastore.Job) string {
	if target.Scope == "" {
		return job.Repository
	}
	return target.Scope
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
	pendingRuns, err := datastore.GetPendingWorkflowRunByRecentRepositories(ctx, s.ds)
	if err != nil {
		logger.Logf(false, "failed to get pending workflow runs: %+v", err)
		return
	}

	for _, pendingRun := range pendingRuns {
		if err := reRunWorkflowByPendingRun(ctx, s.ds, pendingRun); err != nil {
			logger.Logf(false, "failed to re-run workflow: %+v", err)
			continue
		}
	}
}

func reRunWorkflowByPendingRun(ctx context.Context, ds datastore.Datastore, pendingRun datastore.PendingWorkflowRunWithTarget) error {
	if err := enqueueRescueRun(ctx, pendingRun, ds); err != nil {
		return fmt.Errorf("failed to enqueue rescue job: %w", err)
	}
	return nil
}

func enqueueRescueRun(ctx context.Context, pendingRun datastore.PendingWorkflowRunWithTarget, ds datastore.Datastore) error {
	fullName := pendingRun.WorkflowRun.GetRepository().GetFullName()

	client, target, err := datastore.NewClientInstallationByRepo(ctx, ds, fullName)
	if err != nil {
		return fmt.Errorf("failed to create a client of GitHub by repo (full_name: %s): %w", fullName, err)
	}

	jobs, err := gh.ListWorkflowJobByRunID(
		ctx,
		client,
		pendingRun.WorkflowRun.GetRepository().GetOwner().GetLogin(),
		pendingRun.WorkflowRun.GetRepository().GetName(),
		pendingRun.WorkflowRun.GetID(),
	)
	if err != nil {
		return fmt.Errorf("failed to list workflow jobs: %w", err)
	}

	for _, job := range jobs {
		if job.GetStatus() != "queued" && job.GetStatus() != "pending" {
			continue
		}

		// Check if the job has appropriate labels for myshoes
		if !gh.IsRequestedMyshoesLabel(job.Labels) {
			logger.Logf(true, "skip rescue job because it doesn't have myshoes labels: (repo: %s, gh_run_id: %d, gh_job_id: %d, labels: %v)",
				fullName, pendingRun.WorkflowRun.GetID(), job.GetID(), job.Labels)
			continue
		}

		// Get installation ID from target scope
		installationID, err := gh.IsInstalledGitHubApp(ctx, target.Scope)
		if err != nil {
			return fmt.Errorf("failed to get installation ID: %w", err)
		}

		// Get full installation data from cache
		installation, err := gh.GetInstallationByID(ctx, installationID)
		if err != nil {
			logger.Logf(false, "failed to get installation from cache (installationID: %d), using minimal data: %+v", installationID, err)
			// Fallback to minimal installation data
			installation = &github.Installation{
				ID: &installationID,
			}
		}

		owner := pendingRun.WorkflowRun.GetRepository().GetOwner()
		var org *github.Organization
		if owner != nil {
			org = &github.Organization{
				ID:    owner.ID,
				Login: owner.Login,
				Name:  owner.Name,
			}
		}

		event := &github.WorkflowJobEvent{
			WorkflowJob:  job,
			Action:       github.String("queued"),
			Org:          org,
			Repo:         pendingRun.WorkflowRun.GetRepository(),
			Sender:       pendingRun.WorkflowRun.GetActor(),
			Installation: installation,
		}

		if err := enqueueRescueJob(ctx, event, *target, ds); err != nil {
			return fmt.Errorf("failed to enqueue rescue job: %w", err)
		}
	}

	return nil
}

func enqueueRescueJob(ctx context.Context, workflowJob *github.WorkflowJobEvent, target datastore.Target, ds datastore.Datastore) error {
	jobJSON, err := json.Marshal(workflowJob)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	repository := workflowJob.GetRepo()
	if repository == nil {
		return fmt.Errorf("repository is nil")
	}
	fullName := repository.GetFullName()
	if fullName == "" {
		return fmt.Errorf("repository full name is empty")
	}

	htmlURL := repository.GetHTMLURL()
	if htmlURL == "" {
		return fmt.Errorf("repository html url is empty")
	}
	u, err := url.Parse(htmlURL)
	if err != nil {
		return fmt.Errorf("failed to parse repository url from event: %w", err)
	}

	gheDomain := ""
	if u.Host != "github.com" {
		gheDomain = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	logger.Logf(false, "rescue pending job: (repo: %s, gh_run_id: %d, gh_job_id: %d)", *repository.HTMLURL, workflowJob.WorkflowJob.GetRunID(), workflowJob.WorkflowJob.GetID())
	jobID := uuid.NewV4()
	job := datastore.Job{
		UUID: jobID,
		GHEDomain: sql.NullString{
			String: gheDomain,
			Valid:  gheDomain != "",
		},
		Repository:     fullName,
		CheckEventJSON: string(jobJSON),
		TargetID:       target.UUID,
	}
	if err := ds.EnqueueJob(ctx, job); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Increment rescued runs counter
	v, _ := CountRescued.LoadOrStore(target.Scope, &atomic.Int64{})
	counter := v.(*atomic.Int64)
	counter.Add(1)

	return nil
}
