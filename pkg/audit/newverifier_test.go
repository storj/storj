// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
)

func TestVerifierHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Skip("flaky")

		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err = rand.Read(testData)
		require.NoError(t, err)

		err = uplink.Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointerdb := planet.Satellites[0].Metainfo.Service
		overlay := planet.Satellites[0].Overlay.Service
		cursor := audit.NewCursor(pointerdb)

		var stripe *audit.Stripe
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			stripe, err = cursor.NextStripe(ctx)
			if stripe != nil || err != nil {
				break
			}
		}
		require.NoError(t, err)
		require.NotNil(t, stripe, "unable to get stripe; likely no pointers in pointerdb")

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

		// remove stopped nodes from overlay cache
		err = planet.Satellites[0].Overlay.Service.Delete(ctx, planet.StorageNodes[0].ID())
		require.NoError(t, err)
		err = planet.Satellites[0].Overlay.Service.Delete(ctx, planet.StorageNodes[1].ID())
		require.NoError(t, err)

		_, err = verifier.Verify(ctx, stripe)
		require.NoError(t, err)
	})
}
