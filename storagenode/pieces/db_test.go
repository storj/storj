// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPieceInfo(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		pieceinfos := db.PieceInfo()

		satellite0 := testplanet.MustPregeneratedSignedIdentity(0)
		satellite1 := testplanet.MustPregeneratedSignedIdentity(1)
		satellite2 := testplanet.MustPregeneratedSignedIdentity(2)

		uplink0 := testplanet.MustPregeneratedSignedIdentity(3)
		uplink1 := testplanet.MustPregeneratedSignedIdentity(4)
		uplink2 := testplanet.MustPregeneratedSignedIdentity(5)

		pieceid0 := storj.NewPieceID()

		now := time.Now()

		piecehash0, err := signing.SignPieceHash(
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
			PieceExpiration: &now,

			UplinkPieceHash: piecehash0,
			Uplink:          uplink0.PeerIdentity(),
		}

		piecehash1, err := signing.SignPieceHash(
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
			PieceExpiration: &now,

			UplinkPieceHash: piecehash1,
			Uplink:          uplink1.PeerIdentity(),
		}

		piecehash2, err := signing.SignPieceHash(
			signing.SignerFromFullIdentity(uplink2),
			&pb.PieceHash{
				PieceId: pieceid0,
				Hash:    []byte{1, 2, 3, 4, 5},
			})
		require.NoError(t, err)

		info2 := &pieces.Info{
			SatelliteID: satellite2.ID,

			PieceID:         pieceid0,
			PieceSize:       123,
			PieceExpiration: &now,

			UplinkPieceHash: piecehash2,
			Uplink:          uplink2.PeerIdentity(),
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

		// getting expired pieces
		exp := time.Now().Add(time.Hour * 24 * 1)
		infoexp, err := pieceinfos.GetExpired(ctx, exp)
		assert.NoError(t, err)
		assert.NotEmpty(t, infoexp)

		// deleting
		err = pieceinfos.Delete(ctx, info0.SatelliteID, info0.PieceID)
		require.NoError(t, err)
		err = pieceinfos.Delete(ctx, info1.SatelliteID, info1.PieceID)
		require.NoError(t, err)

		// deleting expired pieces
		err = pieceinfos.DeleteExpired(ctx, exp, info2.SatelliteID, info2.PieceID)
		require.NoError(t, err)

		// getting after delete
		_, err = pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
		require.Error(t, err)
		_, err = pieceinfos.Get(ctx, info1.SatelliteID, info1.PieceID)
		require.Error(t, err)
	})
}
