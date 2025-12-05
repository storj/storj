// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
}
