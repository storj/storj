// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/shared/mud"
)

// Repair is a subcommand to start only Repairer (worker) service.
type Repair struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Repair) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*repairer.Service](ball))
}
