// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
)

func TestVerifierHappyPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		err := planet.Satellites[0].Audit.Service.Close()
		assert.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := make([]byte, 5*memory.MiB)
		_, err = rand.Read(testData)
		assert.NoError(t, err)

		err = uplink.Upload(ctx, planet.Satellites[0], "test/bucket", "test/path", testData)
		assert.NoError(t, err)

		pointerdb := planet.Satellites[0].Metainfo.Service
		allocation := planet.Satellites[0].Metainfo.Allocation
		overlay := planet.Satellites[0].Overlay.Service
		cursor := audit.NewCursor(pointerdb, allocation, overlay, planet.Satellites[0].Identity)

		var stripe *audit.Stripe
		for {
			stripe, err = cursor.NextStripe(ctx)
			if stripe != nil || err != nil {
				break
			}
		}
		require.NoError(t, err)
		require.NotNil(t, stripe)

		transport := planet.Satellites[0].Transport

		minBytesPerSecond := 128 * memory.B

		verifier := audit.NewVerifier(zap.L(), transport, overlay, planet.Satellites[0].Identity, minBytesPerSecond)
		require.NotNil(t, verifier)

		err = planet.StopPeer(planet.StorageNodes[0])
		assert.NoError(t, err)
		err = planet.StopPeer(planet.StorageNodes[1])
		assert.NoError(t, err)

		_, err = verifier.Verify(ctx, stripe)
		assert.NoError(t, err)
	})
}
