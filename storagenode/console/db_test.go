// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestDB_Trivial(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

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
