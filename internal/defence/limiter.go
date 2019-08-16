package defence

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type limit struct {
	limiter *rate.Limiter
	expire  time.Time
	key     string

	IsBanned bool
}

// Limiter is used to store and manage list of banned entities
type Limiter struct {
	limited map[string]*limit

	// Attempts defines how many times attacker could perform an operation
	Attempts int
	// AttemptsPeriod defines period in which attempts will count. For example, 5 attempts per minute.
	AttemptsPeriod time.Duration
	BanDuration    time.Duration

	sync.Mutex
}

// New is a constructor for Limiter
func New(attempts int, attemptsPeriod, banDuration time.Duration) *Limiter {
	return &Limiter{
		limited:        map[string]*limit{},
		Attempts:       attempts,
		AttemptsPeriod: attemptsPeriod,
		BanDuration:    banDuration,
	}
}

// Banned returns the list of banned limits
func (limiter *Limiter) Banned() []*limit {
	var limits []*limit

	for _, limit := range limiter.limited {
		if limit.IsBanned {
			limits = append(limits, limit)
		}
	}

	return limits
}

// Find can be used to find a limit by specified key
func (limiter *Limiter) Find(key string) (*limit, bool) {
	limiter.Lock()

	defer limiter.Unlock()

	client, ok := limiter.limited[key]
	return client, ok
}

// Clear is used to clean all limits whom ban time is expired
func (limiter *Limiter) Clear(quit <-chan struct{}) {
	limiter.Lock()

	defer limiter.Unlock()

	tick := time.Tick(limiter.AttemptsPeriod)

	for {
		select {
		case <-quit:
			break
		case <-tick:
			for key, limit := range limiter.limited {
				if time.Now().After(limit.expire) {
					delete(limiter.limited, key)
				}
			}
		}
	}
}
