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
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
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

func TestBackfillPlacementBatch(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		bmDB := db.Buckets()
		vaDB := db.Attribution()

		project, err := db.Console().Projects().Insert(ctx, &console.Project{Name: "test"})
		require.NoError(t, err)

		// Prepare four bucket names: three with nil initial placement, one pre-filled.
		bucketNames := []string{
			testrand.BucketName(),
			testrand.BucketName(),
			testrand.BucketName(),
			testrand.BucketName(), // this one gets initial non-null placement.
		}
		bmPlacements := []storj.PlacementConstraint{
			storj.PlacementConstraint(1),
			storj.PlacementConstraint(2),
			storj.PlacementConstraint(3),
			storj.PlacementConstraint(42),
		}
		// Value attribution initial placements: nil for first three, non-nil for the fourth.
		vaInitial := map[string]*storj.PlacementConstraint{
			bucketNames[0]: nil,
			bucketNames[1]: nil,
			bucketNames[2]: nil,
			bucketNames[3]: &bmPlacements[3],
		}

		// Seed bucket_metainfos and value_attributions.
		for i, name := range bucketNames {
			_, err = bmDB.CreateBucket(ctx, buckets.Bucket{
				ProjectID: project.ID,
				Name:      name,
				Placement: bmPlacements[i],
			})
			require.NoError(t, err)

			_, err = vaDB.Insert(ctx, &attribution.Info{
				ProjectID:  project.ID,
				BucketName: []byte(name),
				Placement:  vaInitial[name],
			})
			require.NoError(t, err)
		}

		// First batch of size 2: should update exactly two of the nil-initial buckets.
		rows, hasNext, err := vaDB.BackfillPlacementBatch(ctx, 2)
		require.NoError(t, err)
		require.Equal(t, int64(2), rows)
		require.True(t, hasNext)

		// Fetch all attributions and count updated ones (non-nil).
		updated := make(map[string]storj.PlacementConstraint)
		for _, name := range bucketNames {
			info, err := vaDB.Get(ctx, project.ID, []byte(name))
			require.NoError(t, err)

			if info.Placement != nil {
				updated[name] = *info.Placement
			}
		}
		// Exactly two newly backfilled + the pre-filled one => 3 non-nil.
		require.Len(t, updated, 3)

		getIndex := func(slice []string, name string) int {
			for i, v := range slice {
				if v == name {
					return i
				}
			}
			return -1
		}

		// Verify that only the pre-filled bucket keeps its original value.
		for name, got := range updated {
			if name == bucketNames[3] {
				// the fourth bucket was pre-filled, should remain 42.
				require.Equal(t, bmPlacements[3], got)
			} else {
				// the other two updated should match their bmPlacements.
				want := bmPlacements[getIndex(bucketNames, name)]
				require.Equal(t, want, got)
			}
		}

		// Second batch: should update the remaining nil-initial bucket.
		rows, hasNext, err = vaDB.BackfillPlacementBatch(ctx, 2)
		require.NoError(t, err)
		require.Equal(t, int64(1), rows)
		require.False(t, hasNext)

		// Final check: all four buckets must now have their bmPlacements.
		for i, name := range bucketNames {
			info, err := vaDB.Get(ctx, project.ID, []byte(name))
			require.NoError(t, err)
			require.NotNil(t, info.Placement)
			require.Equal(t, bmPlacements[i], *info.Placement)
		}
	})
}
