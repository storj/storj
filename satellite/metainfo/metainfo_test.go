// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	satMetainfo "storj.io/storj/satellite/metainfo"
	"storj.io/storj/uplink/metainfo"
)

// mockAPIKeys is mock for api keys store of pointerdb
type mockAPIKeys struct {
	info console.APIKeyInfo
	err  error
}

// GetByKey return api key info for given key
func (keys *mockAPIKeys) GetByKey(ctx context.Context, key macaroon.APIKey) (*console.APIKeyInfo, error) {
	return &keys.info, keys.err
}

func TestInvalidAPIKey(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	for _, invalidAPIKey := range []string{"", "invalid", "testKey"} {
		client, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], invalidAPIKey)
		require.NoError(t, err)
		defer ctx.Check(client.Close)

		_, _, err = client.CreateSegment(ctx, "hello", "world", 1, &pb.RedundancyScheme{}, 123, time.Now())
		assertUnauthenticated(t, err, false)

		_, err = client.CommitSegment(ctx, "testbucket", "testpath", 0, &pb.Pointer{}, nil)
		assertUnauthenticated(t, err, false)

		_, err = client.SegmentInfo(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err, false)

		_, _, err = client.ReadSegment(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err, false)

		_, err = client.DeleteSegment(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err, false)

		_, _, err = client.ListSegments(ctx, "testbucket", "", "", "", true, 1, 0)
		assertUnauthenticated(t, err, false)
	}
}

func TestRestrictedAPIKey(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	key, err := macaroon.ParseAPIKey(planet.Uplinks[0].APIKey[planet.Satellites[0].ID()])
	require.NoError(t, err)

	tests := []struct {
		Caveat               macaroon.Caveat
		CreateSegmentAllowed bool
		CommitSegmentAllowed bool
		SegmentInfoAllowed   bool
		ReadSegmentAllowed   bool
		DeleteSegmentAllowed bool
		ListSegmentsAllowed  bool
		ReadBucketAllowed    bool
	}{
		{ // Everything disallowed
			Caveat: macaroon.Caveat{
				DisallowReads:   true,
				DisallowWrites:  true,
				DisallowLists:   true,
				DisallowDeletes: true,
			},
			ReadBucketAllowed: true,
		},

		{ // Read only
			Caveat: macaroon.Caveat{
				DisallowWrites:  true,
				DisallowDeletes: true,
			},
			SegmentInfoAllowed:  true,
			ReadSegmentAllowed:  true,
			ListSegmentsAllowed: true,
			ReadBucketAllowed:   true,
		},

		{ // Write only
			Caveat: macaroon.Caveat{
				DisallowReads: true,
				DisallowLists: true,
			},
			CreateSegmentAllowed: true,
			CommitSegmentAllowed: true,
			DeleteSegmentAllowed: true,
			ReadBucketAllowed:    true,
		},

		{ // Bucket restriction
			Caveat: macaroon.Caveat{
				AllowedPaths: []*macaroon.Caveat_Path{{
					Bucket: []byte("otherbucket"),
				}},
			},
		},

		{ // Path restriction
			Caveat: macaroon.Caveat{
				AllowedPaths: []*macaroon.Caveat_Path{{
					Bucket:              []byte("testbucket"),
					EncryptedPathPrefix: []byte("otherpath"),
				}},
			},
			ReadBucketAllowed: true,
		},

		{ // Time restriction after
			Caveat: macaroon.Caveat{
				NotAfter: func(x time.Time) *time.Time { return &x }(time.Now()),
			},
		},

		{ // Time restriction before
			Caveat: macaroon.Caveat{
				NotBefore: func(x time.Time) *time.Time { return &x }(time.Now().Add(time.Hour)),
			},
		},
	}

	for _, test := range tests {
		restrictedKey, err := key.Restrict(test.Caveat)
		require.NoError(t, err)

		client, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], restrictedKey.Serialize())
		require.NoError(t, err)
		defer ctx.Check(client.Close)

		_, _, err = client.CreateSegment(ctx, "testbucket", "testpath", 1, &pb.RedundancyScheme{}, 123, time.Now())
		assertUnauthenticated(t, err, test.CreateSegmentAllowed)

		_, err = client.CommitSegment(ctx, "testbucket", "testpath", 0, &pb.Pointer{}, nil)
		assertUnauthenticated(t, err, test.CommitSegmentAllowed)

		_, err = client.SegmentInfo(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err, test.SegmentInfoAllowed)

		_, _, err = client.ReadSegment(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err, test.ReadSegmentAllowed)

		_, err = client.DeleteSegment(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err, test.DeleteSegmentAllowed)

		_, _, err = client.ListSegments(ctx, "testbucket", "testpath", "", "", true, 1, 0)
		assertUnauthenticated(t, err, test.ListSegmentsAllowed)

		_, _, err = client.ReadSegment(ctx, "testbucket", "", -1)
		assertUnauthenticated(t, err, test.ReadBucketAllowed)
	}
}

