// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package web

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

//IPRateLimiterConfig configures an IPRateLimiter.
type IPRateLimiterConfig struct {
	Duration  time.Duration `help:"the rate at which request are allowed" default:"5m"`
	Burst     int           `help:"number of events before the limit kicks in" default:"3"`
	NumLimits int           `help:"number of IPs whose rate limits we store" default:"1000"`
}

//IPRateLimiter imposes a rate limit per HTTP user IP.
type IPRateLimiter struct {
	config   IPRateLimiterConfig
	mu       sync.Mutex
	ipLimits map[string]*userLimit
}

//userLimit is the per-IP limiter.
type userLimit struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

//NewIPRateLimiter constructs an IPRateLimiter.
func NewIPRateLimiter(config IPRateLimiterConfig) *IPRateLimiter {
	return &IPRateLimiter{
		config:   config,
		ipLimits: make(map[string]*userLimit),
	}
}

// Run occasionally cleans old rate-limiting data, until context cancel.
func (rl *IPRateLimiter) Run(ctx context.Context) {
	cleanupTicker := time.NewTicker(rl.config.Duration)
	defer cleanupTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanupTicker.C:
			rl.cleanupLimiters()
		}
	}
}

// cleanupLimiters removes old rate limits to free memory.
func (rl *IPRateLimiter) cleanupLimiters() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, v := range rl.ipLimits {
		if time.Since(v.lastSeen) > rl.config.Duration {
			delete(rl.ipLimits, ip)
		}
	}
}

//Limit applies a per IP rate limiting as an HTTP Handler.
func (rl *IPRateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		ipLimit := rl.getUserLimit(ip)
		if !ipLimit.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

//getUserLimit returns a rate limiter for an IP.
func (rl *IPRateLimiter) getUserLimit(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.ipLimits[ip]
	if !exists {
		if len(rl.ipLimits) >= rl.config.NumLimits {
			// Tracking only N limits prevents an out-of-memory DOS attack
			// Returning StatusTooManyRequests would be just as bad
			// The least-bad option may be to remove the oldest key
			oldestKey := ""
			var oldestTime *time.Time
			for ip, v := range rl.ipLimits {
				// while we're looping, we'd prefer to just delete expired records
				if time.Since(v.lastSeen) > rl.config.Duration {
					delete(rl.ipLimits, ip)
				}
				// but we're prepared to delete the oldest non-expired
				if oldestTime == nil || v.lastSeen.Before(*oldestTime) {
					oldestTime = &v.lastSeen
					oldestKey = ip
				}
			}
			//only delete the oldest non-expired if there's still an issue
			if oldestKey != "" && len(rl.ipLimits) >= rl.config.NumLimits {
				delete(rl.ipLimits, oldestKey)
			}
		}
		limiter := rate.NewLimiter(rate.Limit(time.Second)/rate.Limit(rl.config.Duration), rl.config.Burst)
		rl.ipLimits[ip] = &userLimit{limiter, time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

//Burst returns the number of events that happen before the rate limit.
func (rl *IPRateLimiter) Burst() int {
	return rl.config.Burst
}

//Duration returns the amount of time required between events.
func (rl *IPRateLimiter) Duration() time.Duration {
	return rl.config.Duration
}
