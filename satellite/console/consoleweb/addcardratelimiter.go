// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"time"

	"go.uber.org/zap"

	"storj.io/storj/private/web"
)

// AddCardRateLimiterConfig defines the configuration for the add card rate limiter.
type AddCardRateLimiterConfig struct {
	Duration  time.Duration `help:"the rate at which add card requests are refilled" default:"144m"`
	Burst     int           `help:"number of add card events before the limit kicks in" default:"3"`
	NumLimits int           `help:"number of clients whose rate limits we store" default:"1000"`
}

// NewAddCardRateLimiter creates a new web.NewRateLimiter with the given configuration and logger.
func NewAddCardRateLimiter(cfg AddCardRateLimiterConfig, log *zap.Logger) *web.RateLimiter {
	config := web.RateLimiterConfig{
		Duration:  cfg.Duration,
		Burst:     cfg.Burst,
		NumLimits: cfg.NumLimits,
	}
	return web.NewRateLimiter(config, log, getUserIDFromContext)
}
