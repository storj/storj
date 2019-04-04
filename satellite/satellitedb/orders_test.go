// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSerialNumbers(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		orders := db.Orders()

		expectedBucket := []byte("bucketID")
		err := orders.CreateSerialInfo(ctx, storj.SerialNumber{1}, expectedBucket, time.Now())
		require.NoError(t, err)

		bucketID, err := orders.UseSerialNumber(ctx, storj.SerialNumber{1}, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, expectedBucket, bucketID)

		_, err = orders.UseSerialNumber(ctx, storj.SerialNumber{1}, storj.NodeID{1})
		require.Error(t, err)

		err = orders.UnuseSerialNumber(ctx, storj.SerialNumber{1}, storj.NodeID{1})
		require.NoError(t, err)

		bucketID, err = orders.UseSerialNumber(ctx, storj.SerialNumber{1}, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, expectedBucket, bucketID)

		// not existing serial number
		bucketID, err = orders.UseSerialNumber(ctx, storj.SerialNumber{99}, storj.NodeID{1})
		require.Error(t, err)
		require.Contains(t, err.Error(), "serial number not found")
		require.Empty(t, bucketID)
	})
}
