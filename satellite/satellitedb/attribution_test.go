// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUpdateValueAttributionPlacement(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		bucketName := testrand.BucketName()
		projectID := testrand.UUID()

		vaDB := db.Attribution()

		info, err := vaDB.Insert(ctx, &attribution.Info{
			ProjectID:  projectID,
			BucketName: []byte(bucketName),
			UserAgent:  []byte("test"),
		})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Nil(t, info.Placement)

		newPlacement := storj.PlacementConstraint(1)

		err = vaDB.UpdatePlacement(ctx, projectID, bucketName, &newPlacement)
		require.NoError(t, err)

		info, err = vaDB.Get(ctx, projectID, []byte(bucketName))
		require.NoError(t, err)
		require.NotNil(t, info)
		require.NotNil(t, info.Placement)
		require.Equal(t, newPlacement, *info.Placement)

		err = vaDB.UpdatePlacement(ctx, projectID, bucketName, nil)
		require.NoError(t, err)

		info, err = vaDB.Get(ctx, projectID, []byte(bucketName))
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Nil(t, info.Placement)
	})
}
