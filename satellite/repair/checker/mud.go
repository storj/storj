// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud.Ball module definition.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "checker")
	mud.Provide[*ReliabilityCache](ball, func(overlay *overlay.Service, config Config) *ReliabilityCache {
		return NewReliabilityCache(overlay, config.ReliabilityCacheStaleness, config.OnlineWindow)
	})
	mud.Provide[Health](ball, func(config Config, cache *ReliabilityCache) Health {
		var health Health
		switch config.HealthScore {
		case "probability":
			health = NewProbabilityHealth(config.NodeFailureRate, cache)
		case "normalized":
			health = NewNormalizedHealth()
		default:
			panic("invalid health score: " + config.HealthScore)
		}
		return health
	})
	mud.Provide[*Observer](ball, func(log *zap.Logger, repairQueue queue.RepairQueue, overlay *overlay.Service, placements nodeselection.PlacementDefinitions, config Config, config2 overlay.Config, health Health) *Observer {
		if len(config.RepairExcludedCountryCodes) == 0 {
			config.RepairExcludedCountryCodes = config2.RepairExcludedCountryCodes
		}

		return NewObserver(
			log,
			repairQueue,
			overlay,
			placements,
			config,
			health,
		)
	})
	mud.Implementation[[]rangedloop.Observer, *Observer](ball)
	mud.Tag[*Observer, mud.Optional](ball, mud.Optional{})
}
