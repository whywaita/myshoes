package util

import (
	"math/rand/v2"
	"time"
)

// CalcRetryTime is caliculate retry time by exponential backoff and jitter
func CalcRetryTime(count int) time.Duration {
	if count == 0 {
		return 0
	}

	backoff := 1 << count
	jitter := time.Duration(rand.IntN(1000)) * time.Millisecond

	return time.Duration(backoff)*time.Second + jitter
}
