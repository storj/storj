// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular/tracing"
	"storj.io/storj/shared/mud"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/retain"
)

// Run is a subcommand to start the regular storagenode.
type Run struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Run) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		mud.Select[debug.Wrapper](ball),
		mud.Select[*tracing.Tracing](ball),
		mud.Select[*storagenode.EndpointRegistration](ball),
		mud.Select[*contact.Endpoint](ball),
		mud.Select[*contact.Chore](ball),
		mud.Select[*bandwidth.Service](ball),
		mud.Select[*retain.Service](ball),
		mud.Select[*monitor.Service](ball),
		mud.Select[*orders.Service](ball),
		mud.Select[*reputation.Chore](ball),
		mud.Select[*consoleserver.Server](ball),
	)
}
