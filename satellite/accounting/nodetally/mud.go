// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetally

import (
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Observer](ball, NewObserver)
	mud.Implementation[[]rangedloop.Observer, *Observer](ball)
	config.RegisterConfig[Config](ball, "node-tally")
	mud.Tag[*Observer, mud.Optional](ball, mud.Optional{})

}
