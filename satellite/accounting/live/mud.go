// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[accounting.Cache](ball, OpenCache)
	config.RegisterConfig[Config](ball, "live-accounting")
}
