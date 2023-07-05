// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/buckets"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
)

func TestBucketExistenceCheck(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		// test object methods for bucket existence check
		_, err = metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
			Bucket:             []byte("non-existing-bucket"),
			EncryptedObjectKey: []byte("encrypted-path"),
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		require.Equal(t, buckets.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errs.Unwrap(err).Error())

		_, _, err = metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
			Bucket: []byte("non-existing-bucket"),
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		require.Equal(t, buckets.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errs.Unwrap(err).Error())
	})
}

func TestMaxOutBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		limit := planet.Satellites[0].Config.Metainfo.ProjectLimits.MaxBuckets
		for i := 1; i <= limit; i++ {
			name := "test" + strconv.Itoa(i)
			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], name)
			require.NoError(t, err)
		}
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], fmt.Sprintf("test%d", limit+1))
		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("number of allocated buckets (%d) exceeded", limit))
	})
}

func TestBucketNameValidation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		validNames := []string{
			"tes", "testbucket",
			"test-bucket", "testbucket9",
			"9testbucket", "a.b",
			"test.bucket", "test-one.bucket-one",
			"test.bucket.one",
			"testbucket-63-0123456789012345678901234567890123456789012345abc",
		}
		for _, name := range validNames {
			_, err = metainfoClient.CreateBucket(ctx, metaclient.CreateBucketParams{
				Name: []byte(name),
			})
			require.NoError(t, err, "bucket name: %v", name)

			_, err = metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte(name),
				EncryptedObjectKey: []byte("123"),
				Version:            0,
				ExpiresAt:          time.Now().Add(16 * 24 * time.Hour),
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: storj.EncAESGCM,
					BlockSize:   256,
				},
			})
			require.NoError(t, err, "bucket name: %v", name)
		}

		invalidNames := []string{
			"", "t", "te", "-testbucket",
			"testbucket-", "-testbucket-",
			"a.b.", "test.bucket-.one",
			"test.-bucket.one", "1.2.3.4",
			"192.168.1.234", "testBUCKET",
			"test/bucket",
			"testbucket-64-0123456789012345678901234567890123456789012345abcd",
			"test\\", "test%",
		}
		for _, name := range invalidNames {
			_, err = metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte(name),
				EncryptedObjectKey: []byte("123"),
			})
			require.Error(t, err, "bucket name: %v", name)
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

			_, err = metainfoClient.CreateBucket(ctx, metaclient.CreateBucketParams{
				Name: []byte(name),
			})
			require.Error(t, err, "bucket name: %v", name)
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
		}
	})
}

func TestBucketEmptinessBeforeDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i := 0; i < 5; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket", "object-key"+strconv.Itoa(i), testrand.Bytes(memory.KiB))
			require.NoError(t, err)
		}

		for i := 0; i < 5; i++ {
			err := planet.Uplinks[0].DeleteBucket(ctx, planet.Satellites[0], "test-bucket")
			require.Error(t, err)
			require.True(t, errors.Is(err, uplink.ErrBucketNotEmpty))

			err = planet.Uplinks[0].DeleteObject(ctx, planet.Satellites[0], "test-bucket", "object-key"+strconv.Itoa(i))
			require.NoError(t, err)
		}

		err := planet.Uplinks[0].DeleteBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)
	})
}

func TestDeleteBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
				testplanet.MaxSegmentSize(13*memory.KiB),
			),
		},
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		satelliteSys := planet.Satellites[0]
		uplnk := planet.Uplinks[0]

		expectedBucketName := "remote-segments-bucket"

		err := uplnk.Upload(ctx, planet.Satellites[0], expectedBucketName, "single-segment-object", testrand.Bytes(10*memory.KiB))
		require.NoError(t, err)
		err = uplnk.Upload(ctx, planet.Satellites[0], expectedBucketName, "multi-segment-object", testrand.Bytes(50*memory.KiB))
		require.NoError(t, err)
		err = uplnk.Upload(ctx, planet.Satellites[0], expectedBucketName, "remote-segment-inline-object", testrand.Bytes(33*memory.KiB))
		require.NoError(t, err)

		objects, err := satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 3)

		delResp, err := satelliteSys.API.Metainfo.Endpoint.DeleteBucket(ctx, &pb.BucketDeleteRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      []byte(expectedBucketName),
			DeleteAll: true,
		})
		require.NoError(t, err)
		require.Equal(t, int64(3), delResp.DeletedObjectsCount)

		// confirm the bucket is deleted
		buckets, err := satelliteSys.Metainfo.Endpoint.ListBuckets(ctx, &pb.BucketListRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Direction: buckets.DirectionForward,
		})
		require.NoError(t, err)
		require.Len(t, buckets.GetItems(), 0)
	})
}

func TestListBucketsWithAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		type testCase struct {
			UserAgent string
			Bucket    string
		}

		var testCases = []testCase{
			{
				Bucket: "bucket-without-user-agent",
			},
			{
				UserAgent: "storj",
				Bucket:    "bucket-with-user-agent",
			},
		}

		bucketExists := func(tc testCase, buckets *pb.BucketListResponse) bool {
			for _, bucket := range buckets.Items {
				if string(bucket.Name) == tc.Bucket {
					require.EqualValues(t, tc.UserAgent, string(bucket.UserAgent))
					return true
				}
			}
			t.Fatalf("bucket was not found in results:%s", tc.Bucket)
			return false
		}

		for _, tc := range testCases {
			config := uplink.Config{
				UserAgent: tc.UserAgent,
			}

			project, err := config.OpenProject(ctx, planet.Uplinks[0].Access[satellite.ID()])
			require.NoError(t, err)

			_, err = project.CreateBucket(ctx, tc.Bucket)
			require.NoError(t, err)

			buckets, err := satellite.Metainfo.Endpoint.ListBuckets(ctx, &pb.BucketListRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Direction: buckets.DirectionForward,
			})
			require.NoError(t, err)
			require.True(t, bucketExists(tc, buckets))
		}
	})
}

func TestBucketCreationWithDefaultPlacement(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID

		// change the default_placement of the project
		project, err := planet.Satellites[0].API.DB.Console().Projects().Get(ctx, projectID)
		project.DefaultPlacement = storj.EU
		require.NoError(t, err)
		err = planet.Satellites[0].API.DB.Console().Projects().Update(ctx, project)
		require.NoError(t, err)

		// create a new bucket
		up, err := planet.Uplinks[0].GetProject(ctx, planet.Satellites[0])
		require.NoError(t, err)

		_, err = up.CreateBucket(ctx, "eu1")
		require.NoError(t, err)

		// check if placement is set
		placement, err := planet.Satellites[0].API.DB.Buckets().GetBucketPlacement(ctx, []byte("eu1"), projectID)
		require.NoError(t, err)
		require.Equal(t, storj.EU, placement)

	})
}
