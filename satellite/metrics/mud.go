// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*Observer](ball, NewObserver)
	mud.Implementation[[]rangedloop.Observer, *Observer](ball)
	mud.Tag[*Observer, mud.Optional](ball, mud.Optional{})
}
