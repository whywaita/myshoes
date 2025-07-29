package gh

import "strings"

// IsRequestedMyshoesLabel checks if the job has appropriate labels for myshoes
func IsRequestedMyshoesLabel(labels []string) bool {
	// Accept dependabot runner in GHES
	if len(labels) == 1 && strings.EqualFold(labels[0], "dependabot") {
		return true
	}

	for _, label := range labels {
		if strings.EqualFold(label, "myshoes") || strings.EqualFold(label, "self-hosted") {
			return true
		}
	}
	return false
}