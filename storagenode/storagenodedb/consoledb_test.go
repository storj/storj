// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
)

func TestConsoledb_Trivial(t *testing.T) {
	Run(t, func(t *testing.T, ctx context.Context, db *DB) {
		{ // Ensure GetSatelliteIDs works at all
			_, err := db.Console().GetSatelliteIDs(ctx, time.Now(), time.Now())
			require.NoError(t, err)
		}

		{ // Ensure GetDailyTotalBandwidthUsed works at all
			_, err := db.Console().GetDailyTotalBandwidthUsed(ctx, time.Now(), time.Now())
			require.NoError(t, err)
		}

		{ // Ensure GetDailyBandwidthUsed works at all
			_, err := db.Console().GetDailyBandwidthUsed(ctx, testrand.NodeID(), time.Now(), time.Now())
			require.NoError(t, err)
		}
	})
}
