// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

func TestReverifyContainedNodes(t *testing.T) {
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

		randBytes := make([]byte, 10)
		_, err = rand.Read(randBytes)
		require.NoError(t, err)
		someHash := pkcrypto.SHA256Hash(randBytes)

		for _, node := range planet.StorageNodes {
			pending := &audit.PendingAudit{
				NodeID:            node.ID(),
				PieceID:           storj.PieceID{},
				StripeIndex:       0,
				ShareSize:         0,
				ExpectedShareHash: someHash,
				ReverifyCount:     0,
			}

			err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, pending)
			require.NoError(t, err)
		}

		metainfo := planet.Satellites[0].Metainfo.Service
		overlay := planet.Satellites[0].Overlay.Service
		cursor := audit.NewCursor(metainfo)

		var stripe *audit.Stripe
		stripe, _, err = cursor.NextStripe(ctx)
		require.NoError(t, err)
		require.NotNil(t, stripe)

		transport := planet.Satellites[0].Transport
		orders := planet.Satellites[0].Orders.Service
		containment := planet.Satellites[0].DB.Containment()
		minBytesPerSecond := 128 * memory.B
		reporter := audit.NewReporter(overlay, containment, 1)
		verifier := audit.NewVerifier(zap.L(), reporter, transport, overlay, containment, orders, planet.Satellites[0].Identity, minBytesPerSecond)
		require.NotNil(t, verifier)

		_, err = verifier.Verify(ctx, stripe, nil)
		require.True(t, audit.ErrNotEnoughShares.Has(err))
	})
}

func TestContainIncrementAndGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		randBytes := make([]byte, 10)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)
		someHash := pkcrypto.SHA256Hash(randBytes)

		input := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: someHash,
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, input)
		require.NoError(t, err)

		output, err := planet.Satellites[0].DB.Containment().Get(ctx, input.NodeID)
		require.NoError(t, err)

		require.Equal(t, input, output)

		// check contained flag set to true
		node, err := planet.Satellites[0].DB.OverlayCache().Get(ctx, input.NodeID)
		require.NoError(t, err)
		require.True(t, node.Contained)

		nodeID1 := planet.StorageNodes[1].ID()
		_, err = planet.Satellites[0].DB.Containment().Get(ctx, nodeID1)
		require.Error(t, err, audit.ErrContainedNotFound.New(nodeID1.String()))
		require.True(t, audit.ErrContainedNotFound.Has(err))
	})
}

func TestContainIncrementPendingEntryExists(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		randBytes := make([]byte, 10)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)
		hash1 := pkcrypto.SHA256Hash(randBytes)

		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: hash1,
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
		require.NoError(t, err)

		randBytes = make([]byte, 10)
		_, err = rand.Read(randBytes)
		require.NoError(t, err)
		hash2 := pkcrypto.SHA256Hash(randBytes)

		info2 := &audit.PendingAudit{
			NodeID:            info1.NodeID,
			PieceID:           storj.PieceID{},
			StripeIndex:       1,
			ShareSize:         1,
			ExpectedShareHash: hash2,
			ReverifyCount:     0,
		}

		// expect failure when an entry with the same nodeID but different expected share data already exists
		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info2)
		require.Error(t, err)
		require.True(t, audit.ErrAlreadyExists.Has(err))

		// expect reverify count for an entry to be 0 after first IncrementPending call
		pending, err := planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.EqualValues(t, 0, pending.ReverifyCount)

		// expect reverify count to be 1 after second IncrementPending call
		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
		require.NoError(t, err)
		pending, err = planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.EqualValues(t, 1, pending.ReverifyCount)
	})
}

func TestContainDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		randBytes := make([]byte, 10)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)
		hash1 := pkcrypto.SHA256Hash(randBytes)

		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: hash1,
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
		require.NoError(t, err)

		// check contained flag set to true
		node, err := planet.Satellites[0].DB.OverlayCache().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.True(t, node.Contained)

		isDeleted, err := planet.Satellites[0].DB.Containment().Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		require.True(t, isDeleted)

		// check contained flag set to false
		node, err = planet.Satellites[0].DB.OverlayCache().Get(ctx, info1.NodeID)
		require.NoError(t, err)
		require.False(t, node.Contained)

		// get pending audit that doesn't exist
		_, err = planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.Error(t, err, audit.ErrContainedNotFound.New(info1.NodeID.String()))
		require.True(t, audit.ErrContainedNotFound.Has(err))

		// delete pending audit that doesn't exist
		isDeleted, err = planet.Satellites[0].DB.Containment().Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		require.False(t, isDeleted)
	})
}
