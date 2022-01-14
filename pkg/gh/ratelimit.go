package gh

import (
	"fmt"
	"sync"
)

func storeRateLimit(scope string, rateLimit int) {
	rateLimitCount.Store(scope, rateLimit)
}

func getRateLimitKey(org, repo string) string {
	if repo == "" {
		return org
	}
	return fmt.Sprintf("%s/%s", org, repo)
}

// GetRateLimit get a list of rate limit
func GetRateLimit() map[string]int {
	m := map[string]int{}
	mu := sync.Mutex{}

	rateLimitCount.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return false
		}
		v, ok := value.(int)
		if !ok {
			return false
		}

		mu.Lock()
		m[k] = v
		mu.Unlock()
		return true
	})

	return m
}
