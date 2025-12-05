// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package mailservice

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "mail")
}
