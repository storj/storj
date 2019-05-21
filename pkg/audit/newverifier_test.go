// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/uplink"
)

func TestVerifierHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		ul := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = ul.UploadWithConfig(ctx, planet.Satellites[0], &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  5,
			SuccessThreshold: 6,
			MaxThreshold:     6,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		metainfo := planet.Satellites[0].Metainfo.Service
		overlay := planet.Satellites[0].Overlay.Service
		cursor := audit.NewCursor(metainfo)

		stripe, _, err := cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		fmt.Println("all nodes:")
		for _, node := range planet.StorageNodes {
			fmt.Println(node.ID())
		}

		fmt.Println("nodes associated with stripe:")
		for _, piece := range stripe.Segment.GetRemote().GetRemotePieces() {
			fmt.Println(piece.NodeId)
		}

		transport := planet.Satellites[0].Transport
		orders := planet.Satellites[0].Orders.Service
		minBytesPerSecond := 128 * memory.B
		verifier := audit.NewVerifier(zap.L(), transport, overlay, orders, planet.Satellites[0].Identity, minBytesPerSecond)
		require.NotNil(t, verifier)

		// stop some storage nodes to ensure audit can deal with it
		err = planet.StopPeer(planet.StorageNodes[0])
		require.NoError(t, err)
		err = planet.StopPeer(planet.StorageNodes[1])
		require.NoError(t, err)

		fmt.Println("stopped nodes:")
		fmt.Println(planet.StorageNodes[0].ID())
		fmt.Println(planet.StorageNodes[1].ID())

		// mark stopped nodes as offline in overlay cache
		_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, planet.StorageNodes[0].ID(), false)
		require.NoError(t, err)
		_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, planet.StorageNodes[1].ID(), false)
		require.NoError(t, err)

		// get nodes from overlay to see if they're marked as offline
		node0, err := planet.Satellites[0].Overlay.Service.Get(ctx, planet.StorageNodes[0].ID())
		require.NoError(t, err)

		isOnline := planet.Satellites[0].Overlay.Service.IsOnline(node0)
		fmt.Println(planet.StorageNodes[0].ID(), "online:", isOnline)

		node1, err := planet.Satellites[0].Overlay.Service.Get(ctx, planet.StorageNodes[1].ID())
		require.NoError(t, err)

		isOnline = planet.Satellites[0].Overlay.Service.IsOnline(node1)
		fmt.Println(planet.StorageNodes[1].ID(), "online:", isOnline)

		verifiedNodes, err := verifier.Verify(ctx, stripe)
		require.NoError(t, err)

		require.Len(t, verifiedNodes.SuccessNodeIDs, 4)
		require.Len(t, verifiedNodes.FailNodeIDs, 0)

		fmt.Println("success nodes:")
		for _, id := range verifiedNodes.SuccessNodeIDs {
			fmt.Println(id)
		}

		fmt.Println("offline nodes:")
		for _, id := range verifiedNodes.OfflineNodeIDs {
			fmt.Println(id)
		}
		require.Len(t, verifiedNodes.OfflineNodeIDs, 0)
	})
}
