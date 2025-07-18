// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metainfo"
	"storj.io/uplink"
)

func TestTrimUserAgent(t *testing.T) {
	oversizeProduct := testrand.RandAlphaNumeric(metainfo.MaxUserAgentLength)
	oversizeVersion := testrand.RandNumeric(metainfo.MaxUserAgentLength)
	for _, tt := range []struct {
		userAgent         []byte
		strippedUserAgent []byte
	}{
		{userAgent: nil, strippedUserAgent: nil},
		{userAgent: []byte(""), strippedUserAgent: []byte("")},
		{userAgent: []byte("not-a-partner"), strippedUserAgent: []byte("not-a-partner")},
		{userAgent: []byte("Zenko"), strippedUserAgent: []byte("Zenko")},
		{userAgent: []byte("Zenko uplink/v1.0.0"), strippedUserAgent: []byte("Zenko")},
		{userAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"), strippedUserAgent: []byte("Zenko")},
		{userAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0) (common/v0.0.0-00010101000000-000000000000)"), strippedUserAgent: []byte("Zenko")},
		{userAgent: []byte("uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"), strippedUserAgent: []byte("")},
		{userAgent: []byte("uplink/v1.0.0"), strippedUserAgent: []byte("")},
		{userAgent: []byte("uplink/v1.0.0 Zenko/v3"), strippedUserAgent: []byte("Zenko/v3")},
		// oversize alphanumeric as 2nd entry product should use just the first entry
		{userAgent: append([]byte("Zenko/v3 "), oversizeProduct...), strippedUserAgent: []byte("Zenko/v3")},
		// all comments (small or oversize) should be completely removed
		{userAgent: append([]byte("Zenko ("), append(oversizeVersion, []byte(")")...)...), strippedUserAgent: []byte("Zenko")},
		// oversize version should truncate
		{userAgent: append([]byte("Zenko/v"), oversizeVersion...), strippedUserAgent: []byte("Zenko/v" + string(oversizeVersion[:len(oversizeVersion)-len("Zenko/v")]))},
		// oversize product names should truncate
		{userAgent: append([]byte("Zenko"), oversizeProduct...), strippedUserAgent: []byte("Zenko" + string(oversizeProduct[:len(oversizeProduct)-len("Zenko")]))},
	} {
		userAgent, err := metainfo.TrimUserAgent(tt.userAgent)
		require.NoError(t, err)
		assert.Equal(t, tt.strippedUserAgent, userAgent)
	}
}

func TestBucketAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i, tt := range []struct {
			signupPartner       []byte
			userAgent           []byte
			expectedAttribution []byte
		}{
			{signupPartner: nil, userAgent: nil, expectedAttribution: nil},
			{signupPartner: []byte(""), userAgent: []byte(""), expectedAttribution: []byte("")},
			{signupPartner: []byte("Minio"), userAgent: nil, expectedAttribution: []byte("Minio")},
			{signupPartner: []byte("Minio"), userAgent: []byte("Minio"), expectedAttribution: []byte("Minio")},
			{signupPartner: []byte("Minio"), userAgent: []byte("Zenko"), expectedAttribution: []byte("Minio")},
			{signupPartner: nil, userAgent: []byte("rclone/1.0 uplink/v1.6.1-0.20211005203254-bb2eda8c28d3"), expectedAttribution: []byte("rclone/1.0")},
			{signupPartner: nil, userAgent: []byte("Zenko"), expectedAttribution: []byte("Zenko")},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			satellite := planet.Satellites[0]

			user1, err := satellite.AddUser(ctx, console.CreateUser{
				FullName:  "Test User " + strconv.Itoa(i),
				Email:     "user@test" + strconv.Itoa(i),
				UserAgent: tt.signupPartner,
			}, 1)
			require.NoError(t, err, errTag)

			satProject, err := satellite.AddProject(ctx, user1.ID, "test"+strconv.Itoa(i))
			require.NoError(t, err, errTag)

			// add a second user to the project, and create the api key with the new user to ensure that
			// the project owner's attribution is used for a new bucket, even if someone else creates it.
			user2, err := satellite.AddUser(ctx, console.CreateUser{
				FullName:  "Test User 2" + strconv.Itoa(i),
				Email:     "user2@test" + strconv.Itoa(i),
				UserAgent: tt.signupPartner,
			}, 1)
			require.NoError(t, err, errTag)
			_, err = satellite.DB.Console().ProjectMembers().Insert(ctx, user2.ID, satProject.ID, console.RoleAdmin)
			require.NoError(t, err)

			createBucketAndCheckAttribution := func(userID uuid.UUID, apiKeyName, bucketName string) {
				userCtx, err := satellite.UserContext(ctx, userID)
				require.NoError(t, err, errTag)

				_, apiKeyInfo, err := satellite.API.Console.Service.CreateAPIKey(userCtx, satProject.ID, apiKeyName, macaroon.APIKeyVersionMin)
				require.NoError(t, err, errTag)

				config := uplink.Config{
					UserAgent: string(tt.userAgent),
				}
				access, err := config.RequestAccessWithPassphrase(ctx, satellite.NodeURL().String(), apiKeyInfo.Serialize(), "mypassphrase")
				require.NoError(t, err, errTag)

				project, err := config.OpenProject(ctx, access)
				require.NoError(t, err, errTag)

				_, err = project.CreateBucket(ctx, bucketName)
				require.NoError(t, err, errTag)

				bucketInfo, err := satellite.API.Buckets.Service.GetBucket(ctx, []byte(bucketName), satProject.ID)
				require.NoError(t, err, errTag)
				assert.Equal(t, tt.expectedAttribution, bucketInfo.UserAgent, errTag)

				attributionInfo, err := planet.Satellites[0].DB.Attribution().Get(ctx, satProject.ID, []byte(bucketName))
				require.NoError(t, err, errTag)
				assert.Equal(t, tt.expectedAttribution, attributionInfo.UserAgent, errTag)
			}

			createBucketAndCheckAttribution(user1.ID, "apikey1", "bucket1")
			createBucketAndCheckAttribution(user2.ID, "apikey2", "bucket2")
		}
	})
}

func TestBucketPlacementAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		satProject, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		config := planet.Uplinks[0].Config
		access, err := config.RequestAccessWithPassphrase(ctx, sat.NodeURL().String(), planet.Uplinks[0].APIKey[sat.ID()].Serialize(), "mypassphrase")
		require.NoError(t, err)

		project, err := config.OpenProject(ctx, access)
		require.NoError(t, err)

		// Test happy path.
		bucketName := "testbucket"
		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err)

		bucketInfo, err := sat.API.Buckets.Service.GetBucket(ctx, []byte(bucketName), satProject.ID)
		require.NoError(t, err)
		assert.Equal(t, storj.DefaultPlacement, bucketInfo.Placement)

		attributionInfo, err := planet.Satellites[0].DB.Attribution().Get(ctx, satProject.ID, []byte(bucketName))
		require.NoError(t, err)
		require.NotNil(t, attributionInfo.Placement)
		assert.Equal(t, storj.DefaultPlacement, *attributionInfo.Placement)

		require.NoError(t, planet.Satellites[0].DB.Buckets().DeleteBucket(ctx, []byte(bucketName), satProject.ID))

		// Change the project default placement and confirm that recreating bucket fails due to preexisting attribution with different placement.
		require.NoError(t, sat.API.DB.Console().Projects().UpdateDefaultPlacement(ctx, satProject.ID, storj.PlacementConstraint(1)))

		_, err = project.CreateBucket(ctx, bucketName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already attributed to a different placement constraint")

		// test case where bucket attribution has nil placement
		bucketName += "2"
		require.NoError(t, sat.API.DB.Console().Projects().UpdateDefaultPlacement(ctx, satProject.ID, storj.DefaultPlacement))
		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err)

		err = planet.Satellites[0].DB.Attribution().UpdatePlacement(ctx, satProject.ID, bucketName, nil)
		require.NoError(t, err)

		require.NoError(t, planet.Satellites[0].DB.Buckets().DeleteBucket(ctx, []byte(bucketName), satProject.ID))

		require.NoError(t, sat.API.DB.Console().Projects().UpdateDefaultPlacement(ctx, satProject.ID, storj.PlacementConstraint(1)))
		_, err = project.CreateBucket(ctx, bucketName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already attributed to a different placement constraint")

		require.NoError(t, sat.API.DB.Console().Projects().UpdateDefaultPlacement(ctx, satProject.ID, storj.DefaultPlacement))
		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err)
	})
}

func TestQueryAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
		// TODO(spanner): There's an emulator bug with regards to MAX(timestamp),
		// which causes some queries to fail.
		// https://github.com/GoogleCloudPlatform/cloud-spanner-emulator/issues/73
		SkipSpanner: true,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "test"
			objectKey  = "test-key"
		)
		satellite := planet.Satellites[0]
		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)

		userAgent := "Minio"

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName:  "user@test",
			Email:     "user@test",
			UserAgent: []byte(userAgent),
		}, 1)
		require.NoError(t, err)

		satProject, err := satellite.AddProject(ctx, user.ID, "test")
		require.NoError(t, err)

		userCtx, err := satellite.UserContext(ctx, user.ID)
		require.NoError(t, err)

		_, apiKeyInfo, err := satellite.API.Console.Service.CreateAPIKey(userCtx, satProject.ID, "root", macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		access, err := uplink.RequestAccessWithPassphrase(ctx, satellite.NodeURL().String(), apiKeyInfo.Serialize(), "mypassphrase")
		require.NoError(t, err)

		project, err := uplink.OpenProject(ctx, access)
		require.NoError(t, err)

		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err)

		{ // upload and download should be accounted for Minio
			upload, err := project.UploadObject(ctx, bucketName, objectKey, nil)
			require.NoError(t, err)

			_, err = upload.Write(testrand.Bytes(5 * memory.KiB))
			require.NoError(t, err)

			err = upload.Commit()
			require.NoError(t, err)

			download, err := project.DownloadObject(ctx, bucketName, objectKey, nil)
			require.NoError(t, err)

			_, err = io.ReadAll(download)
			require.NoError(t, err)

			err = download.Close()
			require.NoError(t, err)
		}

		// Wait for the storage nodes to be done processing the download
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		{ // Flush all the pending information through the system.
			// Calculate the usage used for upload
			for _, sn := range planet.StorageNodes {
				sn.Storage2.Orders.SendOrders(ctx, tomorrow)
			}

			// The orders chore writes bucket bandwidth rollup changes to satellitedb
			planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

			// Trigger tally so it gets all set up and can return a storage usage
			planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()
		}

		{
			before := now.Add(-time.Hour)
			after := before.Add(2 * time.Hour)

			usage, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectTotal(ctx, satProject.ID, before, after)
			require.NoError(t, err)
			require.NotZero(t, usage.Egress)

			userAgent := []byte("Minio")
			require.NoError(t, err)

			rows, err := planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, userAgent, before, after)
			require.NoError(t, err)
			require.NotZero(t, rows[0].ByteHours)
			require.Equal(t, rows[0].EgressData, usage.Egress)

			// also test QueryAllAttribution
			rows, err = planet.Satellites[0].DB.Attribution().QueryAllAttribution(ctx, before, after)
			require.NoError(t, err)
			require.Equal(t, rows[0].UserAgent, userAgent)
			require.NotZero(t, rows[0].ByteHours)
			require.Equal(t, rows[0].EgressData, usage.Egress)
		}
	})
}

