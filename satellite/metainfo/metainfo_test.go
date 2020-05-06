// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	satMetainfo "storj.io/storj/satellite/metainfo"
	"storj.io/uplink"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/storage/meta"
)

func TestInvalidAPIKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		throwawayKey, err := macaroon.NewAPIKey([]byte("secret"))
		require.NoError(t, err)

		for _, invalidAPIKey := range []string{"", "invalid", "testKey"} {
			client, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], throwawayKey)
			require.NoError(t, err)
			defer ctx.Check(client.Close)

			client.SetRawAPIKey([]byte(invalidAPIKey))

			_, err = client.BeginObject(ctx, metainfo.BeginObjectParams{})
			assertInvalidArgument(t, err, false)

			_, _, err = client.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{})
			assertInvalidArgument(t, err, false)

			_, err = client.ListBuckets(ctx, metainfo.ListBucketsParams{})
			assertInvalidArgument(t, err, false)

			_, _, err = client.ListObjects(ctx, metainfo.ListObjectsParams{})
			assertInvalidArgument(t, err, false)

			err = client.CommitObject(ctx, metainfo.CommitObjectParams{})
			assertInvalidArgument(t, err, false)

			_, err = client.CreateBucket(ctx, metainfo.CreateBucketParams{})
			assertInvalidArgument(t, err, false)

			_, err = client.DeleteBucket(ctx, metainfo.DeleteBucketParams{})
			assertInvalidArgument(t, err, false)

			_, _, err = client.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{})
			assertInvalidArgument(t, err, false)

			err = client.FinishDeleteObject(ctx, metainfo.FinishDeleteObjectParams{})
			assertInvalidArgument(t, err, false)

			_, err = client.GetBucket(ctx, metainfo.GetBucketParams{})
			assertInvalidArgument(t, err, false)

			_, err = client.GetObject(ctx, metainfo.GetObjectParams{})
			assertInvalidArgument(t, err, false)

			err = client.SetBucketAttribution(ctx, metainfo.SetBucketAttributionParams{})
			assertInvalidArgument(t, err, false)

			_, err = client.GetProjectInfo(ctx)
			assertInvalidArgument(t, err, false)

			// these methods needs StreamID to do authentication

			signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			satStreamID := &pb.SatStreamID{
				CreationDate: time.Now(),
			}
			signedStreamID, err := signing.SignStreamID(ctx, signer, satStreamID)
			require.NoError(t, err)

			encodedStreamID, err := pb.Marshal(signedStreamID)
			require.NoError(t, err)

			streamID, err := storj.StreamIDFromBytes(encodedStreamID)
			require.NoError(t, err)

			_, _, _, err = client.BeginSegment(ctx, metainfo.BeginSegmentParams{StreamID: streamID})
			assertInvalidArgument(t, err, false)

			_, _, _, err = client.BeginDeleteSegment(ctx, metainfo.BeginDeleteSegmentParams{StreamID: streamID})
			assertInvalidArgument(t, err, false)

			err = client.MakeInlineSegment(ctx, metainfo.MakeInlineSegmentParams{StreamID: streamID})
			assertInvalidArgument(t, err, false)

			_, _, err = client.ListSegments(ctx, metainfo.ListSegmentsParams{StreamID: streamID})
			assertInvalidArgument(t, err, false)

			_, _, err = client.DownloadSegment(ctx, metainfo.DownloadSegmentParams{StreamID: streamID})
			assertInvalidArgument(t, err, false)

			// these methods needs SegmentID

			signedSegmentID, err := signing.SignSegmentID(ctx, signer, &pb.SatSegmentID{
				StreamId:     satStreamID,
				CreationDate: time.Now(),
			})
			require.NoError(t, err)

			encodedSegmentID, err := pb.Marshal(signedSegmentID)
			require.NoError(t, err)

			segmentID, err := storj.SegmentIDFromBytes(encodedSegmentID)
			require.NoError(t, err)

			err = client.CommitSegment(ctx, metainfo.CommitSegmentParams{SegmentID: segmentID})
			assertInvalidArgument(t, err, false)
		}
	})
}

func assertInvalidArgument(t *testing.T, err error, allowed bool) {
	t.Helper()

	// If it's allowed, we allow any non-unauthenticated error because
	// some calls error after authentication checks.
	if !allowed {
		assert.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
	}
}

