// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecelist

import (
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*PieceList](ball, NewPieceList)
	mud.Implementation[[]rangedloop.Observer, *PieceList](ball)
	mud.Tag[*PieceList, mud.Optional](ball, mud.Optional{})
	config.RegisterConfig[Config](ball, "piecelist")

}
