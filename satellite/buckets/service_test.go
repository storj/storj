// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

const TestBucket = "testbucket"
const TestObject = "testobject"

func TestBucketPlacement_EmptyBucket(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			buckets := satellite.API.Buckets.Service
			uplink := planet.Uplinks[0]
			projectID := uplink.Projects[0].ID

			// create new bucket
			err := uplink.TestingCreateBucket(ctx, satellite, TestBucket)
			require.NoError(t, err)

			// check that the placement is not set yet
			bucket, err := buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)

			// set bucket placement
			euPlacement := storj.PlacementConstraint(1)
			bucket.Placement = euPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.NoError(t, err)

			// check that the placement is now set
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Equal(t, euPlacement, bucket.Placement)

			// change bucket placement to new location
			usPlacement := storj.PlacementConstraint(3)
			bucket.Placement = usPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.NoError(t, err)

			// check that the placement is now at the new location
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Equal(t, usPlacement, bucket.Placement)

			// remove bucket placement constraints
			bucket.Placement = storj.DefaultPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.NoError(t, err)

			// check that the placement is not set anymore
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)
		},
	)
}

func TestBucketPlacement_SetOnNonEmptyBucket(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			buckets := satellite.API.Buckets.Service
			uplink := planet.Uplinks[0]
			projectID := uplink.Projects[0].ID

			// create new bucket
			err := uplink.TestingCreateBucket(ctx, satellite, TestBucket)
			require.NoError(t, err)

			// check that the placement is not set yet
			bucket, err := buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)

			// upload an empty object - just to have the bucket non-empty
			err = uplink.Upload(ctx, satellite, TestBucket, TestObject, []byte{})
			require.NoError(t, err)

			euPlacement := storj.PlacementConstraint(1)

			// set bucket placement - it should fail
			bucket.Placement = euPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.Error(t, err)

			// check that the placement is still not set
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)

			// delete the file
			err = uplink.DeleteObject(ctx, satellite, TestBucket, TestObject)
			require.NoError(t, err)

			// set bucket placement
			bucket.Placement = euPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.NoError(t, err)

			// check that the placement is now set
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Equal(t, euPlacement, bucket.Placement)
		},
	)
}

func TestBucketPlacement_ChangeOnNonEmptyBucket(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			buckets := satellite.API.Buckets.Service
			uplink := planet.Uplinks[0]
			projectID := uplink.Projects[0].ID

			euPlacement := storj.PlacementConstraint(1)
			usPlacement := storj.PlacementConstraint(3)
			everyCountryPlacement := storj.DefaultPlacement

			// create new bucket
			err := uplink.TestingCreateBucket(ctx, satellite, TestBucket)
			require.NoError(t, err)

			// check that the placement is not set yet
			bucket, err := buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)

			// set bucket placement
			bucket.Placement = euPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.NoError(t, err)

			// check that the placement is now set
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Equal(t, euPlacement, bucket.Placement)

			// upload an empty object - just to have the bucket non-empty
			err = uplink.Upload(ctx, satellite, TestBucket, TestObject, []byte{})
			require.NoError(t, err)

			// change bucket placement to new location - it should fail
			bucket.Placement = usPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.Error(t, err)

			// check that the placement has not changed
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Equal(t, euPlacement, bucket.Placement)

			// remove bucket placement constraints - it should fail
			bucket.Placement = everyCountryPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.Error(t, err)

			// check that the placement has not changed
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Equal(t, euPlacement, bucket.Placement)

			// delete the file
			err = uplink.DeleteObject(ctx, satellite, TestBucket, TestObject)
			require.NoError(t, err)

			// remove bucket placement constraints
			bucket.Placement = everyCountryPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.NoError(t, err)

			// check that the placement is not set anymore
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)
		},
	)
}

func TestBucketPlacement_PendingObject(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			buckets := satellite.API.Buckets.Service
			uplink := planet.Uplinks[0]
			projectID := uplink.Projects[0].ID

			euPlacement := storj.PlacementConstraint(1)

			// create new bucket
			err := uplink.TestingCreateBucket(ctx, satellite, TestBucket)
			require.NoError(t, err)

			// check that the placement is not set yet
			bucket, err := buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)

			project, err := uplink.GetProject(ctx, satellite)
			require.NoError(t, err)

			// begin a new upload - a pending object is created, and the bucket
			// is considered non-empty
			upload, err := project.BeginUpload(ctx, TestBucket, TestObject, nil)
			require.NoError(t, err)

			// set bucket placement - it should fail
			bucket.Placement = euPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.Error(t, err)

			// check that the placement is still not set
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Empty(t, bucket.Placement)

			// cancel the upload - the pending object is deleted, and the
			// bucket is empty again
			err = project.AbortUpload(ctx, TestBucket, TestObject, upload.UploadID)
			require.NoError(t, err)

			// set bucket placement
			bucket.Placement = euPlacement
			_, err = buckets.UpdateBucket(ctx, bucket)
			require.NoError(t, err)

			// check that the placement is now set
			bucket, err = buckets.GetBucket(ctx, []byte(TestBucket), projectID)
			require.NoError(t, err)
			assert.Equal(t, euPlacement, bucket.Placement)
		},
	)
}
