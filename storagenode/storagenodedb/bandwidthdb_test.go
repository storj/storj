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
)

func TestBandwidthdb_Trivial(t *testing.T) {
	Run(t, func(t *testing.T, ctx context.Context, db *DB) {
		{ // Ensure Add works at all
			err := db.Bandwidth().Add(ctx, testrand.NodeID(), pb.PieceAction_GET, 0, time.Now())
			require.NoError(t, err)
		}

		{ // Ensure MonthSummary works at all
			_, err := db.Bandwidth().MonthSummary(ctx)
			require.NoError(t, err)
		}

		{ // Ensure Summary works at all
			_, err := db.Bandwidth().Summary(ctx, time.Now(), time.Now())
			require.NoError(t, err)
		}

		{ // Ensure SummaryBySatellite works at all
			_, err := db.Bandwidth().SummaryBySatellite(ctx, time.Now(), time.Now())
			require.NoError(t, err)
		}
	})
}
