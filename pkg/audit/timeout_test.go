// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// TestGetShareTimeout should test that getShare calls
// will have context canceled if it takes too long to
// receive data back from a storage node.
func TestGetShareTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		metainfo := planet.Satellites[0].Metainfo.Service
		overlay := planet.Satellites[0].Overlay.Service
		cursor := audit.NewCursor(metainfo)

		var stripe *audit.Stripe
		stripe, _, err = cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		network := &transport.SimulatedNetwork{
			DialLatency:    200 * time.Second,
			BytesPerSecond: 1 * memory.KB,
		}

		slowClient := network.NewClient(planet.Satellites[0].Transport)
		require.NotNil(t, slowClient)

		// This config value will create a very short timeframe allowed for receiving
		// data from storage nodes. This will cause context to cancel and start
		// downloading from new nodes.
		minBytesPerSecond := 110 * memory.KB
		orders := planet.Satellites[0].Orders.Service
		verifier := audit.NewVerifier(zap.L(), slowClient, overlay, orders, planet.Satellites[0].Identity, minBytesPerSecond)
		require.NotNil(t, verifier)

		// stop some storage nodes to ensure audit can deal with it
		pieces := stripe.Segment.GetRemote().GetRemotePieces()
		k := int(stripe.Segment.GetRemote().GetRedundancy().GetMinReq())
		for i := k; i < len(pieces); i++ {
			id := pieces[i].NodeId
			err = stopStorageNode(planet, id)
			require.NoError(t, err)

			// mark stopped node as offline in overlay cache
			_, err = planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, id, false)
			require.NoError(t, err)
		}

		_, err = verifier.Verify(ctx, stripe)
		require.NoError(t, err)
	})
}

func stopStorageNode(planet *testplanet.Planet, nodeID storj.NodeID) error {
	for _, node := range planet.StorageNodes {
		if node.ID() == nodeID {
			return planet.StopPeer(node)
		}
	}
	return fmt.Errorf("no such node: %s", nodeID.String())
}
