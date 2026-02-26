// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"storj.io/storj/satellite/metabase/rangedloop"
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
	mud.Provide[*ExpansionFactor](ball, NewExpansionFactor)
	mud.Tag[*ExpansionFactor, mud.Optional](ball, mud.Optional{})
	mud.Implementation[[]rangedloop.Observer, *ExpansionFactor](ball)

	config.RegisterConfig[ColdLegacyStatConfig](ball, "nodeaudit.cold-legacy-stat")
	mud.Provide[*ColdLegacyStat](ball, NewColdLegacyStat)
	mud.Tag[*ColdLegacyStat, mud.Optional](ball, mud.Optional{})
	mud.Implementation[[]rangedloop.Observer, *ColdLegacyStat](ball)
}
