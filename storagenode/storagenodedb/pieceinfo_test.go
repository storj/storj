// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

func TestPieceinfo_Size(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := NewInMemory(log, ctx.Dir("storage"))
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	err = db.CreateTables()
	if err != nil {
		t.Fatal(err)
	}

	var pieceID storj.PieceID
	satelliteID1, satelliteID2 := testrand.NodeID(), testrand.NodeID()

	check := func(sat1, sat2 int64) {
		t.Helper()
		used, err := db.PieceInfo().SpaceUsed(ctx)
		require.NoError(t, err)
		require.Equal(t, used, sat1+sat2)
		used, err = db.PieceInfo().SpaceUsedBySatellite(ctx, satelliteID1)
		require.NoError(t, err)
		require.Equal(t, used, sat1)
		used, err = db.PieceInfo().SpaceUsedBySatellite(ctx, satelliteID2)
		require.NoError(t, err)
		require.Equal(t, used, sat2)
	}

	for i := 0; i < 10; i++ {
		pieceID = testrand.PieceID()
		satelliteID := satelliteID1
		if i >= 5 {
			satelliteID = satelliteID2
		}

		require.NoError(t, db.PieceInfo().Add(ctx, &pieces.Info{
			SatelliteID:     satelliteID,
			PieceID:         pieceID,
			PieceSize:       10,
			PieceCreation:   time.Now(),
			OrderLimit:      new(pb.OrderLimit),
			UplinkPieceHash: new(pb.PieceHash),
		}))
	}

	check(50, 50)

	require.NoError(t, db.PieceInfo().Delete(ctx, satelliteID2, pieceID))

	check(50, 40)
}