func assertUnauthenticated(t *testing.T, err error, allowed bool) {
	t.Helper()

	// If it's allowed, we allow any non-unauthenticated error because
	// some calls error after authentication checks.
	if err, ok := status.FromError(errs.Unwrap(err)); ok {
		assert.Equal(t, codes.Unauthenticated == err.Code(), !allowed)
	} else if !allowed {
		assert.Fail(t, "got unexpected error", "%T", err)
	}
}

func TestServiceList(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

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
	metainfo, _, cleanup, err := testplanet.DialMetainfo(ctx, planet.Uplinks[0].Log.Named("metainfo"), config, planet.Uplinks[0].Identity)
	require.NoError(t, err)
	defer ctx.Check(cleanup)

	type Test struct {
		Request  storj.ListOptions
		Expected storj.ObjectList // objects are partial
	}

	list, err := metainfo.ListObjects(ctx, "testbucket", storj.ListOptions{Recursive: true, Direction: storj.After})
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

	list, err = metainfo.ListObjects(ctx, "testbucket", storj.ListOptions{Recursive: false, Direction: storj.After})
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
}

func TestCommitSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RS.Validate = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		{
			// error if pointer is nil
			_, err = metainfo.CommitSegment(ctx, "bucket", "path", -1, nil, []*pb.OrderLimit{})
			require.Error(t, err)
		}
		{
			// error if number of remote pieces is lower then repair threshold
			redundancy := &pb.RedundancyScheme{
				MinReq:           1,
				RepairThreshold:  2,
				SuccessThreshold: 3,
				Total:            4,
				ErasureShareSize: 256,
			}
			expirationDate := time.Now()
			addresedLimits, rootPieceID, err := metainfo.CreateSegment(ctx, "bucket", "path", -1, redundancy, 1000, expirationDate)
			require.NoError(t, err)

			// create number of pieces below repair threshold
			usedForPieces := addresedLimits[:redundancy.RepairThreshold-1]
			pieces := make([]*pb.RemotePiece, len(usedForPieces))
			for i, limit := range usedForPieces {
				pieces[i] = &pb.RemotePiece{
					PieceNum: int32(i),
					NodeId:   limit.Limit.StorageNodeId,
				}
			}

			expirationDateProto, err := ptypes.TimestampProto(expirationDate)
			require.NoError(t, err)

			pointer := &pb.Pointer{
				Type: pb.Pointer_REMOTE,
				Remote: &pb.RemoteSegment{
					RootPieceId:  rootPieceID,
					Redundancy:   redundancy,
					RemotePieces: pieces,
				},
				ExpirationDate: expirationDateProto,
			}

			limits := make([]*pb.OrderLimit, len(addresedLimits))
			for i, addresedLimit := range addresedLimits {
				limits[i] = addresedLimit.Limit
			}
			_, err = metainfo.CommitSegment(ctx, "bucket", "path", -1, pointer, limits)
			require.Error(t, err)
			require.Contains(t, err.Error(), "less than or equal to the repair threshold")
		}
	})
}

func TestCreateSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RS.Validate = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		for _, r := range []struct {
			rs   *pb.RedundancyScheme
			fail bool
		}{
			{ // error - ErasureShareSize <= 0
				rs: &pb.RedundancyScheme{
					MinReq:           1,
					RepairThreshold:  2,
					SuccessThreshold: 3,
					Total:            4,
					ErasureShareSize: -1,
				},
				fail: true,
			},
			{ // error - any of the values are negative
				rs: &pb.RedundancyScheme{
					MinReq:           1,
					RepairThreshold:  -2,
					SuccessThreshold: 3,
					Total:            -4,
					ErasureShareSize: 10,
				},
				fail: true,
			},
			{ // error - MinReq >= RepairThreshold
				rs: &pb.RedundancyScheme{
					MinReq:           10,
					RepairThreshold:  2,
					SuccessThreshold: 3,
					Total:            4,
					ErasureShareSize: 10,
				},
				fail: true,
			},
			{ // error - MinReq >= RepairThreshold
				rs: &pb.RedundancyScheme{
					MinReq:           2,
					RepairThreshold:  2,
					SuccessThreshold: 3,
					Total:            4,
					ErasureShareSize: 10,
				},
				fail: true,
			},
			{ // error - RepairThreshold >= SuccessThreshol
				rs: &pb.RedundancyScheme{
					MinReq:           1,
					RepairThreshold:  3,
					SuccessThreshold: 3,
					Total:            4,
					ErasureShareSize: 10,
				},
				fail: true,
			},
			{ // error -  SuccessThreshold >= Total
				rs: &pb.RedundancyScheme{
					MinReq:           1,
					RepairThreshold:  2,
					SuccessThreshold: 4,
					Total:            4,
					ErasureShareSize: 10,
				},
				fail: true,
			},
			{ // ok - valid RS parameters
				rs: &pb.RedundancyScheme{
					MinReq:           1,
					RepairThreshold:  2,
					SuccessThreshold: 3,
					Total:            4,
					ErasureShareSize: 256,
				},
				fail: false,
			},
		} {
			_, _, err := metainfo.CreateSegment(ctx, "bucket", "path", -1, r.rs, 1000, time.Now())
			if r.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	})
}

func TestDoubleCommitSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		pointer, limits := runCreateSegment(ctx, t, metainfo)

		_, err = metainfo.CommitSegment(ctx, "my-bucket-name", "file/path", -1, pointer, limits)
		require.NoError(t, err)

		_, err = metainfo.CommitSegment(ctx, "my-bucket-name", "file/path", -1, pointer, limits)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing create request or request expired")
	})
}

func TestCommitSegmentPointer(t *testing.T) {
	// all tests needs to generate error
	tests := []struct {
		// defines how modify pointer before CommitSegment
		Modify       func(pointer *pb.Pointer)
		ErrorMessage string
	}{
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.ExpirationDate.Seconds += 100
			},
			ErrorMessage: "pointer expiration date does not match requested one",
		},
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.Remote.Redundancy.MinReq += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.Remote.Redundancy.RepairThreshold += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.Remote.Redundancy.SuccessThreshold += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.Remote.Redundancy.Total += 100
			},
			// this error is triggered earlier then Create/Commit RS comparison
			ErrorMessage: "invalid no order limit for piece",
		},
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.Remote.Redundancy.ErasureShareSize += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.Remote.Redundancy.Type = 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(pointer *pb.Pointer) {
				pointer.Type = pb.Pointer_INLINE
			},
			ErrorMessage: "pointer type is INLINE but remote segment is set",
		},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		for _, test := range tests {
			pointer, limits := runCreateSegment(ctx, t, metainfo)
			test.Modify(pointer)

			_, err = metainfo.CommitSegment(ctx, "my-bucket-name", "file/path", -1, pointer, limits)
			require.Error(t, err)
			require.Contains(t, err.Error(), test.ErrorMessage)
		}
	})
}

func TestSetAttribution(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		config := uplink.GetConfig(planet.Satellites[0])
		metainfo, _, cleanup, err := testplanet.DialMetainfo(ctx, uplink.Log.Named("metainfo"), config, uplink.Identity)
		require.NoError(t, err)
		defer ctx.Check(cleanup)

		_, err = metainfo.CreateBucket(ctx, "alpha", &storj.Bucket{PathCipher: config.GetEncryptionScheme().Cipher})
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		partnerID := testrand.UUID()
		{
			// bucket with no items
			err = metainfoClient.SetAttribution(ctx, "alpha", partnerID)
			require.NoError(t, err)

			// no bucket exists
			err = metainfoClient.SetAttribution(ctx, "beta", partnerID)
			require.NoError(t, err)
		}
		{
			// already attributed bucket, adding files
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "alpha", "path", []byte{1, 2, 3})
			assert.NoError(t, err)

			// bucket with items
			err = metainfoClient.SetAttribution(ctx, "alpha", partnerID)
			require.NoError(t, err)
		}
		{
			//non attributed bucket, and adding files
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "alphaNew", "path", []byte{1, 2, 3})
			assert.NoError(t, err)

			// bucket with items
			err = metainfoClient.SetAttribution(ctx, "alphaNew", partnerID)
			require.Error(t, err)
		}
	})
}

func TestGetProjectInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 2,
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

func runCreateSegment(ctx context.Context, t *testing.T, metainfo *metainfo.Client) (*pb.Pointer, []*pb.OrderLimit) {
	pointer := createTestPointer(t)
	expirationDate, err := ptypes.Timestamp(pointer.ExpirationDate)
	require.NoError(t, err)

	addressedLimits, rootPieceID, err := metainfo.CreateSegment(ctx, "my-bucket-name", "file/path", -1, pointer.Remote.Redundancy, memory.MiB.Int64(), expirationDate)
	require.NoError(t, err)

	pointer.Remote.RootPieceId = rootPieceID
	pointer.Remote.RemotePieces[0].NodeId = addressedLimits[0].Limit.StorageNodeId
	pointer.Remote.RemotePieces[1].NodeId = addressedLimits[1].Limit.StorageNodeId

	limits := make([]*pb.OrderLimit, len(addressedLimits))
	for i, addressedLimit := range addressedLimits {
		limits[i] = addressedLimit.Limit
	}

	return pointer, limits
}

func createTestPointer(t *testing.T) *pb.Pointer {
	rs := &pb.RedundancyScheme{
		MinReq:           1,
		RepairThreshold:  1,
		SuccessThreshold: 3,
		Total:            4,
		ErasureShareSize: 1024,
		Type:             pb.RedundancyScheme_RS,
	}

	pointer := &pb.Pointer{
		Type: pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
			Redundancy: rs,
			RemotePieces: []*pb.RemotePiece{
				{
					PieceNum: 0,
				},
				{
					PieceNum: 1,
				},
			},
		},
		ExpirationDate: ptypes.TimestampNow(),
	}
	return pointer
}

func TestBucketNameValidation(t *testing.T) {
	if !satMetainfo.BucketNameRestricted {
		t.Skip("Skip until bucket name validation is not enabled")
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

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
			_, _, err = metainfo.CreateSegment(ctx, name, "", -1, rs, 1, time.Now())
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
			_, _, err = metainfo.CreateSegment(ctx, name, "", -1, rs, 1, time.Now())
			require.Error(t, err, "bucket name: %v", name)
		}
	})
}
