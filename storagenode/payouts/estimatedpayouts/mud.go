// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package estimatedpayouts

import (
	"storj.io/storj/shared/mud"
)

// Module registers the estimated payouts service dependency injection components.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
}
