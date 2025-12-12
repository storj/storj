// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*Endpoint](ball, NewEndpoint)
	config.RegisterConfig[Config](ball, "graceful-exit")
}
