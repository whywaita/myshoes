package unlimited

import "github.com/whywaita/myshoes/pkg/datastore"

// Unlimited is implement of safety.
// Unlimited has not safety, so create a runner quickly.
type Unlimited struct{}

// Check is not limited
func (u Unlimited) Check(job *datastore.Job) (bool, error) {
	return true, nil
}