func TestServiceList(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		items := []struct {
			Key   string
			Value []byte
		}{
			{Key: "sample.😶", Value: []byte{1}},
			{Key: "müsic", Value: []byte{2}},
			{Key: "müsic/söng1.mp3", Value: []byte{3}},
			{Key: "müsic/söng2.mp3", Value: []byte{4}},
			{Key: "müsic/album/söng3.mp3", Value: []byte{5}},
			{Key: "müsic/söng4.mp3", Value: []byte{6}},
			{Key: "ビデオ/movie.mkv", Value: []byte{7}},
		}

		for _, item := range items {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", item.Key, item.Value)
			assert.NoError(t, err)
		}

		config := planet.Uplinks[0].GetConfig(planet.Satellites[0])
		project, bucket, err := planet.Uplinks[0].GetProjectAndBucket(ctx, planet.Satellites[0], "testbucket", config)
		require.NoError(t, err)
		defer ctx.Check(bucket.Close)
		defer ctx.Check(project.Close)
		list, err := bucket.ListObjects(ctx, &storj.ListOptions{Recursive: true, Direction: storj.After})
		require.NoError(t, err)

		expected := []storj.Object{
			{Path: "müsic"},
			{Path: "müsic/album/söng3.mp3"},
			{Path: "müsic/söng1.mp3"},
			{Path: "müsic/söng2.mp3"},
			{Path: "müsic/söng4.mp3"},
			{Path: "sample.😶"},
			{Path: "ビデオ/movie.mkv"},
		}

		require.Equal(t, len(expected), len(list.Items))
		sort.Slice(list.Items, func(i, k int) bool {
			return list.Items[i].Path < list.Items[k].Path
		})
		for i, item := range expected {
			require.Equal(t, item.Path, list.Items[i].Path)
			require.Equal(t, item.IsPrefix, list.Items[i].IsPrefix)
		}

		list, err = bucket.ListObjects(ctx, &storj.ListOptions{Recursive: false, Direction: storj.After})
		require.NoError(t, err)

		expected = []storj.Object{
			{Path: "müsic"},
			{Path: "müsic/", IsPrefix: true},
			{Path: "sample.😶"},
			{Path: "ビデオ/", IsPrefix: true},
		}

		require.Equal(t, len(expected), len(list.Items))
		sort.Slice(list.Items, func(i, k int) bool {
			return list.Items[i].Path < list.Items[k].Path
		})
		for i, item := range expected {
			t.Log(item.Path, list.Items[i].Path)
			require.Equal(t, item.Path, list.Items[i].Path)
			require.Equal(t, item.IsPrefix, list.Items[i].IsPrefix)
		}
	})
}

func TestExpirationTimeSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "my-bucket-name")
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		for _, r := range []struct {
			expirationDate time.Time
			errFlag        bool
		}{
			{ // expiration time not set
				time.Time{},
				false,
			},
			{ // 10 days into future
				time.Now().AddDate(0, 0, 10),
				false,
			},
			{ // current time
				time.Now(),
				true,
			},
			{ // 10 days into past
				time.Now().AddDate(0, 0, -10),
				true,
			},
		} {
			_, err := metainfoClient.BeginObject(ctx, metainfo.BeginObjectParams{
				Bucket:        []byte("my-bucket-name"),
				EncryptedPath: []byte("path"),
				ExpiresAt:     r.expirationDate,
			})
			if r.errFlag {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	})
}

func TestGetProjectInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey0 := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		apiKey1 := planet.Uplinks[1].APIKey[planet.Satellites[0].ID()]

		metainfo0, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey0)
		require.NoError(t, err)

		metainfo1, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey1)
		require.NoError(t, err)

		info0, err := metainfo0.GetProjectInfo(ctx)
		require.NoError(t, err)
		require.NotNil(t, info0.ProjectSalt)

		info1, err := metainfo1.GetProjectInfo(ctx)
		require.NoError(t, err)
		require.NotNil(t, info1.ProjectSalt)

		// Different projects should have different salts
		require.NotEqual(t, info0.ProjectSalt, info1.ProjectSalt)
	})
}

func TestBucketNameValidation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		rs := &pb.RedundancyScheme{
			MinReq:           1,
			RepairThreshold:  1,
			SuccessThreshold: 3,
			Total:            4,
			ErasureShareSize: 1024,
			Type:             pb.RedundancyScheme_RS,
		}

		validNames := []string{
			"tes", "testbucket",
			"test-bucket", "testbucket9",
			"9testbucket", "a.b",
			"test.bucket", "test-one.bucket-one",
			"test.bucket.one",
			"testbucket-63-0123456789012345678901234567890123456789012345abc",
		}
		for _, name := range validNames {
			_, _, _, err = metainfoClient.CreateSegmentOld(ctx, name, "", -1, rs, 1, time.Now().Add(time.Hour))
			require.NoError(t, err, "bucket name: %v", name)

			_, err = metainfoClient.CreateBucket(ctx, metainfo.CreateBucketParams{
				Name: []byte(name),
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
		}
		for _, name := range invalidNames {
			_, _, _, err = metainfoClient.CreateSegmentOld(ctx, name, "", -1, rs, 1, time.Now().Add(time.Hour))
			require.Error(t, err, "bucket name: %v", name)

			_, err = metainfoClient.CreateBucket(ctx, metainfo.CreateBucketParams{
				Name: []byte(name),
			})
			require.Error(t, err, "bucket name: %v", name)
		}
	})
}

func TestListGetObjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		files := make([]string, 10)
		data := testrand.Bytes(1 * memory.KiB)
		for i := 0; i < len(files); i++ {
			files[i] = "path" + strconv.Itoa(i)
			err := uplink.Upload(ctx, planet.Satellites[0], "testbucket", files[i], data)
			require.NoError(t, err)
		}

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		expectedBucketName := "testbucket"
		items, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte(expectedBucketName),
		})
		require.NoError(t, err)
		require.Equal(t, len(files), len(items))
		for _, item := range items {
			require.NotEmpty(t, item.EncryptedPath)
			require.True(t, item.CreatedAt.Before(time.Now()))

			object, err := metainfoClient.GetObject(ctx, metainfo.GetObjectParams{
				Bucket:        []byte(expectedBucketName),
				EncryptedPath: item.EncryptedPath,
			})
			require.NoError(t, err)
			require.Equal(t, item.EncryptedPath, []byte(object.Path))

			require.NotEmpty(t, object.StreamID)
		}

		items, _, err = metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte(expectedBucketName),
			Limit:  3,
		})
		require.NoError(t, err)
		require.Equal(t, 3, len(items))
	})
}

func TestBucketExistenceCheck(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		// test object methods for bucket existence check
		_, err = metainfoClient.BeginObject(ctx, metainfo.BeginObjectParams{
			Bucket:        []byte("non-existing-bucket"),
			EncryptedPath: []byte("encrypted-path"),
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		require.Equal(t, storj.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errs.Unwrap(err).Error())

		_, _, err = metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte("non-existing-bucket"),
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		require.Equal(t, storj.ErrBucketNotFound.New("%s", "non-existing-bucket").Error(), errs.Unwrap(err).Error())
	})
}

func TestBeginCommitListSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		config := uplink.GetConfig(planet.Satellites[0])
		metainfoService := planet.Satellites[0].Metainfo.Service

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		projectID := projects[0].ID

		bucket := storj.Bucket{
			Name:       "initial-bucket",
			ProjectID:  projectID,
			PathCipher: config.GetEncryptionParameters().CipherSuite,
		}
		_, err = metainfoService.CreateBucket(ctx, bucket)
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		params := metainfo.BeginObjectParams{
			Bucket:        []byte(bucket.Name),
			EncryptedPath: []byte("encrypted-path"),
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      256,
				RequiredShares: 1,
				RepairShares:   1,
				OptimalShares:  3,
				TotalShares:    4,
			},
			EncryptionParameters: storj.EncryptionParameters{},
			ExpiresAt:            time.Now().Add(24 * time.Hour),
		}
		beginObjectResponse, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		segmentID, limits, _, err := metainfoClient.BeginSegment(ctx, metainfo.BeginSegmentParams{
			StreamID: beginObjectResponse.StreamID,
			Position: storj.SegmentPosition{
				Index: 0,
			},
			MaxOrderLimit: memory.MiB.Int64(),
		})
		require.NoError(t, err)

		fullIDMap := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			fullIDMap[node.ID()] = node.Identity
		}

		makeResult := func(num int32) *pb.SegmentPieceUploadResult {
			nodeID := limits[num].Limit.StorageNodeId
			hash := &pb.PieceHash{
				PieceId:   limits[num].Limit.PieceId,
				PieceSize: 1048832,
				Timestamp: time.Now(),
			}

			fullID := fullIDMap[nodeID]
			require.NotNil(t, fullID)
			signer := signing.SignerFromFullIdentity(fullID)
			signedHash, err := signing.SignPieceHash(ctx, signer, hash)
			require.NoError(t, err)

			return &pb.SegmentPieceUploadResult{
				PieceNum: num,
				NodeId:   nodeID,
				Hash:     signedHash,
			}
		}
		err = metainfoClient.CommitSegment(ctx, metainfo.CommitSegmentParams{
			SegmentID: segmentID,

			SizeEncryptedData: memory.MiB.Int64(),
			UploadResult: []*pb.SegmentPieceUploadResult{
				makeResult(0),
				makeResult(1),
				makeResult(2),
			},
		})
		require.NoError(t, err)

		metadata, err := pb.Marshal(&pb.StreamMeta{
			NumberOfSegments: 1,
		})
		require.NoError(t, err)
		err = metainfoClient.CommitObject(ctx, metainfo.CommitObjectParams{
			StreamID:          beginObjectResponse.StreamID,
			EncryptedMetadata: metadata,
		})
		require.NoError(t, err)

		objects, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte(bucket.Name),
		})
		require.NoError(t, err)
		require.Len(t, objects, 1)

		require.Equal(t, params.EncryptedPath, objects[0].EncryptedPath)
		require.True(t, params.ExpiresAt.Equal(objects[0].ExpiresAt))

		object, err := metainfoClient.GetObject(ctx, metainfo.GetObjectParams{
			Bucket:        []byte(bucket.Name),
			EncryptedPath: objects[0].EncryptedPath,
		})
		require.NoError(t, err)

		segments, _, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
			StreamID: object.StreamID,
		})
		require.NoError(t, err)
		require.Len(t, segments, 1)
	})
}

func TestListSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		data := testrand.Bytes(15 * memory.KiB)
		config := uplink.GetConfig(planet.Satellites[0])
		config.Client.SegmentSize = memory.KiB
		err := uplink.UploadWithClientConfig(ctx, planet.Satellites[0], config, "testbucket", "test-path", data)
		require.NoError(t, err)

		// 15KiB + encryption should be uploaded into 16 segments with SegmentSize == 1KiB
		numberOfSegments := 16

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		items, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte("testbucket"),
			Limit:  1,
		})
		require.NoError(t, err)

		object, err := metainfoClient.GetObject(ctx, metainfo.GetObjectParams{
			Bucket:        []byte("testbucket"),
			EncryptedPath: items[0].EncryptedPath,
		})
		require.NoError(t, err)

		for i, test := range []struct {
			Index  int32
			Limit  int32
			Result int
			More   bool
		}{
			{Index: 0, Result: numberOfSegments},
			{Index: 0, Result: numberOfSegments, Limit: int32(numberOfSegments), More: false},
			{Index: 0, Result: 5, Limit: 5, More: true},
			{Index: 16, Result: 0, More: false},
			{Index: 11, Result: 5, Limit: 5, More: false},
			{Index: 15, Result: 1, More: false},
		} {
			segments, more, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: object.StreamID,
				Limit:    test.Limit,
				CursorPosition: storj.SegmentPosition{
					Index: test.Index,
				},
			})
			require.NoErrorf(t, err, "test case: %d", i)
			require.Lenf(t, segments, test.Result, "test case: %d", i)
			require.Equalf(t, test.More, more, "test case: %d", i)
			if !more && test.Result > 0 {
				require.Equalf(t, int32(-1), segments[test.Result-1].Position.Index, "test case: %d", i)
			}
		}
	})
}

func TestInlineSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		config := uplink.GetConfig(planet.Satellites[0])
		metainfoService := planet.Satellites[0].Metainfo.Service

		projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		projectID := projects[0].ID

		// TODO maybe split into separate cases
		// Test:
		// * create bucket
		// * begin object
		// * send several inline segments
		// * commit object
		// * list created object
		// * list object segments
		// * download segments
		// * delete segments and object

		bucket := storj.Bucket{
			Name:       "inline-segments-bucket",
			ProjectID:  projectID,
			PathCipher: config.GetEncryptionParameters().CipherSuite,
		}
		_, err = metainfoService.CreateBucket(ctx, bucket)
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		params := metainfo.BeginObjectParams{
			Bucket:        []byte(bucket.Name),
			EncryptedPath: []byte("encrypted-path"),
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      256,
				RequiredShares: 1,
				RepairShares:   1,
				OptimalShares:  3,
				TotalShares:    4,
			},
			EncryptionParameters: storj.EncryptionParameters{},
			ExpiresAt:            time.Now().Add(24 * time.Hour),
		}
		beginObjectResp, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		segments := []int32{0, 1, 2, 3, 4, 5, 6}
		segmentsData := make([][]byte, len(segments))
		for i, segment := range segments {
			segmentsData[i] = testrand.Bytes(memory.KiB)
			err = metainfoClient.MakeInlineSegment(ctx, metainfo.MakeInlineSegmentParams{
				StreamID: beginObjectResp.StreamID,
				Position: storj.SegmentPosition{
					Index: segment,
				},
				EncryptedInlineData: segmentsData[i],
			})
			require.NoError(t, err)
		}

		metadata, err := pb.Marshal(&pb.StreamMeta{
			NumberOfSegments: int64(len(segments)),
		})
		require.NoError(t, err)
		err = metainfoClient.CommitObject(ctx, metainfo.CommitObjectParams{
			StreamID:          beginObjectResp.StreamID,
			EncryptedMetadata: metadata,
		})
		require.NoError(t, err)

		objects, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte(bucket.Name),
		})
		require.NoError(t, err)
		require.Len(t, objects, 1)

		require.Equal(t, params.EncryptedPath, objects[0].EncryptedPath)
		require.True(t, params.ExpiresAt.Equal(objects[0].ExpiresAt))

		object, err := metainfoClient.GetObject(ctx, metainfo.GetObjectParams{
			Bucket:        params.Bucket,
			EncryptedPath: params.EncryptedPath,
		})
		require.NoError(t, err)

		{ // Confirm data larger than our configured max inline segment size of 4 KiB cannot be inlined
			beginObjectResp, err := metainfoClient.BeginObject(ctx, metainfo.BeginObjectParams{
				Bucket:        []byte(bucket.Name),
				EncryptedPath: []byte("too-large-inline-segment"),
			})
			require.NoError(t, err)

			data := testrand.Bytes(10 * memory.KiB)
			err = metainfoClient.MakeInlineSegment(ctx, metainfo.MakeInlineSegmentParams{
				StreamID: beginObjectResp.StreamID,
				Position: storj.SegmentPosition{
					Index: 0,
				},
				EncryptedInlineData: data,
			})
			require.Error(t, err)
		}
		{ // test listing inline segments
			for _, test := range []struct {
				Index  int32
				Limit  int
				Result int
				More   bool
			}{
				{Index: 0, Result: len(segments), More: false},
				{Index: 2, Result: len(segments) - 2, More: false},
				{Index: 0, Result: 3, More: true, Limit: 3},
				{Index: 0, Result: len(segments), More: false, Limit: len(segments)},
				{Index: 0, Result: len(segments) - 1, More: true, Limit: len(segments) - 1},
			} {
				items, more, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
					StreamID: object.StreamID,
					CursorPosition: storj.SegmentPosition{
						Index: test.Index,
					},
					Limit: int32(test.Limit),
				})
				require.NoError(t, err)
				require.Equal(t, test.Result, len(items))
				require.Equal(t, test.More, more)
			}
		}

		{ // test download inline segments
			items, _, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: object.StreamID,
			})
			require.NoError(t, err)
			require.Equal(t, len(segments), len(items))

			for i, item := range items {
				info, limits, err := metainfoClient.DownloadSegment(ctx, metainfo.DownloadSegmentParams{
					StreamID: object.StreamID,
					Position: storj.SegmentPosition{
						Index: item.Position.Index,
					},
				})
				require.NoError(t, err)
				require.Nil(t, limits)
				require.Equal(t, segmentsData[i], info.EncryptedInlineData)
			}
		}

		{ // test deleting segments
			streamID, _, err := metainfoClient.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{
				Bucket:        params.Bucket,
				EncryptedPath: params.EncryptedPath,
			})
			require.NoError(t, err)
			segments, more, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: streamID,
			})
			require.NoError(t, err)
			require.Empty(t, segments)
			require.False(t, more)

			err = metainfoClient.FinishDeleteObject(ctx, metainfo.FinishDeleteObjectParams{
				StreamID: streamID,
			})
			require.NoError(t, err)
		}
	})
}

func TestRemoteSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		expectedBucketName := "remote-segments-bucket"
		err := uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "file-object", testrand.Bytes(10*memory.KiB))
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		items, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte(expectedBucketName),
		})
		require.NoError(t, err)
		require.Len(t, items, 1)

		{
			// Get object
			// List segments
			// Download segment

			object, err := metainfoClient.GetObject(ctx, metainfo.GetObjectParams{
				Bucket:        []byte(expectedBucketName),
				EncryptedPath: items[0].EncryptedPath,
			})
			require.NoError(t, err)

			segments, _, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: object.StreamID,
			})
			require.NoError(t, err)
			require.Len(t, segments, 1)

			_, limits, err := metainfoClient.DownloadSegment(ctx, metainfo.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: storj.SegmentPosition{
					Index: segments[0].Position.Index,
				},
			})
			require.NoError(t, err)
			require.NotEmpty(t, limits)
		}

		{
			// Begin deleting object
			// List segments
			// Begin/Finish deleting segment
			// List objects

			streamID, _, err := metainfoClient.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{
				Bucket:        []byte(expectedBucketName),
				EncryptedPath: items[0].EncryptedPath,
			})
			require.NoError(t, err)

			segments, more, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: streamID,
			})
			require.NoError(t, err)
			require.Empty(t, segments)
			require.False(t, more)

			err = metainfoClient.FinishDeleteObject(ctx, metainfo.FinishDeleteObjectParams{
				StreamID: streamID,
			})
			require.NoError(t, err)

			items, _, err = metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
				Bucket: []byte(expectedBucketName),
			})
			require.NoError(t, err)
			require.Len(t, items, 0)
		}
	})
}

