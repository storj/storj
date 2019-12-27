// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"fmt"

	"storj.io/common/storj"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
)

func getStorageNode(planet *testplanet.Planet, nodeID storj.NodeID) *storagenode.Peer {
	for _, node := range planet.StorageNodes {
		if node.ID() == nodeID {
			return node
		}
	}
	return nil
}

func stopStorageNode(ctx context.Context, planet *testplanet.Planet, nodeID storj.NodeID) error {
	node := getStorageNode(planet, nodeID)
	if node == nil {
		return fmt.Errorf("no such node: %s", nodeID.String())
	}

	err := planet.StopPeer(node)
	if err != nil {
		return err
	}

	// mark stopped node as offline in overlay
	_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, nodeID, false)
	return err
}
