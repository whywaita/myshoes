package gh

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v80/github"
)

// parseEventJSON parse a json of webhook from GitHub.
// github.ParseWebHook need *http.Request because it checks headers in request.
func parseEventJSON(in []byte) (interface{}, error) {
	var checkRun *github.CheckRunEvent
	err := json.Unmarshal(in, &checkRun)
	if err == nil && checkRun.GetCheckRun() != nil {
		return checkRun, nil
	}

	var workflowJobEvent *github.WorkflowJobEvent
	err = json.Unmarshal(in, &workflowJobEvent)
	if err == nil && workflowJobEvent.GetWorkflowJob() != nil {
		return workflowJobEvent, nil
	}

	var workflowJob *github.WorkflowJob
	err = json.Unmarshal(in, &workflowJob)
	if err == nil && workflowJob != nil {
		return workflowJob, nil
	}

	return nil, fmt.Errorf("input json is unsupported type")
}

// ExtractRunsOnLabels extract labels from github.WorkflowJobEvent
func ExtractRunsOnLabels(in []byte) ([]string, error) {
	event, err := parseEventJSON(in)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event json: %w", err)
	}

	switch t := event.(type) {
	case *github.WorkflowJobEvent:
		// workflow_job has labels, can extract labels
		return t.GetWorkflowJob().Labels, nil
	case *github.WorkflowJob:
		return t.Labels, nil
	}

	return []string{}, nil
}
