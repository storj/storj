// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/eventing/changestream"
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/mud"
)

// ChangeStream is a subcommand to start only Repairer (worker) service.
type ChangeStream struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *ChangeStream) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		mud.Select[debug.Wrapper](ball),
		mud.Select[*changestream.Service](ball))
}
