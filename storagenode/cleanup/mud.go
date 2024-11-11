// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*SafeLoop](ball, NewSafeLoop)
	mud.Provide[*Cleanup](ball, NewCleanup)
	mud.Provide[*CoreLoad](ball, NewCoreLoad)
	mud.RegisterInterfaceImplementation[Availability, *CoreLoad](ball)
	config.RegisterConfig[SafeLoopConfig](ball, "cleanup.loop")
	config.RegisterConfig[CoreLoadConfig](ball, "cleanup.load")
}
