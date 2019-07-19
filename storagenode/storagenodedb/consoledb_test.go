// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/storagenode/console"
)

func TestConsoledb_Trivial(t *testing.T) {
	Run(t, func(t *testing.T, ctx context.Context, db *DB) {
		satelliteID := testrand.NodeID()
		now := time.Now()

		{ // Ensure Satellites GetIDs works at all
			_, err := db.Console().Satellites().GetIDs(ctx, now, now)
			require.NoError(t, err)
		}

		{ // Ensure Bandwidth GetDailyTotal works at all
			_, err := db.Console().Bandwidth().GetDailyTotal(ctx, now, now)
			require.NoError(t, err)
		}

		{ // Ensure Bandwidth GetDaily works at all
			_, err := db.Console().Bandwidth().GetDaily(ctx, satelliteID, now, now)
			require.NoError(t, err)
		}

		{ // Ensure DiskSpaceUsages Store works at all
			usages := []console.DiskSpaceUsage{
				{
					SatelliteID: satelliteID,
					Timestamp:   now,
				},
			}

			err := db.Console().DiskSpaceUsages().Store(ctx, usages)
			require.NoError(t, err)
		}

		{ // Ensure DiskSpaceUsages GetDaily works at all
			_, err := db.Console().DiskSpaceUsages().GetDaily(ctx, satelliteID, now, now)
			require.NoError(t, err)
		}

		{ // Ensure DiskSpaceUsages GetDailyTotal works at all
			_, err := db.Console().DiskSpaceUsages().GetDailyTotal(ctx, now, now)
			require.NoError(t, err)
		}

		{ // Ensure Stats Create works at all
			stats := console.NodeStats{
				SatelliteID: satelliteID,
				UpdatedAt:   now,
			}

			_, err := db.Console().Stats().Create(ctx, stats)
			require.NoError(t, err)
		}

		{ // Ensure Stats Get works at all
			_, err := db.Console().Stats().Get(ctx, satelliteID)
			require.NoError(t, err)
		}

		{ // Ensure Stats Update works at all
			stats := console.NodeStats{
				SatelliteID: satelliteID,
				UpdatedAt:   now,
			}

			err := db.Console().Stats().Update(ctx, stats)
			require.NoError(t, err)
		}
	})
}
