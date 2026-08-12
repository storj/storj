// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module provides dependency injection configuration for garbage collection bloom filter components.
func Module(ball *mud.Ball) {

	config.RegisterConfig[Config](ball, "garbage-collection-bf")

	mud.Provide[*Observer](ball, NewObserver)
	mud.Implementation[[]rangedloop.Observer, *Observer](ball)
	mud.Tag[*Observer, mud.Optional](ball, mud.Optional{})
}
