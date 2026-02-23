// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	jobqserver "storj.io/storj/satellite/jobq/server"
	root "storj.io/storj/satellite/run"
	"storj.io/storj/shared/mud"
)

// Run is subcommand to start jobq
type Run struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Run) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		root.Observability(ball),
		mud.Select[*jobqserver.EndpointRegistration](ball),
	)
}
