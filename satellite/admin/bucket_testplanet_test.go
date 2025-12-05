// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
)

func TestAdminBucketPlacementAPI(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplink := planet.Uplinks[0]
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		bucketsDB := sat.DB.Buckets()
		attributionDB := sat.DB.Attribution()

		project, err := sat.DB.Console().Projects().Get(ctx, uplink.Projects[0].ID)
		require.NoError(t, err)

		filledBucket := "filled"
		err = uplink.CreateBucket(ctx, sat, filledBucket)
		require.NoError(t, err)

		_, err = bucketsDB.UpdateBucket(ctx, buckets.Bucket{
			Name:      filledBucket,
			ProjectID: project.ID,
			Placement: storj.DefaultPlacement,
		})
		require.NoError(t, err)

		err = uplink.Upload(ctx, sat, filledBucket, "README.md", []byte("hello world"))
		require.NoError(t, err)

		_, err = attributionDB.Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(filledBucket),
		})
		require.NoError(t, err)

		emptyBucket := "empty"
		err = uplink.CreateBucket(ctx, sat, emptyBucket)
		require.NoError(t, err)

		_, err = attributionDB.Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(emptyBucket),
		})
		require.NoError(t, err)

		testCases := []struct {
			name    string
			project uuid.UUID
			bucket  []byte
			// expectations
			status int
			body   string
		}{
			{
				name:    "bucket does not exist",
				project: project.ID,
				bucket:  []byte("non-existent"),
				status:  http.StatusNotFound,
				body:    `{"error":"bucket does not exist","detail":""}`,
			},
			{
				name:    "bucket is not empty",
				project: project.ID,
				bucket:  []byte(filledBucket),
				status:  http.StatusBadRequest,
				body:    `{"error":"bucket must be empty","detail":""}`,
			},
			{
				name:    "validated",
				project: project.ID,
				bucket:  []byte(emptyBucket),
				status:  http.StatusOK,
				body:    "",
			},
		}

		for _, testCase := range testCases {
			baseURL := fmt.Sprintf("http://%s/api/projects/%s/buckets/%s", address, testCase.project, string(testCase.bucket))
			t.Log(baseURL)
			basePlacementURL := fmt.Sprintf("http://%s/api/projects/%s/buckets/%s/placement", address, testCase.project, string(testCase.bucket))
			t.Log(basePlacementURL)

			t.Run(testCase.name, func(t *testing.T) {
				newPlacement := storj.PlacementConstraint(1)
				assertReq(ctx, t, basePlacementURL+"?id="+strconv.Itoa(int(newPlacement)), http.MethodPut, "", testCase.status, testCase.body, sat.Config.Console.AuthToken)

				if testCase.status == http.StatusOK {
					b, err := bucketsDB.GetBucket(ctx, testCase.bucket, testCase.project)
					require.NoError(t, err)

					attr, err := attributionDB.Get(ctx, testCase.project, []byte(b.Name))
					require.NoError(t, err)
					require.NotNil(t, attr)
					require.NotNil(t, attr.Placement)
					require.Equal(t, newPlacement, *attr.Placement)

					expected, err := json.Marshal(buckets.Bucket{
						ID:         b.ID,
						Name:       b.Name,
						ProjectID:  testCase.project,
						Created:    b.Created,
						CreatedBy:  b.CreatedBy,
						Placement:  newPlacement,
						Versioning: buckets.Unversioned,
					})
					require.NoError(t, err, "failed to json encode expected bucket")

					assertGet(ctx, t, baseURL, string(expected), sat.Config.Console.AuthToken)
				}
			})
		}
	})
}

func TestAdminUpdateBucketValueAttributionPlacement(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplink := planet.Uplinks[0]
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()

		project, err := sat.DB.Console().Projects().Get(ctx, uplink.Projects[0].ID)
		require.NoError(t, err)

		bucketName := "test-bucket"
		baseURL := fmt.Sprintf("http://%s/api/projects/%s/buckets/%s/value-attributions", address, project.ID, bucketName)

		assertReq(ctx, t, baseURL+"?placement=1", http.MethodPut, "", http.StatusNotFound, "", sat.Config.Console.AuthToken)

		info, err := sat.DB.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  project.ID,
			BucketName: []byte(bucketName),
		})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Nil(t, info.Placement)

		assertReq(ctx, t, baseURL+"?placement=-1", "PUT", "", http.StatusBadRequest, "", sat.Config.Console.AuthToken)
		assertReq(ctx, t, baseURL+"?placement=EU", "PUT", "", http.StatusBadRequest, "", sat.Config.Console.AuthToken)
		assertReq(ctx, t, baseURL+"?placement=1", "PUT", "", http.StatusOK, "", sat.Config.Console.AuthToken)
		assertReq(ctx, t, baseURL+"?placement=null", "PUT", "", http.StatusOK, "", sat.Config.Console.AuthToken)
	})
}
