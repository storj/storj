// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.View[DB, APIKeys](ball, DB.APIKeys)
	mud.View[DB, APIKeyTails](ball, DB.APIKeyTails)
	mud.View[DB, Projects](ball, DB.Projects)
	mud.View[DB, ProjectMembers](ball, DB.ProjectMembers)
	mud.View[DB, Users](ball, DB.Users)

	mud.View[DB, restapikeys.DB](ball, DB.RestApiKeys)
	mud.Provide[*AccountFreezeService](ball, NewAccountFreezeService)
	mud.View[Config, AccountFreezeConfig](ball, func(c Config) AccountFreezeConfig {
		return c.AccountFreeze
	})

}
