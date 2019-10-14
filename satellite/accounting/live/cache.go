// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/accounting"
)

// Config contains configurable values for the live accounting service.
type Config struct {
	StorageBackend string `help:"what to use for storing real-time accounting data" default:"plainmemory"`
}

// NewCache creates a new live.Service instance of the type specified in
// the provided config.
func NewCache(log *zap.Logger, config Config) (accounting.LiveAccounting, error) {
	parts := strings.SplitN(config.StorageBackend, ":", 2)
	var backendType string
	if len(parts) == 0 || parts[0] == "" {
		backendType = "plainmemory"
	} else {
		backendType = parts[0]
	}
	switch backendType {
	case "plainmemory":
		return newPlainMemoryLiveAccounting(log)
	case "redis":
		return newRedisLiveAccounting(log, config.StorageBackend)
	default:
		return nil, errs.New("unrecognized live accounting backend specifier %q", backendType)
	}
}
