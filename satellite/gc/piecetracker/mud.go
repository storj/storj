// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecetracker

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Observer](ball, NewObserver)
	mud.Implementation[[]rangedloop.Observer, *Observer](ball)
	config.RegisterConfig[Config](ball, "piece-tracker")
}
