// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

func TestContainIncrementAndGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		input := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
			ReverifyCount:     0,
		}

		err := planet.Satellites[0].DB.Containment().IncrementPending(ctx, input)
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
		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
			ReverifyCount:     0,
		}

		err := planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
		require.NoError(t, err)

		info2 := &audit.PendingAudit{
			NodeID:            info1.NodeID,
			PieceID:           storj.PieceID{},
			StripeIndex:       1,
			ShareSize:         1,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
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
		info1 := &audit.PendingAudit{
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       0,
			ShareSize:         0,
			ExpectedShareHash: pkcrypto.SHA256Hash(testrand.Bytes(10)),
			ReverifyCount:     0,
		}

		err := planet.Satellites[0].DB.Containment().IncrementPending(ctx, info1)
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
