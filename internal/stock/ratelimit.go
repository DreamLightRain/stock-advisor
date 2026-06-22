package stock

import (
	"math/rand"
	"sync"
	"time"
)

type RateLimiter struct {
	lastTime    time.Time
	minInterval time.Duration
	jitter      time.Duration
	mu          sync.Mutex
}

func NewRateLimiter(minInterval time.Duration) *RateLimiter {
	return &RateLimiter{
		minInterval: minInterval,
		jitter:      minInterval / 4,
	}
}

func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.lastTime)
	if elapsed < rl.minInterval {
		jitter := time.Duration(rand.Int63n(int64(rl.jitter * 2))) - rl.jitter
		wait := rl.minInterval - elapsed + jitter
		if wait > 0 {
			time.Sleep(wait)
		}
	}
	rl.lastTime = time.Now()
}

func doWithBackoff(attempt int) {
	if attempt < 2 {
		base := time.Duration(attempt+1) * 300 * time.Millisecond
		jitter := time.Duration(rand.Int63n(int64(base) / 3))
		time.Sleep(base + jitter)
	}
}
