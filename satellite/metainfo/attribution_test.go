// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
)

func TestResolvePartnerID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		endpoint := planet.Satellites[0].Metainfo.Endpoint2

		zenkoPartnerID, err := uuid.FromString("8cd605fa-ad00-45b6-823e-550eddc611d6")
		require.NoError(t, err)

		// no header
		_, err = endpoint.ResolvePartnerID(ctx, nil)
		require.Error(t, err)

		partnerID, err := endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("not-a-partner"),
		})
		require.NoError(t, err)
		require.Equal(t, uuid.UUID{}, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko"),
		})
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0"),
		})
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"),
		})
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0) (common/v0.0.0-00010101000000-000000000000)"),
		})
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"),
		})
		require.NoError(t, err)
		require.Equal(t, uuid.UUID{}, partnerID)
	})
}

func TestBucketAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i, tt := range []struct {
			signupPartner       string
			userAgent           string
			expectedAttribution string
		}{
			{signupPartner: "", userAgent: "", expectedAttribution: ""},
			{signupPartner: "Minio", userAgent: "", expectedAttribution: "Minio"},
			{signupPartner: "Minio", userAgent: "Minio", expectedAttribution: "Minio"},
			{signupPartner: "Minio", userAgent: "Zenko", expectedAttribution: "Minio"},
			{signupPartner: "", userAgent: "Zenko", expectedAttribution: "Zenko"},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			satellite := planet.Satellites[0]

			var signupPartnerID string
			if tt.signupPartner != "" {
				partner, err := satellite.API.Marketing.PartnersService.ByName(ctx, tt.signupPartner)
				require.NoError(t, err, errTag)
				signupPartnerID = partner.ID
			}

			user, err := satellite.AddUser(ctx, console.CreateUser{
				FullName:  "Test User " + strconv.Itoa(i),
				Email:     "user@test" + strconv.Itoa(i),
				PartnerID: signupPartnerID,
			}, 1)
			require.NoError(t, err, errTag)

			satProject, err := satellite.AddProject(ctx, user.ID, "test"+strconv.Itoa(i))
			require.NoError(t, err, errTag)

			authCtx, err := satellite.AuthenticatedContext(ctx, user.ID)
			require.NoError(t, err, errTag)

			_, apiKeyInfo, err := satellite.API.Console.Service.CreateAPIKey(authCtx, satProject.ID, "root")
			require.NoError(t, err, errTag)

			config := uplink.Config{
				UserAgent: tt.userAgent,
			}
			access, err := config.RequestAccessWithPassphrase(ctx, satellite.NodeURL().String(), apiKeyInfo.Serialize(), "mypassphrase")
			require.NoError(t, err, errTag)

			project, err := config.OpenProject(ctx, access)
			require.NoError(t, err, errTag)

			_, err = project.CreateBucket(ctx, "bucket")
			require.NoError(t, err, errTag)

			var expectedPartnerID uuid.UUID
			if tt.expectedAttribution != "" {
				expectedPartner, err := planet.Satellites[0].API.Marketing.PartnersService.ByName(ctx, tt.expectedAttribution)
				require.NoError(t, err, errTag)
				expectedPartnerID = expectedPartner.UUID
			}

			bucketInfo, err := satellite.DB.Buckets().GetBucket(ctx, []byte("bucket"), satProject.ID)
			require.NoError(t, err, errTag)
			assert.Equal(t, expectedPartnerID, bucketInfo.PartnerID, errTag)

			attributionInfo, err := planet.Satellites[0].DB.Attribution().Get(ctx, satProject.ID, []byte("bucket"))
			if tt.expectedAttribution == "" {
				assert.True(t, attribution.ErrBucketNotAttributed.Has(err), errTag)
			} else {
				require.NoError(t, err, errTag)
				assert.Equal(t, expectedPartnerID, attributionInfo.PartnerID, errTag)
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

		partner, err := satellite.API.Marketing.PartnersService.ByName(ctx, "Minio")
		require.NoError(t, err)

		user, err := satellite.AddUser(ctx, console.CreateUser{
			FullName:  "user@test",
			Email:     "user@test",
			PartnerID: partner.ID,
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

			partner, err := planet.Satellites[0].API.Marketing.PartnersService.ByName(ctx, "Minio")
			require.NoError(t, err)

			rows, err := planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, partner.UUID, before, after)
			require.NoError(t, err)
			require.NotZero(t, rows[0].RemoteBytesPerHour)
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
		up.Config.UserAgent = "Zenko/1.0"

		err := up.CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		{ // upload and download as Zenko
			err = up.Upload(ctx, planet.Satellites[0], bucketName, filePath, testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)

			_, err = up.Download(ctx, planet.Satellites[0], bucketName, filePath)
			require.NoError(t, err)
		}

		up.Config.UserAgent = "Minio/1.0"
		{ // upload and download as Minio
			err = up.Upload(ctx, planet.Satellites[0], bucketName, filePath, testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)

			_, err = up.Download(ctx, planet.Satellites[0], bucketName, filePath)
			require.NoError(t, err)
		}

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

			partner, err := planet.Satellites[0].API.Marketing.PartnersService.ByUserAgent(ctx, "Zenko")
			require.NoError(t, err)

			rows, err := planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, partner.UUID, before, after)
			require.NoError(t, err)
			require.NotZero(t, rows[0].RemoteBytesPerHour)
			require.Equal(t, rows[0].EgressData, usage.Egress)

			// Minio should have no attribution because bucket was created by Zenko
			partner, err = planet.Satellites[0].API.Marketing.PartnersService.ByUserAgent(ctx, "Minio")
			require.NoError(t, err)

			rows, err = planet.Satellites[0].DB.Attribution().QueryAttribution(ctx, partner.UUID, before, after)
			require.NoError(t, err)
			require.Empty(t, rows)
		}
	})
}
