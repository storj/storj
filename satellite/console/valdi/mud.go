// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package valdi

import (
	"storj.io/storj/satellite/console/valdi/valdiclient"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "valdi")
	mud.Provide[*Service](ball, NewService)
	mud.View[Config, valdiclient.Config](ball, func(c Config) valdiclient.Config {
		return c.Config
	})
	mud.Tag[*Service](ball, mud.Optional{})
	mud.Tag[*Service](ball, mud.Nullable{})
}
