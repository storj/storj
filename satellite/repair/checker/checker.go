// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"storj.io/storj/satellite/nodeselection"
)

// Overlay is used to fetch information about nodes to check for repairs.
type Overlay interface {
	// GetAllParticipatingNodesForRepair returns all known participating nodes (this includes all known
	// nodes excluding nodes that have been disqualified or gracefully exited).
	// The passed onlineWindow is used to determine whether each node is marked as Online.
	GetAllParticipatingNodesForRepair(_ context.Context, onlineWindow time.Duration) ([]nodeselection.SelectedNode, error)
}
