// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/shared/mud"
)

// ChangeStream is a subcommand to start only Repairer (worker) service.
type ChangeStream struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *ChangeStream) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*eventing.Service](ball))
}
