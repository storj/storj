// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"storj.io/storj/internal/testidentity"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
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

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		satellite1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

		uplink0 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())
		uplink1 := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())

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

		_, err = pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
		require.Error(t, err, "getting element that doesn't exist")

		// adding stuff
		err = pieceinfos.Add(ctx, info0)
		require.NoError(t, err)

		err = pieceinfos.Add(ctx, info1)
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

		// deleting
		err = pieceinfos.Delete(ctx, info0.SatelliteID, info0.PieceID)
		require.NoError(t, err)
		err = pieceinfos.Delete(ctx, info1.SatelliteID, info1.PieceID)
		require.NoError(t, err)

		// getting after delete
		_, err = pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
		require.Error(t, err)
		_, err = pieceinfos.Get(ctx, info1.SatelliteID, info1.PieceID)
		require.Error(t, err)
	})
}
