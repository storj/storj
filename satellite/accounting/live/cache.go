// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/accounting"
)

var (
	// Error is the default error class for live-accounting.
	Error = errs.Class("live-accounting")
	mon   = monkit.Package()
)

// Config contains configurable values for the live accounting service.
type Config struct {
	StorageBackend    string        `help:"what to use for storing real-time accounting data"`
	BandwidthCacheTTL time.Duration `default:"5m" help:"bandwidth cache key time to live"`
}

// NewCache creates a new accounting.Cache instance using the type specified backend in
// the provided config.
func NewCache(log *zap.Logger, config Config) (accounting.Cache, error) {
	parts := strings.SplitN(config.StorageBackend, ":", 2)
	var backendType string
	if len(parts) == 0 || parts[0] == "" {
		return nil, Error.New("please specify a backend for live accounting")
	}

	backendType = parts[0]
	switch backendType {
	case "redis":
		return newRedisLiveAccounting(log, config.StorageBackend)
	default:
		return nil, Error.New("unrecognized live accounting backend specifier %q. Currently only redis is supported", backendType)
	}
}
