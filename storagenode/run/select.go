// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular/profiler"
	"storj.io/storj/shared/modular/tracing"
	"storj.io/storj/shared/mud"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/retain"
)

// Select is a subcommand to start select specific version of storagenode.
type Select struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Select) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	mud.ReplaceDependency[piecestore.PieceBackend, *piecestore.HashStoreBackend](ball)
	mud.DisableImplementation[monitor.DiskVerification](ball)
	mud.Tag[*retain.Service, mud.Optional](ball, mud.Optional{})
	return mud.Or(
		mud.Select[debug.Wrapper](ball),
		mud.Select[*profiler.Profiler](ball),
		mud.Select[*tracing.Tracing](ball),
		mud.Select[*storagenode.EndpointRegistration](ball),
		mud.Select[*contact.Endpoint](ball),
		mud.Select[*contact.Chore](ball),
		mud.Select[*orders.Service](ball),
		mud.Select[*reputation.Service](ball),
		mud.Select[*reputation.Chore](ball),
	)
}
