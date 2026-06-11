// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/accountfreeze"
	"storj.io/storj/satellite/admin"
	_ "storj.io/storj/satellite/admin/legacy/ui" // embed ui
	_ "storj.io/storj/satellite/admin/ui"        // embed ui
	"storj.io/storj/shared/mud"
)

// Admin defines the satellite admin configuration and component selection.
type Admin struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Admin) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*admin.Server](ball),
		mud.Select[*accountfreeze.Chore](ball),
		mud.Select[*accountfreeze.TrialFreezeChore](ball))
}
