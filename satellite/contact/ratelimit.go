// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"storj.io/storj/shared/lrucache"
)

// RateLimiter allows to prevent multiple events in fixed period of time.
type RateLimiter struct {
	limiters *lrucache.ExpiringLRUOf[*rate.Limiter]
	interval time.Duration // interval during which events are not limiting.
	burst    int           // maximum number of events allowed during duration.
}

// NewRateLimiter is a constructor for RateLimiter.
func NewRateLimiter(interval time.Duration, burst, numLimits int) *RateLimiter {
	return &RateLimiter{
		limiters: lrucache.NewOf[*rate.Limiter](lrucache.Options{
			Expiration: -1,
			Capacity:   numLimits,
			Name:       "contact-ratelimit",
		}),
		interval: interval,
		burst:    burst,
	}
}

func (rateLimiter *RateLimiter) get(ctx context.Context, key string) *rate.Limiter {
	limiter, err := rateLimiter.limiters.Get(ctx, key, func() (*rate.Limiter, error) {
		return rate.NewLimiter(
			rate.Limit(time.Second)/rate.Limit(rateLimiter.interval),
			rateLimiter.burst,
		), nil
	})
	if err != nil {
		panic(fmt.Sprintf("unreachable: %+v", err))
	}
	return limiter
}

// IsAllowed indicates if event is allowed to happen.
func (rateLimiter *RateLimiter) IsAllowed(ctx context.Context, key string) bool {
	return rateLimiter.get(ctx, key).Allow()

}

// BackOut undoes the rate limiting tracking of IsAllowed, in case it turns
// out we do want to let the operation happen again soon after.
func (rateLimiter *RateLimiter) BackOut(ctx context.Context, key string) {
	rateLimiter.get(ctx, key).AllowN(time.Now(), -1)
}
