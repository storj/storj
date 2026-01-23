// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package sender

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
	config.RegisterConfig[Config](ball, "garbage-collection")
}
