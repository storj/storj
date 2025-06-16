// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package repaircsv

import (
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud Module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*Queue](ball, NewQueue)
	config.RegisterConfig[Config](ball, "csv")
	mud.RegisterInterfaceImplementation[queue.Consumer, *Queue](ball)
}
