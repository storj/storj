// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[RangeSplitter](ball, func(db *metabase.DB, config Config) *MetabaseRangeSplitter {
		return NewMetabaseRangeSplitter(db, config.AsOfSystemInterval, config.SpannerStaleInterval, config.BatchSize)
	})
	mud.Provide[*Service](ball, NewService)
	mud.Provide[*LiveCountObserver](ball, func(db *metabase.DB, cfg Config) *LiveCountObserver {
		return NewLiveCountObserver(db, cfg.SuspiciousProcessedRatio, cfg.AsOfSystemInterval)
	})
	config.RegisterConfig[Config](ball, "ranged-loop")
	mud.RegisterImplementation[[]Observer](ball)
	mud.Implementation[[]Observer, *LiveCountObserver](ball)

}
