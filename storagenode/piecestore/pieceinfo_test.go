// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPieceinfo_Size(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

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
	})
}
