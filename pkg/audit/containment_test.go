// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

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
			NodeID:            planet.StorageNodes[0].ID(),
			PieceID:           storj.PieceID{},
			StripeIndex:       1,
			ShareSize:         1,
			ExpectedShareHash: hash2,
			ReverifyCount:     0,
		}

		err = planet.Satellites[0].DB.Containment().IncrementPending(ctx, info2)
		require.NoError(t, err)

		pending, err := planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.NoError(t, err)

		require.Equal(t, uint32(1), pending.ReverifyCount)

		// Even though info2 had different pending audit info,
		// we only need to save the original pending audit info
		// since the node ID is the same.
		require.Equal(t, info1.ExpectedShareHash, pending.ExpectedShareHash)
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

		isDeleted, err := planet.Satellites[0].DB.Containment().Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		require.True(t, isDeleted)

		_, err = planet.Satellites[0].DB.Containment().Get(ctx, info1.NodeID)
		require.Error(t, err, audit.ErrContainedNotFound.New(info1.NodeID.String()))
		require.True(t, audit.ErrContainedNotFound.Has(err))

		isDeleted, err = planet.Satellites[0].DB.Containment().Delete(ctx, info1.NodeID)
		require.NoError(t, err)
		require.False(t, isDeleted)
	})
}