func TestIDs(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		{
			streamID := testrand.StreamID(256)
			err = metainfoClient.CommitObject(ctx, metainfo.CommitObjectParams{
				StreamID: streamID,
			})
			require.Error(t, err) // invalid streamID

			segmentID := testrand.SegmentID(512)
			err = metainfoClient.CommitSegment(ctx, metainfo.CommitSegmentParams{
				SegmentID: segmentID,
			})
			require.Error(t, err) // invalid segmentID
		}

		satellitePeer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)

		{ // streamID expired
			signedStreamID, err := signing.SignStreamID(ctx, satellitePeer, &pb.SatStreamID{
				CreationDate: time.Now().Add(-36 * time.Hour),
			})
			require.NoError(t, err)

			encodedStreamID, err := pb.Marshal(signedStreamID)
			require.NoError(t, err)

			streamID, err := storj.StreamIDFromBytes(encodedStreamID)
			require.NoError(t, err)

			err = metainfoClient.CommitObject(ctx, metainfo.CommitObjectParams{
				StreamID: streamID,
			})
			require.Error(t, err)
		}

		{ // segment id missing stream id
			signedSegmentID, err := signing.SignSegmentID(ctx, satellitePeer, &pb.SatSegmentID{
				CreationDate: time.Now().Add(-1 * time.Hour),
			})
			require.NoError(t, err)

			encodedSegmentID, err := pb.Marshal(signedSegmentID)
			require.NoError(t, err)

			segmentID, err := storj.SegmentIDFromBytes(encodedSegmentID)
			require.NoError(t, err)

			err = metainfoClient.CommitSegment(ctx, metainfo.CommitSegmentParams{
				SegmentID: segmentID,
			})
			require.Error(t, err)
		}

		{ // segmentID expired
			signedSegmentID, err := signing.SignSegmentID(ctx, satellitePeer, &pb.SatSegmentID{
				CreationDate: time.Now().Add(-36 * time.Hour),
				StreamId: &pb.SatStreamID{
					CreationDate: time.Now(),
				},
			})
			require.NoError(t, err)

			encodedSegmentID, err := pb.Marshal(signedSegmentID)
			require.NoError(t, err)

			segmentID, err := storj.SegmentIDFromBytes(encodedSegmentID)
			require.NoError(t, err)

			err = metainfoClient.CommitSegment(ctx, metainfo.CommitSegmentParams{
				SegmentID: segmentID,
			})
			require.Error(t, err)
		}
	})
}

