// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.View[DB, APIKeys](ball, DB.APIKeys)
	mud.View[DB, Projects](ball, DB.Projects)
	mud.View[DB, ProjectMembers](ball, DB.ProjectMembers)
	mud.View[DB, Users](ball, DB.Users)
	config.RegisterConfig[Config](ball, "console")
}
