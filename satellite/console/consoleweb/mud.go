// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "console")
}
