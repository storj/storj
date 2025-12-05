// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package web

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	internalServerErrMsg = "An internal server error has occurred."
	rateLimitErrMsg      = "You've exceeded your request limit. Please try again later."
)

// RateLimiterConfig configures a RateLimiter.
type RateLimiterConfig struct {
	Duration  time.Duration `help:"the rate at which request are allowed" default:"5m"`
	Burst     int           `help:"number of events before the limit kicks in" default:"5" testDefault:"3"`
	NumLimits int           `help:"number of clients whose rate limits we store" default:"1000" testDefault:"10"`
}

// RateLimiter imposes a rate limit per key.
type RateLimiter struct {
	config  RateLimiterConfig
	log     *zap.Logger
	mu      sync.Mutex
	limits  map[string]*userLimit
	keyFunc func(*http.Request) (string, error)
}

// userLimit is the per-key limiter.
type userLimit struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter constructs a RateLimiter that limits based on IP address.
func NewIPRateLimiter(config RateLimiterConfig, log *zap.Logger) *RateLimiter {
	return NewRateLimiter(config, log, GetRequestIP)
}

// NewRateLimiter constructs a RateLimiter.
func NewRateLimiter(config RateLimiterConfig, log *zap.Logger, keyFunc func(*http.Request) (string, error)) *RateLimiter {
	return &RateLimiter{
		config:  config,
		limits:  make(map[string]*userLimit),
		keyFunc: keyFunc,
	}
}

// Run occasionally cleans old rate-limiting data, until context cancel.
func (rl *RateLimiter) Run(ctx context.Context) {
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
func (rl *RateLimiter) cleanupLimiters() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for k, v := range rl.limits {
		if time.Since(v.lastSeen) > rl.config.Duration {
			delete(rl.limits, k)
		}
	}
}

// Limit applies per-key rate limiting as an HTTP Handler.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := rl.keyFunc(r)
		if err != nil {
			ServeCustomJSONError(r.Context(), rl.log, w, http.StatusInternalServerError, err, internalServerErrMsg)
			return
		}
		limit := rl.getUserLimit(key)
		if !limit.Allow() {
			ServeJSONError(r.Context(), rl.log, w, http.StatusTooManyRequests, errs.New(rateLimitErrMsg))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetRequestIP gets the original IP address of the request by handling the request headers.
func GetRequestIP(r *http.Request) (ip string, err error) {
	realIP := r.Header.Get("X-REAL-IP")
	if realIP != "" {
		return realIP, nil
	}

	forwardedIPs := r.Header.Get("X-FORWARDED-FOR")
	if forwardedIPs != "" {
		ips := strings.Split(forwardedIPs, ", ")
		if len(ips) > 0 {
			return ips[0], nil
		}
	}

	ip, _, err = net.SplitHostPort(r.RemoteAddr)

	return ip, err
}

// getUserLimit returns a rate limiter for a key.
func (rl *RateLimiter) getUserLimit(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.limits[key]
	if !exists {
		if len(rl.limits) >= rl.config.NumLimits {
			// Tracking only N limits prevents an out-of-memory DOS attack
			// Returning StatusTooManyRequests would be just as bad
			// The least-bad option may be to remove the oldest key
			oldestKey := ""
			var oldestTime *time.Time
			for key, v := range rl.limits {
				// while we're looping, we'd prefer to just delete expired records
				if time.Since(v.lastSeen) > rl.config.Duration {
					delete(rl.limits, key)
				}
				// but we're prepared to delete the oldest non-expired
				if oldestTime == nil || v.lastSeen.Before(*oldestTime) {
					oldestTime = &v.lastSeen
					oldestKey = key
				}
			}
			// only delete the oldest non-expired if there's still an issue
			if oldestKey != "" && len(rl.limits) >= rl.config.NumLimits {
				delete(rl.limits, oldestKey)
			}
		}
		maxFreq := rate.Inf
		if rl.config.Duration != 0 {
			maxFreq = rate.Limit(time.Second) / rate.Limit(rl.config.Duration)
		}
		limiter := rate.NewLimiter(maxFreq, rl.config.Burst)
		rl.limits[key] = &userLimit{limiter, time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// Burst returns the number of events that happen before the rate limit.
func (rl *RateLimiter) Burst() int {
	return rl.config.Burst
}

// Duration returns the amount of time required between events.
func (rl *RateLimiter) Duration() time.Duration {
	return rl.config.Duration
}
