package safety

import (
	"github.com/whywaita/myshoes/pkg/datastore"
)

// Safety is interface for safety
type Safety interface {
	// Check check that can create a runner. if can create a runner, return true.
	Check(job *datastore.Job) (bool, error)
}
