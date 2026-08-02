// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module registers the nodeaudit components.
func Module(ball *mud.Ball) {
	config.RegisterConfig[PieceListConfig](ball, "nodeaudit.piece-list")
	mud.Provide[*PieceList](ball, NewPieceList)
	mud.Tag[*PieceList, mud.Optional](ball, mud.Optional{})
	mud.Implementation[[]rangedloop.Observer, *PieceList](ball)

	config.RegisterConfig[ExpansionFactorConfig](ball, "nodeaudit.expansion-factor")
	mud.Provide[*ExpansionFactor](ball, func(log *zap.Logger, overlayService *overlay.Service, placements nodeselection.PlacementDefinitions, config ExpansionFactorConfig, overlayConfig overlay.Config) *ExpansionFactor {
		// Fall back to the same list repair uses, so the healthy classification here
		// matches the repair checker instead of silently skipping the excluded-country
		// check. Mirrors satellite/repair/checker.Module.
		if len(config.ExcludedCountryCodes) == 0 {
			config.ExcludedCountryCodes = overlayConfig.RepairExcludedCountryCodes
		}
		return NewExpansionFactor(log, overlayService, placements, config)
	})
	mud.Tag[*ExpansionFactor, mud.Optional](ball, mud.Optional{})
	mud.Implementation[[]rangedloop.Observer, *ExpansionFactor](ball)

	config.RegisterConfig[ColdLegacyStatConfig](ball, "nodeaudit.cold-legacy-stat")
	mud.Provide[*ColdLegacyStat](ball, NewColdLegacyStat)
	mud.Tag[*ColdLegacyStat, mud.Optional](ball, mud.Optional{})
	mud.Implementation[[]rangedloop.Observer, *ColdLegacyStat](ball)

	config.RegisterConfig[PieceCountHistogramConfig](ball, "nodeaudit.piece-count-histogram")
	mud.Provide[*PieceCountHistogram](ball, NewPieceCountHistogram)
	mud.Tag[*PieceCountHistogram, mud.Optional](ball, mud.Optional{})
	mud.Implementation[[]rangedloop.Observer, *PieceCountHistogram](ball)

	config.RegisterConfig[PieceAuditConfig](ball, "nodeaudit")
	mud.Provide[*PieceAudit](ball, NewChecker)
}
