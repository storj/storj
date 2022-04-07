// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/attribution"
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

			user, err := satellite.AddUser(ctx, console.CreateUser{
				FullName:  "Test User " + strconv.Itoa(i),
				Email:     "user@test" + strconv.Itoa(i),
				PartnerID: "",
				UserAgent: tt.signupPartner,
			}, 1)
			require.NoError(t, err, errTag)

			satProject, err := satellite.AddProject(ctx, user.ID, "test"+strconv.Itoa(i))
			require.NoError(t, err, errTag)

			authCtx, err := satellite.AuthenticatedContext(ctx, user.ID)
			require.NoError(t, err, errTag)

			_, apiKeyInfo, err := satellite.API.Console.Service.CreateAPIKey(authCtx, satProject.ID, "root")
			require.NoError(t, err, errTag)

			config := uplink.Config{
				UserAgent: string(tt.userAgent),
			}
			access, err := config.RequestAccessWithPassphrase(ctx, satellite.NodeURL().String(), apiKeyInfo.Serialize(), "mypassphrase")
			require.NoError(t, err, errTag)

			project, err := config.OpenProject(ctx, access)
			require.NoError(t, err, errTag)

			_, err = project.CreateBucket(ctx, "bucket")
			require.NoError(t, err, errTag)

			bucketInfo, err := satellite.API.Buckets.Service.GetBucket(ctx, []byte("bucket"), satProject.ID)
			require.NoError(t, err, errTag)
			assert.Equal(t, tt.expectedAttribution, bucketInfo.UserAgent, errTag)

			attributionInfo, err := planet.Satellites[0].DB.Attribution().Get(ctx, satProject.ID, []byte("bucket"))
			if tt.expectedAttribution == nil {
				assert.True(t, attribution.ErrBucketNotAttributed.Has(err), errTag)
			} else {
				require.NoError(t, err, errTag)
				assert.Equal(t, tt.expectedAttribution, attributionInfo.UserAgent, errTag)
			}
		}
	})
}

func TestQueryAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
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
			PartnerID: "",
			UserAgent: []byte(userAgent),
		}, 1)
		require.NoError(t, err)

		satProject, err := satellite.AddProject(ctx, user.ID, "test")
		require.NoError(t, err)

		authCtx, err := satellite.AuthenticatedContext(ctx, user.ID)
		require.NoError(t, err)

		_, apiKeyInfo, err := satellite.API.Console.Service.CreateAPIKey(authCtx, satProject.ID, "root")
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

			_, err = ioutil.ReadAll(download)
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

			partner, _ := planet.Satellites[0].API.Marketing.PartnersService.ByName(ctx, "")

			userAgent := []byte("Minio")
			require.NoError(t, err)

			rows, err := planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, partner.UUID, userAgent, before, after)
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

		err := up.CreateBucket(ctx, planet.Satellites[0], bucketName)
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

			partner, _ := planet.Satellites[0].API.Marketing.PartnersService.ByUserAgent(ctx, "")

			rows, err := planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, partner.UUID, []byte(zenkoStr), before, after)
			require.NoError(t, err)
			require.NotZero(t, rows[0].ByteHours)
			require.Equal(t, rows[0].EgressData, usage.Egress)

			// Minio should have no attribution because bucket was created by Zenko
			partner, _ = planet.Satellites[0].API.Marketing.PartnersService.ByUserAgent(ctx, "")

			rows, err = planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, partner.UUID, []byte(minioStr), before, after)
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

		err := planet.Uplinks[0].CreateBucket(ctx, satellite, "attr-bucket")
		require.NoError(t, err)

		config := uplink.Config{
			UserAgent: "Minio",
		}
		project, err := config.OpenProject(ctx, planet.Uplinks[0].Access[satellite.ID()])
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			i := i
			ctx.Go(func() error {
				upload, err := project.UploadObject(ctx, "attr-bucket", "key"+strconv.Itoa(i), nil)
				require.NoError(t, err)

				_, err = upload.Write([]byte("content"))
				require.NoError(t, err)

				err = upload.Commit()
				require.NoError(t, err)
				return nil
			})
		}

		ctx.Wait()

		attributionInfo, err := planet.Satellites[0].DB.Attribution().Get(ctx, planet.Uplinks[0].Projects[0].ID, []byte("attr-bucket"))
		require.NoError(t, err)
		require.Equal(t, []byte(config.UserAgent), attributionInfo.UserAgent)
	})
}
