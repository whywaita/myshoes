package gh

import (
	"fmt"
	"sync"

	"github.com/google/go-github/v35/github"
)

func storeRateLimit(scope string, rateLimit github.Rate) {
	if rateLimit.Reset.IsZero() {
		// Not configure rate limit, don't need to store (e.g. GHES)
		return
	}

	rateLimitLimit.Store(scope, rateLimit.Limit)
	rateLimitRemain.Store(scope, rateLimit.Remaining)
}

func getRateLimitKey(org, repo string) string {
	if repo == "" {
		return org
	}
	return fmt.Sprintf("%s/%s", org, repo)
}

// GetRateLimitRemain get a list of rate limit remaining
// key: scope, value: remain
func GetRateLimitRemain() map[string]int {
	m := map[string]int{}
	mu := sync.Mutex{}

	rateLimitRemain.Range(func(key, value interface{}) bool {
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

// GetRateLimitLimit get a list of rate limit
// key: scope, value: remain
func GetRateLimitLimit() map[string]int {
	m := map[string]int{}
	mu := sync.Mutex{}

	rateLimitLimit.Range(func(key, value interface{}) bool {
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
