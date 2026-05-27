// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package webhook

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "webhook")
	mud.Provide[*Service](ball, New)
}
