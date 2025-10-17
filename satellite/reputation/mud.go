// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
	config.RegisterConfig[Config](ball, "reputation")
	mud.Provide[*CachingDB](ball, NewCachingDB)
	mud.RegisterInterfaceImplementation[DB, *CachingDB](ball)
}
