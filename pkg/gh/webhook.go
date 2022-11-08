package gh

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/go-github/v47/github"
)

// parseEventJSON parse a json of webhook from GitHub.
// github.ParseWebHook need *http.Request because it checks headers in request.
func parseEventJSON(in []byte) (interface{}, error) {
	var checkRun *github.CheckRunEvent
	log.Println(string(in))
	err := json.Unmarshal(in, &checkRun)
	if err == nil && checkRun.GetCheckRun() != nil {
		log.Println("reach checkRun")
		return checkRun, nil
	}

	var workflowJob *github.WorkflowJobEvent
	err = json.Unmarshal(in, &workflowJob)
	if err == nil && workflowJob.GetWorkflowJob() != nil {
		log.Println("reach workflowJob")
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
	case github.WorkflowJobEvent:
		// workflow_job has labels, can extract labels
		log.Printf("t.GetWorkflowJob().Labels: %v", t.GetWorkflowJob().Labels)
		return t.GetWorkflowJob().Labels, nil
	}

	log.Println("nil labels")
	return []string{}, nil
}
