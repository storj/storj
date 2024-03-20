// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
)

func TestAdminBucketGeofenceAPI(t *testing.T) {
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

		err = uplink.CreateBucket(ctx, sat, "filled")
		require.NoError(t, err)

		_, err = sat.DB.Buckets().UpdateBucket(ctx, buckets.Bucket{
			Name:      "filled",
			ProjectID: project.ID,
			Placement: storj.EEA,
		})
		require.NoError(t, err)

		err = uplink.Upload(ctx, sat, "filled", "README.md", []byte("hello world"))
		require.NoError(t, err)

		err = uplink.CreateBucket(ctx, sat, "empty")
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
				status:  http.StatusBadRequest,
				body:    `{"error":"bucket does not exist","detail":""}`,
			},
			{
				name:    "bucket is not empty",
				project: project.ID,
				bucket:  []byte("filled"),
				status:  http.StatusBadRequest,
				body:    `{"error":"bucket must be empty","detail":""}`,
			},
			{
				name:    "validated",
				project: project.ID,
				bucket:  []byte("empty"),
				status:  http.StatusOK,
				body:    "",
			},
		}

		for _, testCase := range testCases {
			baseURL := fmt.Sprintf("http://%s/api/projects/%s/buckets/%s", address, testCase.project, string(testCase.bucket))
			t.Log(baseURL)
			baseGeofenceURL := fmt.Sprintf("http://%s/api/projects/%s/buckets/%s/geofence", address, testCase.project, string(testCase.bucket))
			t.Log(baseGeofenceURL)

			t.Run(testCase.name, func(t *testing.T) {
				assertReq(ctx, t, baseGeofenceURL+"?region=EU", "POST", "", testCase.status, testCase.body, sat.Config.Console.AuthToken)

				if testCase.status == http.StatusOK {
					b, err := sat.DB.Buckets().GetBucket(ctx, testCase.bucket, testCase.project)
					require.NoError(t, err)

					expected, err := json.Marshal(buckets.Bucket{
						ID:        b.ID,
						Name:      b.Name,
						ProjectID: testCase.project,
						Created:   b.Created,
						CreatedBy: b.CreatedBy,
						Placement: storj.EU,
					})
					require.NoError(t, err, "failed to json encode expected bucket")

					assertGet(ctx, t, baseURL, string(expected), sat.Config.Console.AuthToken)
				}

				assertReq(ctx, t, baseGeofenceURL, "DELETE", "", testCase.status, testCase.body, sat.Config.Console.AuthToken)
			})
		}
	})
}
