// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/accounting"
)

var (
	// Error is the default error class for live-accounting
	Error = errs.Class("live-accounting")
	mon   = monkit.Package()
)

// Config contains configurable values for the live accounting service.
type Config struct {
	StorageBackend string `help:"what to use for storing real-time accounting data" default:"memory"`
}

// NewCache creates a new accounting.Cache instance using the type specified backend in
// the provided config.
func NewCache(log *zap.Logger, config Config) (accounting.Cache, error) {
	parts := strings.SplitN(config.StorageBackend, ":", 2)
	var backendType string
	if len(parts) == 0 || parts[0] == "" {
		backendType = "memory"
	} else {
		backendType = parts[0]
	}
	switch backendType {
	case "memory":
		return newMemoryLiveAccounting(log)
	case "redis":
		return newRedisLiveAccounting(log, config.StorageBackend)
	default:
		return nil, Error.New("unrecognized live accounting backend specifier %q", backendType)
	}
}
