// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
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

func TestUserAgentAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		config := uplink.Config{
			UserAgent: "Zenko",
		}

		satellite, uplink := planet.Satellites[0], planet.Uplinks[0]

		access, err := config.RequestAccessWithPassphrase(ctx, satellite.URL(), uplink.Projects[0].APIKey, "mypassphrase")
		require.NoError(t, err)

		project, err := config.OpenProject(ctx, access)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		_, err = project.EnsureBucket(ctx, "bucket")
		require.NoError(t, err)

		upload, err := project.UploadObject(ctx, "bucket", "alpha", nil)
		require.NoError(t, err)

		_, err = upload.Write(testrand.Bytes(5 * memory.KiB))
		require.NoError(t, err)
		require.NoError(t, upload.Commit())

		partnerID, err := uuid.FromString("8cd605fa-ad00-45b6-823e-550eddc611d6")
		require.NoError(t, err)

		bucketInfo, err := satellite.DB.Buckets().GetBucket(ctx, []byte("bucket"), uplink.Projects[0].ID)
		require.NoError(t, err)
		assert.Equal(t, partnerID, bucketInfo.PartnerID)

		attribution, err := satellite.DB.Attribution().Get(ctx, uplink.Projects[0].ID, []byte("bucket"))
		require.NoError(t, err)
		assert.Equal(t, partnerID, attribution.PartnerID)
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

		up := planet.Uplinks[0]
		up.Config.UserAgent = "Zenko/1.0"

		err := up.CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		{ // upload test-data
			err = up.Upload(ctx, planet.Satellites[0], bucketName, filePath, testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)
		}

		{ // download test-data
			_, err = up.Download(ctx, planet.Satellites[0], bucketName, filePath)
			require.NoError(t, err)
		}

		{ // Flush all the pending information through the system.
			// Calculate the usage used for upload
			for _, sn := range planet.StorageNodes {
				sn.Storage2.Orders.Sender.TriggerWait()
			}

			rollout := planet.Satellites[0].Core.Accounting.ReportedRollupChore
			require.NoError(t, rollout.RunOnce(ctx, time.Now()))

			// Trigger tally so it gets all set up and can return a storage usage
			planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()
		}

		{
			before := time.Now().Add(-time.Hour)
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
		}

	})
}
