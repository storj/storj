// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storagenode/pieces"
)

func TestPieceinfo_Trivial(t *testing.T) {
	Run(t, func(t *testing.T, ctx context.Context, db *DB) {
		satelliteID, pieceID := testrand.NodeID(), testrand.PieceID()

		{ // Ensure Add works at all
			err := db.PieceInfo().Add(ctx, &pieces.Info{
				SatelliteID:     satelliteID,
				PieceID:         pieceID,
				PieceCreation:   time.Now(),
				PieceExpiration: time.Now(),
				OrderLimit:      &pb.OrderLimit{},
				UplinkPieceHash: &pb.PieceHash{},
			})
			require.NoError(t, err)
		}

		{ // Ensure GetPieceIDs works at all
			_, err := db.PieceInfo().GetPieceIDs(ctx, satelliteID, time.Now(), 1, 0)
			require.NoError(t, err)
		}

		{ // Ensure Get works at all
			_, err := db.PieceInfo().Get(ctx, satelliteID, pieceID)
			require.NoError(t, err)
		}

		{ // Ensure DeleteFailed works at all
			err := db.PieceInfo().DeleteFailed(ctx, satelliteID, pieceID, time.Now())
			require.NoError(t, err)
		}

		{ // Ensure Delete works at all
			err := db.PieceInfo().Delete(ctx, satelliteID, pieceID)
			require.NoError(t, err)
		}

		{ // Ensure GetExpired works at all
			_, err := db.PieceInfo().GetExpired(ctx, time.Now(), 1)
			require.NoError(t, err)
		}

		{ // Ensure SpaceUsed works at all
			_, err := db.PieceInfo().SpaceUsed(ctx)
			require.NoError(t, err)
		}

		{ // Ensure CalculatedSpaceUsed works at all
			_, err := db.PieceInfo().CalculatedSpaceUsed(ctx)
			require.NoError(t, err)
		}

		{ // Ensure SpaceUsedBySatellite works at all
			_, err := db.PieceInfo().SpaceUsedBySatellite(ctx, satelliteID)
			require.NoError(t, err)
		}
	})
}
