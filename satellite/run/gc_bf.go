// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/mud"
)

// GcBf is a subcommand to start only a ranged loop with BF generation.
type GcBf struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *GcBf) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*rangedloop.Service](ball))
}
