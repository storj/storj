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
//
// The cache instance may be returned despite of returning the
// accounting.ErrSystemOrNetError because some backends allows to reconnect on
// each operation if the connection was not established or it was disconnected,
// which is what it could happen at the moment to instance it and the cache will
// work one the backend system will be reachable later on.
// For this reason, the components that uses the cache should operate despite
// the backend is not responding successfully although their service is
// degraded.
func NewCache(log *zap.Logger, config Config) (accounting.Cache, error) {
	parts := strings.SplitN(config.StorageBackend, ":", 2)
	var backendType string
	if len(parts) == 0 || parts[0] == "" {
		return nil, Error.New("please specify a backend for live accounting")
	}

	backendType = parts[0]
	switch backendType {
	case "redis":
		return newRedisLiveAccounting(config.StorageBackend)
	default:
		return nil, Error.New("unrecognized live accounting backend specifier %q. Currently only redis is supported", backendType)
	}
}
