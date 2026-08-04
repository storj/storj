// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/gc/sender"
	"storj.io/storj/shared/mud"
)

// GcSender is a subcommand to start only the garbage collection sender service,
// which reads bloom filters from a bucket and forwards them to storage nodes.
type GcSender struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *GcSender) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*sender.Service](ball))
}
