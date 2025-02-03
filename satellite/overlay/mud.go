// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "overlay")
	mud.View[Config, NodeSelectionConfig](ball, func(c Config) NodeSelectionConfig {
		return c.Node
	})
}
