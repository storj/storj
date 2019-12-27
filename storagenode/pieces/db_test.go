// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestV0PieceInfo(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		pieceinfos := db.V0PieceInfo().(pieces.V0PieceInfoDBForTest)

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		satellite1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())
		satellite2 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())

		uplink0 := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())
		uplink1 := testidentity.MustPregeneratedSignedIdentity(4, storj.LatestIDVersion())
		uplink2 := testidentity.MustPregeneratedSignedIdentity(5, storj.LatestIDVersion())

		pieceid0 := storj.NewPieceID()

		now := time.Now()

		piecehash0, err := signing.SignPieceHash(ctx,
			signing.SignerFromFullIdentity(uplink0),
			&pb.PieceHash{
				PieceId: pieceid0,
				Hash:    []byte{1, 2, 3, 4, 5},
			})
		require.NoError(t, err)

		info0 := &pieces.Info{
			SatelliteID: satellite0.ID,

			PieceID:         pieceid0,
			PieceSize:       123,
			PieceCreation:   now,
			PieceExpiration: now,

			OrderLimit:      &pb.OrderLimit{},
			UplinkPieceHash: piecehash0,
		}

		piecehash1, err := signing.SignPieceHash(ctx,
			signing.SignerFromFullIdentity(uplink1),
			&pb.PieceHash{
				PieceId: pieceid0,
				Hash:    []byte{1, 2, 3, 4, 5},
			})
		require.NoError(t, err)

		info1 := &pieces.Info{
			SatelliteID: satellite1.ID,

			PieceID:         pieceid0,
			PieceSize:       123,
			PieceCreation:   now,
			PieceExpiration: now,

			OrderLimit:      &pb.OrderLimit{},
			UplinkPieceHash: piecehash1,
		}

		piecehash2, err := signing.SignPieceHash(ctx,
			signing.SignerFromFullIdentity(uplink2),
			&pb.PieceHash{
				PieceId: pieceid0,
				Hash:    []byte{1, 2, 3, 4, 5},
			})
		require.NoError(t, err)

		// use different timezones
		location := time.FixedZone("XYZ", int((8 * time.Hour).Seconds()))
		now2 := now.In(location)

		info2 := &pieces.Info{
			SatelliteID: satellite2.ID,

			PieceID:         pieceid0,
			PieceSize:       123,
			PieceCreation:   now2,
			PieceExpiration: now2,

			OrderLimit:      &pb.OrderLimit{},
			UplinkPieceHash: piecehash2,
		}

		_, err = pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
		require.Error(t, err, "getting element that doesn't exist")

		// adding stuff
		err = pieceinfos.Add(ctx, info0)
		require.NoError(t, err)

		err = pieceinfos.Add(ctx, info1)
		require.NoError(t, err, "adding different satellite, but same pieceid")

		err = pieceinfos.Add(ctx, info2)
		require.NoError(t, err, "adding different satellite, but same pieceid")

		err = pieceinfos.Add(ctx, info0)
		require.Error(t, err, "adding duplicate")

		// getting the added information
		info0loaded, err := pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(info0, info0loaded, cmp.Comparer(pb.Equal)))

		info1loaded, err := pieceinfos.Get(ctx, info1.SatelliteID, info1.PieceID)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(info1, info1loaded, cmp.Comparer(pb.Equal)))

		// getting no expired pieces
		expired, err := pieceinfos.GetExpired(ctx, now.Add(-10*time.Hour), 10)
		assert.NoError(t, err)
		assert.Len(t, expired, 0)

		// getting expired pieces
		exp := now.Add(8 * 24 * time.Hour)
		expired, err = pieceinfos.GetExpired(ctx, exp, 10)
		assert.NoError(t, err)
		assert.Len(t, expired, 3)

		// mark info0 deletion as a failure
		err = pieceinfos.DeleteFailed(ctx, info0.SatelliteID, info0.PieceID, exp)
		assert.NoError(t, err)

		// this shouldn't return info0
		expired, err = pieceinfos.GetExpired(ctx, exp, 10)
		assert.NoError(t, err)
		assert.Len(t, expired, 2)

		// deleting
		err = pieceinfos.Delete(ctx, info0.SatelliteID, info0.PieceID)
		require.NoError(t, err)
		err = pieceinfos.Delete(ctx, info1.SatelliteID, info1.PieceID)
		require.NoError(t, err)

		// deleting expired pieces
		err = pieceinfos.Delete(ctx, info2.SatelliteID, info2.PieceID)
		require.NoError(t, err)
		// duplicate deletion
		err = pieceinfos.Delete(ctx, info2.SatelliteID, info2.PieceID)
		require.NoError(t, err)

		// getting after delete
		_, err = pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
		require.Error(t, err)
		_, err = pieceinfos.Get(ctx, info1.SatelliteID, info1.PieceID)
		require.Error(t, err)
	})
}

func TestPieceinfo_Trivial(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		pieceinfos := db.V0PieceInfo().(pieces.V0PieceInfoDBForTest)
		satelliteID, pieceID := testrand.NodeID(), testrand.PieceID()

		{ // Ensure Add works at all
			err := pieceinfos.Add(ctx, &pieces.Info{
				SatelliteID:     satelliteID,
				PieceID:         pieceID,
				PieceCreation:   time.Now(),
				PieceExpiration: time.Now(),
				OrderLimit:      &pb.OrderLimit{},
				UplinkPieceHash: &pb.PieceHash{},
			})
			require.NoError(t, err)
		}

		{ // Ensure Get works at all
			_, err := pieceinfos.Get(ctx, satelliteID, pieceID)
			require.NoError(t, err)
		}

		{ // Ensure DeleteFailed works at all
			err := pieceinfos.DeleteFailed(ctx, satelliteID, pieceID, time.Now())
			require.NoError(t, err)
		}

		{ // Ensure Delete works at all
			err := pieceinfos.Delete(ctx, satelliteID, pieceID)
			require.NoError(t, err)
		}

		{ // Ensure GetExpired works at all
			_, err := pieceinfos.GetExpired(ctx, time.Now(), 1)
			require.NoError(t, err)
		}
	})
}
