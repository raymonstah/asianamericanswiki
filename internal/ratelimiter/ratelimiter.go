package ratelimiter

import (
	"errors"
	"sync"
	"time"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type RateLimit struct {
	LastAccess time.Time
	Count      int
}

type RateLimiter struct {
	users         map[string]*RateLimit
	lock          *sync.Mutex
	maxRequests   int // per a given time period
	throttleAfter time.Duration
}

func (rl *RateLimiter) Check(id string) error {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	now := time.Now()
	rateLimit, ok := rl.users[id]
	if !ok {
		rateLimit = &RateLimit{}
		rl.users[id] = rateLimit

	}

	if now.Before(rateLimit.LastAccess.Add(rl.throttleAfter)) {
		rateLimit.LastAccess = now
		if rateLimit.Count >= rl.maxRequests {
			return ErrRateLimitExceeded
		}
		rateLimit.Count++
	} else {
		// reset
		rateLimit.LastAccess = now
		rateLimit.Count = 1
	}

	return nil
}

func New(maxRequests int, throttleAfter time.Duration) *RateLimiter {
	return &RateLimiter{
		users:         make(map[string]*RateLimit),
		maxRequests:   maxRequests,
		throttleAfter: throttleAfter,
		lock:          &sync.Mutex{},
	}
}
