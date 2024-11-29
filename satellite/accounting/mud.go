// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/private/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.View[*Service, Service](ball, mud.Dereference[Service])
}
