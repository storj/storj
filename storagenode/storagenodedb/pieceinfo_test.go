// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb"
)

func TestPieceInfo(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := storagenodedb.NewInfoInMemory()
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NoError(t, db.CreateTables(log))

	pieceinfos := db.PieceInfo()

	satellite0 := testplanet.MustPregeneratedSignedIdentity(0)
	satellite1 := testplanet.MustPregeneratedSignedIdentity(1)

	uplink0 := testplanet.MustPregeneratedSignedIdentity(2)
	uplink1 := testplanet.MustPregeneratedSignedIdentity(3)

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
		PieceExpiration: now,

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
		PieceExpiration: now,

		UplinkPieceHash: piecehash1,
		Uplink:          uplink1.PeerIdentity(),
	}

	_, err = pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
	require.Error(t, err, "getting element that doesn't exist")

	err = pieceinfos.Add(ctx, info0)
	require.NoError(t, err)

	err = pieceinfos.Add(ctx, info1)
	require.NoError(t, err, "adding different satellite, but same pieceid")

	err = pieceinfos.Add(ctx, info0)
	require.Error(t, err, "adding duplicate")

	info0loaded, err := pieceinfos.Get(ctx, info0.SatelliteID, info0.PieceID)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(info0, info0loaded, cmp.Comparer(pb.Equal)))

	info1loaded, err := pieceinfos.Get(ctx, info1.SatelliteID, info1.PieceID)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(info1, info1loaded, cmp.Comparer(pb.Equal)))
}
