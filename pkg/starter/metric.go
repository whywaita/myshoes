package starter

import (
	"fmt"
	"sync"

	"github.com/whywaita/myshoes/pkg/gh"

	"github.com/whywaita/myshoes/pkg/datastore"
)

var (
	// DeletedJobMap is map for deleted jobs. key: runs_on, value: number of deleted jobs
	DeletedJobMap = sync.Map{}
)

func incrementDeleteJobMap(j datastore.Job) error {
	runsOnConcat, err := gh.ConcatLabels(j.CheckEventJSON)
	if err != nil {
		return fmt.Errorf("failed to concat labels: %+v", err)
	}
	v, ok := DeletedJobMap.Load(runsOnConcat)
	if !ok {
		DeletedJobMap.Store(runsOnConcat, 1)
		return nil
	}

	DeletedJobMap.Store(runsOnConcat, v.(int)+1)
	return nil
}
