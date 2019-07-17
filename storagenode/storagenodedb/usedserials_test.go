// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
)

func TestUsedserials_Trivial(t *testing.T) {
	Run(t, func(t *testing.T, ctx context.Context, db *DB) {
		satelliteID, serial := testrand.NodeID(), testrand.SerialNumber()

		{ // Ensure Add works at all
			err := db.UsedSerials().Add(ctx, satelliteID, serial, time.Now())
			require.NoError(t, err)
		}

		{ // Ensure IterateAll works at all
			err := db.UsedSerials().IterateAll(ctx, func(storj.NodeID, storj.SerialNumber, time.Time) {})
			require.NoError(t, err)
		}

		{ // Ensure DeleteExpired works at all
			err := db.UsedSerials().DeleteExpired(ctx, time.Now())
			require.NoError(t, err)
		}
	})
}
