// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/audit"
	"storj.io/storj/shared/mud"
)

// Auditor is a subcommand to start only Auditor services.
type Auditor struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Auditor) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*audit.Worker](ball),
		mud.Select[*audit.ReverifyWorker](ball))

}
