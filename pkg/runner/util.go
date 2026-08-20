package runner

import (
	"fmt"
	"strings"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"
)

// ToName convert uuid to runner name
func ToName(u string) string {
	return fmt.Sprintf("myshoes-%s", u)
}

// ToUUID convert runner name to uuid
func ToUUID(name string) (uuid.UUID, error) {
	u := strings.TrimPrefix(name, "myshoes-")
	return uuid.FromString(u)
}

// ToReason convert status from GitHub to datastore.RunnerStatus
func ToReason(status string) datastore.RunnerStatus {
	switch status {
	case StatusWillDelete:
		// is offline
		return datastore.RunnerStatusCompleted
	case StatusSleep:
		// is idle, reach hard limit
		return datastore.RunnerStatusReachHardLimit
	}

	return ""
}
