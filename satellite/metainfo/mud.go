// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "metainfo")
	mud.View[Config, metabase.DatabaseConfig](ball, func(c Config) metabase.DatabaseConfig {
		return metabase.DatabaseConfig{
			URL: c.DatabaseURL,
			// TODO: application name should come from a config.
			Config: c.Metabase("satellite"),
		}
	})
}
