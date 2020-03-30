// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"fmt"

	"storj.io/common/storj"
	"storj.io/storj/private/testplanet"
)

func stopStorageNode(ctx context.Context, planet *testplanet.Planet, nodeID storj.NodeID) error {
	node := planet.FindNode(nodeID)
	if node == nil {
		return fmt.Errorf("no such node: %s", nodeID)
	}

	err := planet.StopPeer(node)
	if err != nil {
		return err
	}

	// mark stopped node as offline in overlay
	_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, nodeID, false)
	return err
}