func TestAttributionReport(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		// TODO(spanner): There's an emulator bug with regards to MAX(timestamp),
		// which causes some queries to fail.
		// https://github.com/GoogleCloudPlatform/cloud-spanner-emulator/issues/73
		SkipSpanner: true,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "test"
			filePath   = "path"
		)
		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)

		up := planet.Uplinks[0]
		zenkoStr := "Zenko/1.0"
		up.Config.UserAgent = zenkoStr

		err := up.TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		{ // upload and download as Zenko
			err = up.Upload(ctx, planet.Satellites[0], bucketName, filePath, testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)

			_, err = up.Download(ctx, planet.Satellites[0], bucketName, filePath)
			require.NoError(t, err)
		}
		minioStr := "Minio/1.0"
		up.Config.UserAgent = minioStr
		{ // upload and download as Minio
			err = up.Upload(ctx, planet.Satellites[0], bucketName, filePath, testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)

			_, err = up.Download(ctx, planet.Satellites[0], bucketName, filePath)
			require.NoError(t, err)
		}

		// Wait for the storage nodes to be done processing the download
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		{ // Flush all the pending information through the system.
			// Calculate the usage used for upload
			for _, sn := range planet.StorageNodes {
				sn.Storage2.Orders.SendOrders(ctx, tomorrow)
			}

			// The orders chore writes bucket bandwidth rollup changes to satellitedb
			planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

			// Trigger tally so it gets all set up and can return a storage usage
			planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()
		}

		{
			before := now.Add(-time.Hour)
			after := before.Add(2 * time.Hour)

			projectID := up.Projects[0].ID

			usage, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectTotal(ctx, projectID, before, after)
			require.NoError(t, err)
			require.NotZero(t, usage.Egress)

			rows, err := planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, []byte(zenkoStr), before, after)
			require.NoError(t, err)
			require.NotZero(t, rows[0].ByteHours)
			require.Equal(t, rows[0].EgressData, usage.Egress)

			// Minio should have no attribution because bucket was created by Zenko
			rows, err = planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, []byte(minioStr), before, after)
			require.NoError(t, err)
			require.Empty(t, rows)

			// also test QueryAllAttribution
			rows, err = planet.Satellites[0].DB.Attribution().QueryAllAttribution(ctx, before, after)
			require.NoError(t, err)

			var zenkoFound, minioFound bool
			for _, r := range rows {
				if bytes.Equal(r.UserAgent, []byte(zenkoStr)) {
					require.NotZero(t, rows[0].ByteHours)
					require.Equal(t, rows[0].EgressData, usage.Egress)
					zenkoFound = true
				} else if bytes.Equal(r.UserAgent, []byte(minioStr)) {
					minioFound = true
				}
			}

			require.True(t, zenkoFound)

			// Minio should have no attribution because bucket was created by Zenko
			require.False(t, minioFound)
		}
	})
}

func TestBucketAttributionConcurrentUpload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		err := planet.Uplinks[0].TestingCreateBucket(ctx, satellite, "attr-bucket")
		require.NoError(t, err)

		config := uplink.Config{
			UserAgent: "Minio",
		}
		project, err := config.OpenProject(ctx, planet.Uplinks[0].Access[satellite.ID()])
		require.NoError(t, err)

		var errgroup errgroup.Group
		for i := 0; i < 3; i++ {
			i := i
			errgroup.Go(func() error {
				upload, err := project.UploadObject(ctx, "attr-bucket", "key"+strconv.Itoa(i), nil)
				require.NoError(t, err)

				_, err = upload.Write([]byte("content"))
				require.NoError(t, err)

				err = upload.Commit()
				require.NoError(t, err)
				return nil
			})
		}

		require.NoError(t, errgroup.Wait())

		attributionInfo, err := planet.Satellites[0].DB.Attribution().Get(ctx, planet.Uplinks[0].Projects[0].ID, []byte("attr-bucket"))
		require.NoError(t, err)
		require.Equal(t, []byte(config.UserAgent), attributionInfo.UserAgent)
	})
}

func TestAttributionDeletedBucketRecreated(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		proj := upl.Projects[0].ID
		bucket := "testbucket"
		ua1 := []byte("minio")
		ua2 := []byte("not minio")

		require.NoError(t, satellite.DB.Console().Projects().UpdateUserAgent(ctx, proj, ua1))

		require.NoError(t, upl.CreateBucket(ctx, satellite, bucket))
		b, err := satellite.DB.Buckets().GetBucket(ctx, []byte(bucket), proj)
		require.NoError(t, err)
		require.Equal(t, ua1, b.UserAgent)

		// test recreate with same user agent
		require.NoError(t, upl.DeleteBucket(ctx, satellite, bucket))
		require.NoError(t, upl.CreateBucket(ctx, satellite, bucket))
		b, err = satellite.DB.Buckets().GetBucket(ctx, []byte(bucket), proj)
		require.NoError(t, err)
		require.Equal(t, ua1, b.UserAgent)

		// test recreate with different user agent
		// should still have original user agent
		require.NoError(t, upl.DeleteBucket(ctx, satellite, bucket))
		upl.Config.UserAgent = string(ua2)
		require.NoError(t, upl.CreateBucket(ctx, satellite, bucket))
		require.NoError(t, err)
		require.Equal(t, ua1, b.UserAgent)
	})
}

func TestAttributionBeginObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		proj := upl.Projects[0].ID
		p, err := satellite.API.DB.Console().Projects().Get(ctx, proj)
		require.NoError(t, err)
		userID := p.OwnerID
		ua := []byte("minio")

		tests := []struct {
			name                                      string
			vaAttrBefore, bktAttrBefore, bktAttrAfter bool
		}{
			// test for existence of user_agent in buckets table given the different possibilities of preconditions of user_agent
			// in value_attributions and bucket_metainfos to make sure nothing breaks and outcome is expected.
			{
				name:          "attribution exists in VA and bucket",
				vaAttrBefore:  true,
				bktAttrBefore: true,
				bktAttrAfter:  true,
			},
			{
				name:          "attribution exists in VA and NOT bucket",
				vaAttrBefore:  true,
				bktAttrBefore: false,
				bktAttrAfter:  false,
			},
			{
				name:          "attribution exists in bucket and NOT VA",
				vaAttrBefore:  false,
				bktAttrBefore: true,
				bktAttrAfter:  true,
			},
			{
				name:          "attribution exists in neither VA nor buckets",
				vaAttrBefore:  false,
				bktAttrBefore: false,
				bktAttrAfter:  true,
			},
		}

		for i, tt := range tests {
			t.Run(tt.name, func(*testing.T) {
				bucketName := fmt.Sprintf("bucket-%d", i)
				var expectedBktUA []byte
				var config uplink.Config
				if tt.bktAttrBefore || tt.vaAttrBefore {
					config.UserAgent = string(ua)
				}
				if tt.bktAttrAfter {
					expectedBktUA = ua
				}

				uplProj, err := config.OpenProject(ctx, upl.Access[satellite.ID()])
				require.NoError(t, err)

				// VA will now always be inserted on first bucket creation in CreateBucket endpoint in order to record the placement.
				// To test buckets created before this change, which may not have a VA row, bypass CreateBucket endpoint and create
				// the bucket directly in the DB.
				if !tt.vaAttrBefore {
					_, err = satellite.API.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
						ID:        testrand.UUID(),
						Name:      bucketName,
						ProjectID: proj,
						CreatedBy: userID,
						UserAgent: ua,
						Created:   time.Now(),
					})
					require.NoError(t, err)
				} else {
					_, err = uplProj.CreateBucket(ctx, bucketName)
					require.NoError(t, err)
				}

				require.NoError(t, uplProj.Close())

				if !tt.bktAttrBefore {
					// remove user agent from bucket
					err = satellite.API.DB.Buckets().UpdateUserAgent(ctx, proj, bucketName, nil)
					require.NoError(t, err)
				}

				_, err = satellite.API.DB.Attribution().Get(ctx, proj, []byte(bucketName))
				if !tt.vaAttrBefore {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				b, err := satellite.API.DB.Buckets().GetBucket(ctx, []byte(bucketName), proj)
				require.NoError(t, err)
				if !tt.bktAttrBefore {
					require.Nil(t, b.UserAgent)
				} else {
					require.Equal(t, expectedBktUA, b.UserAgent)
				}

				config.UserAgent = string(ua)

				uplProj, err = config.OpenProject(ctx, upl.Access[satellite.ID()])
				require.NoError(t, err)

				upload, err := uplProj.UploadObject(ctx, bucketName, fmt.Sprintf("foobar-%d", i), nil)
				require.NoError(t, err)

				_, err = upload.Write([]byte("content"))
				require.NoError(t, err)

				err = upload.Commit()
				require.NoError(t, err)

				attr, err := satellite.API.DB.Attribution().Get(ctx, proj, []byte(bucketName))
				require.NoError(t, err)
				require.Equal(t, ua, attr.UserAgent)

				b, err = satellite.API.DB.Buckets().GetBucket(ctx, []byte(bucketName), proj)
				require.NoError(t, err)
				require.Equal(t, expectedBktUA, b.UserAgent)
			})
		}
	})
}
