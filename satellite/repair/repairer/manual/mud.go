// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package manual

import (
	"storj.io/storj/shared/mud"
)

// Module is a mud module that provides the manual repairer.
func Module(ball *mud.Ball) {
	mud.Provide[*Repairer](ball, NewRepairer)
}
