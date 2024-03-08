// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/nodeselection"
)

func TestAllBucketNames(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		newUser := console.CreateUser{
			FullName:  "Jack-bucket",
			ShortName: "",
			Email:     "bucketest@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "buckettest")
		require.NoError(t, err)

		bucket1 := buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket1",
			ProjectID: project.ID,
		}

		bucket2 := buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket2",
			ProjectID: project.ID,
		}

		_, err = sat.API.Buckets.Service.CreateBucket(ctx, bucket1)
		require.NoError(t, err)

		_, err = sat.API.Buckets.Service.CreateBucket(ctx, bucket2)
		require.NoError(t, err)

		testRequest := func(endpointSuffix string) {
			body, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodGet, "buckets/bucket-names"+endpointSuffix, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			var output []string

			err = json.Unmarshal(body, &output)
			require.NoError(t, err)

			require.Equal(t, bucket1.Name, output[0])
			require.Equal(t, bucket2.Name, output[1])
		}

		// test using Project.ID
		testRequest("?projectID=" + project.ID.String())

		// test using Project.PublicID
		testRequest("?publicID=" + project.PublicID.String())
	})
}

func TestBucketMetadata(t *testing.T) {
	placements := make(map[int]string)
	for i := 0; i < 2; i++ {
		placements[i] = fmt.Sprintf("loc-%d", i)
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
				var plcStr string
				for k, v := range placements {
					plcStr += fmt.Sprintf(`%d:annotation("location", "%s"); `, k, v)
				}
				config.Placement = nodeselection.ConfigurablePlacementRule{PlacementRules: plcStr}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		newUser := console.CreateUser{
			FullName:  "Jack-bucket",
			ShortName: "",
			Email:     "bucketest@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "buckettest")
		require.NoError(t, err)

		bucket1 := buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket1",
			ProjectID: project.ID,
			Placement: 0,
		}

		bucket2 := buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket2",
			ProjectID: project.ID,
			Placement: 1,
		}

		_, err = sat.API.Buckets.Service.CreateBucket(ctx, bucket1)
		require.NoError(t, err)

		_, err = sat.API.Buckets.Service.CreateBucket(ctx, bucket2)
		require.NoError(t, err)

		testRequest := func(path string, requireVersioning bool) {
			body, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodGet, path, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			var output []console.BucketMetadata

			err = json.Unmarshal(body, &output)
			require.NoError(t, err)

			require.Len(t, output, 2)

			require.Equal(t, bucket1.Name, output[0].Name)
			require.Equal(t, bucket1.Placement, output[0].Placement.DefaultPlacement)
			require.NotEqual(t, "", output[0].Placement.Location)
			require.Equal(t, placements[0], output[0].Placement.Location)
			if requireVersioning {
				require.Equal(t, bucket1.Versioning, output[0].Versioning)
			} else {
				require.Equal(t, buckets.VersioningUnsupported, output[0].Versioning)
			}

			require.Equal(t, bucket2.Name, output[1].Name)
			require.Equal(t, bucket2.Placement, output[1].Placement.DefaultPlacement)
			require.NotEqual(t, "", output[1].Placement.Location)
			require.Equal(t, placements[1], output[1].Placement.Location)
			if requireVersioning {
				require.Equal(t, bucket2.Versioning, output[1].Versioning)
			} else {
				require.Equal(t, buckets.VersioningUnsupported, output[1].Versioning)
			}
		}

		base := "buckets/bucket-placements"
		// test using Project.ID
		testRequest(base+"?projectID="+project.ID.String(), false)

		// test using Project.PublicID
		testRequest(base+"?publicID="+project.PublicID.String(), false)

		base = "buckets/bucket-metadata"

		// test using Project.ID
		testRequest(base+"?projectID="+project.ID.String(), true)

		// test using Project.PublicID
		testRequest(base+"?publicID="+project.PublicID.String(), true)
	})
}
