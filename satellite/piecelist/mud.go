// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecelist

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite/metabase/rangedloop"
)

func Module(ball *mud.Ball) {
	mud.Provide[*PieceList](ball, NewPieceList)
	mud.Implementation[[]rangedloop.Observer, *PieceList](ball)

}
