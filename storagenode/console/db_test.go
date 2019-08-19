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

		satelliteID := testrand.NodeID()
		now := time.Now()

		{ // Ensure Bandwidth GetDailyTotal works at all
			_, err := db.Console().Bandwidth().GetDailyTotal(ctx, now, now)
			require.NoError(t, err)
		}

		{ // Ensure Bandwidth GetDaily works at all
			_, err := db.Console().Bandwidth().GetDaily(ctx, satelliteID, now, now)
			require.NoError(t, err)
		}
	})
}
