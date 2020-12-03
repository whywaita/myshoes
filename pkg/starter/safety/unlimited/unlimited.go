package unlimited

import "github.com/whywaita/myshoes/pkg/datastore"

// Unlimited is implement of safety.
// Unlimited has not safety, so create a runner quickly.
type Unlimit struct{}

func (u *Unlimit) Check(job *datastore.Job) (bool, error) {
	return true, nil
}
