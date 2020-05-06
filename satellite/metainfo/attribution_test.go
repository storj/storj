// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/uplink"
	"storj.io/uplink/private/metainfo"
)

func TestResolvePartnerID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		endpoint := planet.Satellites[0].Metainfo.Endpoint2

		zenkoPartnerID, err := uuid.FromString("8cd605fa-ad00-45b6-823e-550eddc611d6")
		require.NoError(t, err)

		// no header
		_, err = endpoint.ResolvePartnerID(ctx, nil, []byte{1, 2, 3})
		require.Error(t, err)

		// bad uuid
		_, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{}, []byte{1, 2, 3})
		require.Error(t, err)

		randomUUID := testrand.UUID()

		// good uuid
		result, err := endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{}, randomUUID[:])
		require.NoError(t, err)
		require.Equal(t, randomUUID, result)

		partnerID, err := endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("not-a-partner"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, uuid.UUID{}, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0) (common/v0.0.0-00010101000000-000000000000)"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, uuid.UUID{}, partnerID)
	})
}

func TestSetBucketAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		err := uplink.CreateBucket(ctx, planet.Satellites[0], "alpha")
		require.NoError(t, err)

		err = uplink.CreateBucket(ctx, planet.Satellites[0], "alpha-new")
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		partnerID := testrand.UUID()
		{ // bucket with no items
			err = metainfoClient.SetBucketAttribution(ctx, metainfo.SetBucketAttributionParams{
				Bucket:    "alpha",
				PartnerID: partnerID,
			})
			require.NoError(t, err)
		}

		{ // setting attribution on a bucket that doesn't exist should fail
			err = metainfoClient.SetBucketAttribution(ctx, metainfo.SetBucketAttributionParams{
				Bucket:    "beta",
				PartnerID: partnerID,
			})
			require.Error(t, err)
		}

		{ // add data to an attributed bucket
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "alpha", "path", []byte{1, 2, 3})
			assert.NoError(t, err)

			// trying to set attribution should be ignored
			err = metainfoClient.SetBucketAttribution(ctx, metainfo.SetBucketAttributionParams{
				Bucket:    "alpha",
				PartnerID: partnerID,
			})
			require.NoError(t, err)
		}

		{ // non attributed bucket, and adding files
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "alpha-new", "path", []byte{1, 2, 3})
			assert.NoError(t, err)

			// bucket with items
			err = metainfoClient.SetBucketAttribution(ctx, metainfo.SetBucketAttributionParams{
				Bucket:    "alpha-new",
				PartnerID: partnerID,
			})
			require.Error(t, err)
		}
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