func TestBatch(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		{ // create few buckets and list them in one batch
			requests := make([]metainfo.BatchItem, 0)
			numOfBuckets := 5
			for i := 0; i < numOfBuckets; i++ {
				requests = append(requests, &metainfo.CreateBucketParams{
					Name:                []byte("test-bucket-" + strconv.Itoa(i)),
					PathCipher:          storj.EncAESGCM,
					DefaultSegmentsSize: memory.MiB.Int64(),
				})
			}
			requests = append(requests, &metainfo.ListBucketsParams{
				ListOpts: storj.BucketListOptions{
					Cursor:    "",
					Direction: storj.After,
				},
			})
			responses, err := metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfBuckets+1, len(responses))

			for i := 0; i < numOfBuckets; i++ {
				response, err := responses[i].CreateBucket()
				require.NoError(t, err)
				require.Equal(t, "test-bucket-"+strconv.Itoa(i), response.Bucket.Name)

				_, err = responses[i].GetBucket()
				require.Error(t, err)
			}

			bucketsListResp, err := responses[numOfBuckets].ListBuckets()
			require.NoError(t, err)
			require.Equal(t, numOfBuckets, len(bucketsListResp.BucketList.Items))
		}

		{ // create bucket, object, upload inline segments in batch, download inline segments in batch
			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "second-test-bucket")
			require.NoError(t, err)

			requests := make([]metainfo.BatchItem, 0)
			requests = append(requests, &metainfo.BeginObjectParams{
				Bucket:        []byte("second-test-bucket"),
				EncryptedPath: []byte("encrypted-path"),
			})
			numOfSegments := 10
			expectedData := make([][]byte, numOfSegments)
			for i := 0; i < numOfSegments; i++ {
				expectedData[i] = testrand.Bytes(memory.KiB)

				requests = append(requests, &metainfo.MakeInlineSegmentParams{
					Position: storj.SegmentPosition{
						Index: int32(i),
					},
					EncryptedInlineData: expectedData[i],
				})
			}

			metadata, err := pb.Marshal(&pb.StreamMeta{
				NumberOfSegments: int64(numOfSegments),
			})
			require.NoError(t, err)
			requests = append(requests, &metainfo.CommitObjectParams{
				EncryptedMetadata: metadata,
			})
			requests = append(requests, &metainfo.ListSegmentsParams{})

			responses, err := metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfSegments+3, len(responses))

			listResponse, err := responses[numOfSegments+2].ListSegment()
			require.NoError(t, err)
			require.Equal(t, numOfSegments, len(listResponse.Items))

			requests = make([]metainfo.BatchItem, 0)
			requests = append(requests, &metainfo.GetObjectParams{
				Bucket:        []byte("second-test-bucket"),
				EncryptedPath: []byte("encrypted-path"),
			})
			for _, segment := range listResponse.Items {
				requests = append(requests, &metainfo.DownloadSegmentParams{
					Position: segment.Position,
				})
			}
			responses, err = metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, len(listResponse.Items)+1, len(responses))

			for i, response := range responses[1:] {
				downloadResponse, err := response.DownloadSegment()
				require.NoError(t, err)

				require.Equal(t, expectedData[i], downloadResponse.Info.EncryptedInlineData)
			}
		}

		{ // test case when StreamID is not set automatically
			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "third-test-bucket")
			require.NoError(t, err)

			beginObjectResp, err := metainfoClient.BeginObject(ctx, metainfo.BeginObjectParams{
				Bucket:        []byte("third-test-bucket"),
				EncryptedPath: []byte("encrypted-path"),
			})
			require.NoError(t, err)

			requests := make([]metainfo.BatchItem, 0)
			numOfSegments := 10
			expectedData := make([][]byte, numOfSegments)
			for i := 0; i < numOfSegments; i++ {
				expectedData[i] = testrand.Bytes(memory.KiB)

				requests = append(requests, &metainfo.MakeInlineSegmentParams{
					StreamID: beginObjectResp.StreamID,
					Position: storj.SegmentPosition{
						Index: int32(i),
					},
					EncryptedInlineData: expectedData[i],
				})
			}

			metadata, err := pb.Marshal(&pb.StreamMeta{
				NumberOfSegments: int64(numOfSegments),
			})
			require.NoError(t, err)
			requests = append(requests, &metainfo.CommitObjectParams{
				StreamID:          beginObjectResp.StreamID,
				EncryptedMetadata: metadata,
			})

			responses, err := metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfSegments+1, len(responses))
		}
	})
}

func TestRateLimit(t *testing.T) {
	rateLimit := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = float64(rateLimit)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)
	})
}

func TestRateLimit_Disabled(t *testing.T) {
	rateLimit := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Enabled = false
				config.Metainfo.RateLimiter.Rate = float64(rateLimit)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 0)
	})
}

func TestRateLimit_ProjectRateLimitOverride(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = 2
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		rateLimit := 3
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 1)
	})
}

func TestRateLimit_ProjectRateLimitOverrideCachedExpired(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = 2
				config.Metainfo.RateLimiter.CacheExpiration = time.Second
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		rateLimit := 3
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		var group1 errs2.Group

		for i := 0; i <= rateLimit; i++ {
			group1.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		group1Errs := group1.Wait()
		require.Len(t, group1Errs, 1)

		rateLimit = 1
		projects[0].RateLimit = &rateLimit

		err = satellite.DB.Console().Projects().Update(ctx, &projects[0])
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		var group2 errs2.Group

		for i := 0; i <= rateLimit; i++ {
			group2.Go(func() error {
				return ul.CreateBucket(ctx, satellite, testrand.BucketName())
			})
		}
		group2Errs := group2.Wait()
		require.Len(t, group2Errs, 1)
	})
}

func TestOverwriteZombieSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]
		config := uplink.GetConfig(planet.Satellites[0])
		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		var testCases = []struct {
			deletedSegments []int32
			objectSize      memory.Size
			segmentSize     memory.Size
			label           string
		}{
			{label: "inline", deletedSegments: []int32{0, -1}, objectSize: 5 * memory.KiB, segmentSize: 1 * memory.KiB},
			{label: "remote", deletedSegments: []int32{0, -1}, objectSize: 23 * memory.KiB, segmentSize: 5 * memory.KiB},
		}

		for i, tc := range testCases {
			i := i
			tc := tc
			t.Run(tc.label, func(t *testing.T) {
				data := testrand.Bytes(tc.objectSize)
				config.Client.SegmentSize = tc.segmentSize
				bucket := "testbucket" + strconv.Itoa(i)
				objectKey := "test-path" + strconv.Itoa(i)
				err := uplink.UploadWithClientConfig(ctx, planet.Satellites[0], config, bucket, objectKey, data)
				require.NoError(t, err)

				items, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
					Bucket: []byte(bucket),
					Limit:  1,
				})
				require.NoError(t, err)

				object, err := metainfoClient.GetObject(ctx, metainfo.GetObjectParams{
					Bucket:        []byte(bucket),
					EncryptedPath: items[0].EncryptedPath,
				})
				require.NoError(t, err)

				// delete some segments to leave only zombie segments
				for _, segment := range tc.deletedSegments {
					_, _, _, err = metainfoClient.BeginDeleteSegment(ctx, metainfo.BeginDeleteSegmentParams{
						StreamID: object.StreamID,
						Position: storj.SegmentPosition{
							Index: segment,
						},
					})
					require.NoError(t, err)
				}

				err = uplink.UploadWithClientConfig(ctx, planet.Satellites[0], config, bucket, objectKey, data)
				require.NoError(t, err)
			})

		}
	})
}

func TestBucketEmptinessBeforeDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i := 0; i < 10; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket", "object-key"+strconv.Itoa(i), testrand.Bytes(memory.KiB))
			require.NoError(t, err)
		}

		for i := 0; i < 10; i++ {
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

func TestDeleteBatchWithoutPermission(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		apiKey, err = apiKey.Restrict(macaroon.Caveat{
			DisallowLists: true,
			DisallowReads: true,
		})
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		responses, err := metainfoClient.Batch(ctx,
			// this request was causing panic becase for deleting object
			// its possible to return no error and empty response for
			// specific set of permissions, see `apiKey.Restrict` from above
			&metainfo.BeginDeleteObjectParams{
				Bucket:        []byte("test-bucket"),
				EncryptedPath: []byte("not-existing-object"),
			},

			// TODO this code should be enabled then issue with read permissions in
			// DeleteBucket method currently user have always permission to read bucket
			// https://storjlabs.atlassian.net/browse/USR-603
			// when it will be fixed commented code from bellow should replace existing DeleteBucketParams
			// the same situation like above
			// &metainfo.DeleteBucketParams{
			// 	Name: []byte("not-existing-bucket"),
			// },

			&metainfo.DeleteBucketParams{
				Name: []byte("test-bucket"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, 2, len(responses))
	})
}

func TestInlineSegmentThreshold(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID

		{ // limit is max inline segment size + encryption overhead
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket-inline", "inline-object", testrand.Bytes(4*memory.KiB))
			require.NoError(t, err)

			// we don't know encrypted path
			prefix, err := satMetainfo.CreatePath(ctx, projectID, -1, []byte("test-bucket-inline"), []byte{})
			require.NoError(t, err)

			items, _, err := planet.Satellites[0].Metainfo.Service.List(ctx, prefix, "", false, 0, meta.All)
			require.NoError(t, err)
			require.Equal(t, 1, len(items))

			pointer, err := planet.Satellites[0].Metainfo.Service.Get(ctx, prefix+"/"+items[0].Path)
			require.NoError(t, err)
			require.Equal(t, pb.Pointer_INLINE, pointer.Type)
		}

		{ // one more byte over limit should enough to create remote segment
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket-remote", "remote-object", testrand.Bytes(4*memory.KiB+1))
			require.NoError(t, err)

			// we don't know encrypted path
			prefix, err := satMetainfo.CreatePath(ctx, projectID, -1, []byte("test-bucket-remote"), []byte{})
			require.NoError(t, err)

			items, _, err := planet.Satellites[0].Metainfo.Service.List(ctx, prefix, "", false, 0, meta.All)
			require.NoError(t, err)
			require.Equal(t, 1, len(items))

			pointer, err := planet.Satellites[0].Metainfo.Service.Get(ctx, prefix+"/"+items[0].Path)
			require.NoError(t, err)
			require.Equal(t, pb.Pointer_REMOTE, pointer.Type)
		}
	})
}
