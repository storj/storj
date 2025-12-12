// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/shared/mud"
)

// Api is a subcommand to start only API services.
type Api struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Api) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*satellite.EndpointRegistration](ball),
		mud.Select[*orders.Chore](ball),
	)
}
