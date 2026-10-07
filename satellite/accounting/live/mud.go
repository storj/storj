// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[accounting.Cache](ball, openModuleCache)
	config.RegisterConfig[Config](ball, "live-accounting")
}

// openModuleCache opens the cache like the classic peers do: an unreachable backend is not fatal, the cache
// reconnects on each operation and its users operate in degraded mode meanwhile.
func openModuleCache(ctx context.Context, log *zap.Logger, config Config) (accounting.Cache, error) {
	cache, err := OpenCache(ctx, log, config)
	if err != nil && accounting.ErrSystemOrNetError.Has(err) && cache != nil {
		log.Warn("Unable to connect to live accounting cache. Verify connection", zap.Error(err))
		return cache, nil
	}
	return cache, err
}
