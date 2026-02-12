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
}
