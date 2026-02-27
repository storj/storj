// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package jobq

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud Module definition.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "jobq")
	mud.Provide[*RepairJobQueue](ball, OpenJobQueue)
}
