// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqueue

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module registers jobqueue configuration with the mud dependency injection framework.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "queue")
}
