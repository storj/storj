// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite/gc/piecetracker"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/overlay"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	rangedloop.Module(ball)
	metainfo.Module(ball)
	metabase.Module(ball)
	piecetracker.Module(ball)
	mud.View[DB, overlay.DB](ball, DB.OverlayCache)

}
