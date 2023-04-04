// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"storj.io/common/lrucache"
)

// RateLimiter allows to prevent multiple events in fixed period of time.
type RateLimiter struct {
	limiters *lrucache.ExpiringLRU
	interval time.Duration // interval during which events are not limiting.
	burst    int           // maximum number of events allowed during duration.
}

// NewRateLimiter is a constructor for RateLimiter.
func NewRateLimiter(interval time.Duration, burst, numLimits int) *RateLimiter {
	return &RateLimiter{
		limiters: lrucache.New(lrucache.Options{
			Expiration: -1,
			Capacity:   numLimits,
			Name:       "contact-ratelimit",
		}),
		interval: interval,
		burst:    burst,
	}
}

// IsAllowed indicates if event is allowed to happen.
func (rateLimiter *RateLimiter) IsAllowed(key string) bool {
	limiter, err := rateLimiter.limiters.Get(key, func() (interface{}, error) {
		return rate.NewLimiter(
			rate.Limit(time.Second)/rate.Limit(rateLimiter.interval),
			rateLimiter.burst,
		), nil
	})
	if err != nil {
		panic(fmt.Sprintf("unreachable: %+v", err))
	}

	return limiter.(*rate.Limiter).Allow()
}
