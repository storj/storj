// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
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
	"storj.io/storj/uplink"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/metainfo"
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

			_, _, _, err = client.CreateSegmentOld(ctx, "hello", "world", 1, &pb.RedundancyScheme{}, 123, time.Now().Add(time.Hour))
			assertUnauthenticated(t, err, false)

			_, err = client.CommitSegmentOld(ctx, "testbucket", "testpath", 0, &pb.Pointer{}, nil)
			assertUnauthenticated(t, err, false)

			_, err = client.SegmentInfoOld(ctx, "testbucket", "testpath", 0)
			assertUnauthenticated(t, err, false)

			_, _, _, err = client.ReadSegmentOld(ctx, "testbucket", "testpath", 0)
			assertUnauthenticated(t, err, false)

			_, _, err = client.DeleteSegmentOld(ctx, "testbucket", "testpath", 0)
			assertUnauthenticated(t, err, false)

			_, _, err = client.ListSegmentsOld(ctx, "testbucket", "", "", "", true, 1, 0)
			assertUnauthenticated(t, err, false)
		}
	})
}

func TestRestrictedAPIKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		key := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

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

			client, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], restrictedKey)
			require.NoError(t, err)
			defer ctx.Check(client.Close)

			_, _, _, err = client.CreateSegmentOld(ctx, "testbucket", "testpath", 1, &pb.RedundancyScheme{}, 123, time.Now().Add(time.Hour))
			assertUnauthenticated(t, err, test.CreateSegmentAllowed)

			_, err = client.CommitSegmentOld(ctx, "testbucket", "testpath", 0, &pb.Pointer{}, nil)
			assertUnauthenticated(t, err, test.CommitSegmentAllowed)

			_, err = client.SegmentInfoOld(ctx, "testbucket", "testpath", 0)
			assertUnauthenticated(t, err, test.SegmentInfoAllowed)

			_, _, _, err = client.ReadSegmentOld(ctx, "testbucket", "testpath", 0)
			assertUnauthenticated(t, err, test.ReadSegmentAllowed)

			_, _, err = client.DeleteSegmentOld(ctx, "testbucket", "testpath", 0)
			assertUnauthenticated(t, err, test.DeleteSegmentAllowed)

			_, _, err = client.ListSegmentsOld(ctx, "testbucket", "testpath", "", "", true, 1, 0)
			assertUnauthenticated(t, err, test.ListSegmentsAllowed)

			_, _, _, err = client.ReadSegmentOld(ctx, "testbucket", "", -1)
			assertUnauthenticated(t, err, test.ReadBucketAllowed)
		}
	})
}

func assertUnauthenticated(t *testing.T, err error, allowed bool) {
	t.Helper()

	// If it's allowed, we allow any non-unauthenticated error because
	// some calls error after authentication checks.
	if !allowed {
		assert.True(t, errs2.IsRPC(err, rpcstatus.Unauthenticated))
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

func TestCommitSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		fullIDMap := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			fullIDMap[node.ID()] = node.Identity
		}

		{
			// error if pointer is nil
			_, err = metainfo.CommitSegmentOld(ctx, "bucket", "path", -1, nil, []*pb.OrderLimit{})
			require.Error(t, err)
		}
		{
			// error if number of remote pieces is lower than repair threshold
			redundancy := &pb.RedundancyScheme{
				MinReq:           1,
				RepairThreshold:  2,
				SuccessThreshold: 3,
				Total:            4,
				ErasureShareSize: 256,
			}
			expirationDate := time.Now().Add(time.Hour)
			addressedLimits, rootPieceID, _, err := metainfo.CreateSegmentOld(ctx, "bucket", "path", -1, redundancy, 1000, expirationDate)
			require.NoError(t, err)

			// create number of pieces below repair threshold
			usedForPieces := addressedLimits[:redundancy.RepairThreshold-1]
			pieces := make([]*pb.RemotePiece, len(usedForPieces))
			for i, limit := range usedForPieces {
				newPiece := &pb.RemotePiece{
					PieceNum: int32(i),
					NodeId:   limit.Limit.StorageNodeId,
					Hash: &pb.PieceHash{
						PieceId:   limit.Limit.PieceId,
						PieceSize: 256,
						Timestamp: time.Now(),
					},
				}

				fullID := fullIDMap[limit.Limit.StorageNodeId]
				require.NotNil(t, fullID)
				signer := signing.SignerFromFullIdentity(fullID)
				newHash, err := signing.SignPieceHash(ctx, signer, newPiece.Hash)
				require.NoError(t, err)

				newPiece.Hash = newHash

				pieces[i] = newPiece
			}

			pointer := &pb.Pointer{
				CreationDate: time.Now(),
				Type:         pb.Pointer_REMOTE,
				SegmentSize:  10,
				Remote: &pb.RemoteSegment{
					RootPieceId:  rootPieceID,
					Redundancy:   redundancy,
					RemotePieces: pieces,
				},
				ExpirationDate: expirationDate,
			}

			limits := make([]*pb.OrderLimit, len(addressedLimits))
			for i, addressedLimit := range addressedLimits {
				limits[i] = addressedLimit.Limit
			}
			_, err = metainfo.CommitSegmentOld(ctx, "bucket", "path", -1, pointer, limits)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
			require.Contains(t, err.Error(), "is less than or equal to the repair threshold")
		}

		{
			// error if number of remote pieces is lower than success threshold
			redundancy := &pb.RedundancyScheme{
				MinReq:           1,
				RepairThreshold:  2,
				SuccessThreshold: 5,
				Total:            6,
				ErasureShareSize: 256,
			}
			expirationDate := time.Now().Add(time.Hour)
			addressedLimits, rootPieceID, _, err := metainfo.CreateSegmentOld(ctx, "bucket", "path", -1, redundancy, 1000, expirationDate)
			require.NoError(t, err)

			// create number of pieces below success threshold
			usedForPieces := addressedLimits[:redundancy.SuccessThreshold-1]
			pieces := make([]*pb.RemotePiece, len(usedForPieces))
			for i, limit := range usedForPieces {
				newPiece := &pb.RemotePiece{
					PieceNum: int32(i),
					NodeId:   limit.Limit.StorageNodeId,
					Hash: &pb.PieceHash{
						PieceId:   limit.Limit.PieceId,
						PieceSize: 256,
						Timestamp: time.Now(),
					},
				}

				fullID := fullIDMap[limit.Limit.StorageNodeId]
				require.NotNil(t, fullID)
				signer := signing.SignerFromFullIdentity(fullID)
				newHash, err := signing.SignPieceHash(ctx, signer, newPiece.Hash)
				require.NoError(t, err)

				newPiece.Hash = newHash

				pieces[i] = newPiece
			}

			pointer := &pb.Pointer{
				CreationDate: time.Now(),
				Type:         pb.Pointer_REMOTE,
				SegmentSize:  10,
				Remote: &pb.RemoteSegment{
					RootPieceId:  rootPieceID,
					Redundancy:   redundancy,
					RemotePieces: pieces,
				},
				ExpirationDate: expirationDate,
			}

			limits := make([]*pb.OrderLimit, len(addressedLimits))
			for i, addressedLimit := range addressedLimits {
				limits[i] = addressedLimit.Limit
			}
			_, err = metainfo.CommitSegmentOld(ctx, "bucket", "path", -1, pointer, limits)
			require.Error(t, err)
			require.Contains(t, err.Error(), "is less than the success threshold")
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
			_, _, _, err := metainfo.CreateSegmentOld(ctx, "bucket", "path", -1, r.rs, 1000, time.Now().Add(time.Hour))
			if r.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	})
}

func TestExpirationTimeSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)
		rs := &pb.RedundancyScheme{
			MinReq:           1,
			RepairThreshold:  1,
			SuccessThreshold: 1,
			Total:            1,
			ErasureShareSize: 1024,
			Type:             pb.RedundancyScheme_RS,
		}

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

			_, _, _, err := metainfo.CreateSegmentOld(ctx, "my-bucket-name", "file/path", -1, rs, memory.MiB.Int64(), r.expirationDate)
			if err != nil {
				assert.True(t, r.errFlag)
			} else {
				assert.False(t, r.errFlag)
			}
		}
	})
}

func TestMaxCommitInterval(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.MaxCommitInterval = -1 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		fullIDMap := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			fullIDMap[node.ID()] = node.Identity
		}

		pointer, limits := runCreateSegment(ctx, t, metainfo, fullIDMap)

		_, err = metainfo.CommitSegmentOld(ctx, "my-bucket-name", "file/path", -1, pointer, limits)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not committed before max commit interval")
	})
}

func TestDoubleCommitSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		fullIDMap := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			fullIDMap[node.ID()] = node.Identity
		}

		pointer, limits := runCreateSegment(ctx, t, metainfo, fullIDMap)

		savedPointer, err := metainfo.CommitSegmentOld(ctx, "my-bucket-name", "file/path", -1, pointer, limits)
		require.NoError(t, err)
		require.True(t, savedPointer.PieceHashesVerified)

		_, err = metainfo.CommitSegmentOld(ctx, "my-bucket-name", "file/path", -1, pointer, limits)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing create request or request expired")
	})
}

func TestCommitSegmentPointer(t *testing.T) {
	// all tests needs to generate error
	tests := []struct {
		// defines how modify pointer before CommitSegment
		Modify       func(ctx context.Context, pointer *pb.Pointer, fullIDMap map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit)
		ErrorMessage string
	}{
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.ExpirationDate = pointer.ExpirationDate.Add(time.Second * 100)
			},
			ErrorMessage: "pointer expiration date does not match requested one",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.Redundancy.MinReq += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.Redundancy.RepairThreshold += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.Redundancy.SuccessThreshold += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.Redundancy.Total += 100
			},
			// this error is triggered earlier then Create/Commit RS comparison
			ErrorMessage: "invalid no order limit for piece",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.Redundancy.ErasureShareSize += 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.Redundancy.Type = 100
			},
			ErrorMessage: "pointer redundancy scheme date does not match requested one",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Type = pb.Pointer_INLINE
			},
			ErrorMessage: "pointer type is INLINE but remote segment is set",
		},
		{
			// no piece hash removes piece from pointer, not enough pieces for successful upload
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.RemotePieces[0].Hash = nil
			},
			ErrorMessage: "Number of valid pieces (2) is less than the success threshold (3)",
		},
		{
			// set piece number to be out of range of limit slice
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.RemotePieces[0].PieceNum = int32(len(limits))
			},
			ErrorMessage: "invalid piece number",
		},
		{
			// invalid timestamp removes piece from pointer, not enough pieces for successful upload
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.RemotePieces[0].Hash.Timestamp = time.Now().Add(-24 * time.Hour)
			},
			ErrorMessage: "Number of valid pieces (2) is less than the success threshold (3)",
		},
		{
			// invalid hash PieceID removes piece from pointer, not enough pieces for successful upload
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.RemotePieces[0].Hash.PieceId = storj.PieceID{1}
			},
			ErrorMessage: "Number of valid pieces (2) is less than the success threshold (3)",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, fullIDMap map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.Remote.RemotePieces[0].Hash.PieceSize = 1

				snFullID := fullIDMap[pointer.Remote.RemotePieces[0].NodeId]
				require.NotNil(t, snFullID)
				signer := signing.SignerFromFullIdentity(snFullID)
				storageNodeHash, err := signing.SignPieceHash(ctx, signer, pointer.Remote.RemotePieces[0].Hash)
				require.NoError(t, err)
				pointer.Remote.RemotePieces[0].Hash = storageNodeHash
			},
			ErrorMessage: "all pieces needs to have the same size",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				pointer.SegmentSize = 100
			},
			ErrorMessage: "expected piece size is different from provided",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				// nil piece hash signature removes piece from pointer, not enough pieces for successful upload
				pointer.Remote.RemotePieces[0].Hash.Signature = nil
			},
			ErrorMessage: "Number of valid pieces (2) is less than the success threshold (3)",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				// invalid piece hash signature removes piece from pointer, not enough pieces for successful upload
				pointer.Remote.RemotePieces[0].Hash.Signature = nil

				ca, err := testidentity.NewTestCA(ctx)
				require.NoError(t, err)
				badFullID, err := ca.NewIdentity()
				require.NoError(t, err)
				signer := signing.SignerFromFullIdentity(badFullID)

				newHash, err := signing.SignPieceHash(ctx, signer, pointer.Remote.RemotePieces[0].Hash)
				require.NoError(t, err)
				pointer.Remote.RemotePieces[0].Hash = newHash
			},
			ErrorMessage: "Number of valid pieces (2) is less than the success threshold (3)",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				firstPiece := pointer.Remote.RemotePieces[0]
				pointer.Remote.RemotePieces[1] = firstPiece
				pointer.Remote.RemotePieces[2] = firstPiece
			},
			ErrorMessage: "piece num 0 is duplicated",
		},
		{
			Modify: func(ctx context.Context, pointer *pb.Pointer, _ map[storj.NodeID]*identity.FullIdentity, limits []*pb.OrderLimit) {
				firstNodeID := pointer.Remote.RemotePieces[0].NodeId
				pointer.Remote.RemotePieces[1].NodeId = firstNodeID
			},
			ErrorMessage: "invalid order limit piece id",
		},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfo, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		fullIDMap := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			fullIDMap[node.ID()] = node.Identity
		}

		for i, test := range tests {
			pointer, limits := runCreateSegment(ctx, t, metainfo, fullIDMap)
			test.Modify(ctx, pointer, fullIDMap, limits)

			_, err = metainfo.CommitSegmentOld(ctx, "my-bucket-name", "file/path", -1, pointer, limits)
			require.Error(t, err, "Case #%v", i)
			require.Contains(t, err.Error(), test.ErrorMessage, "Case #%v", i)
		}
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

func runCreateSegment(ctx context.Context, t *testing.T, metainfo *metainfo.Client, fullIDMap map[storj.NodeID]*identity.FullIdentity) (*pb.Pointer, []*pb.OrderLimit) {
	pointer := createTestPointer(t)

	addressedLimits, rootPieceID, _, err := metainfo.CreateSegmentOld(ctx, "my-bucket-name", "file/path", -1, pointer.Remote.Redundancy, memory.MiB.Int64(), pointer.ExpirationDate)
	require.NoError(t, err)

	pointer.Remote.RootPieceId = rootPieceID

	limits := make([]*pb.OrderLimit, len(addressedLimits))
	for i, addressedLimit := range addressedLimits {
		limits[i] = addressedLimit.Limit

		if len(pointer.Remote.RemotePieces) > i {
			nodeID := addressedLimits[i].Limit.StorageNodeId
			pointer.Remote.RemotePieces[i].NodeId = nodeID
			pointer.Remote.RemotePieces[i].Hash.PieceId = addressedLimits[i].Limit.PieceId

			snFullID := fullIDMap[nodeID]
			require.NotNil(t, snFullID)
			signer := signing.SignerFromFullIdentity(snFullID)
			storageNodeHash, err := signing.SignPieceHash(ctx, signer, pointer.Remote.RemotePieces[i].Hash)
			require.NoError(t, err)
			pointer.Remote.RemotePieces[i].Hash = storageNodeHash
		}
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

	redundancy, err := eestream.NewRedundancyStrategyFromProto(rs)
	require.NoError(t, err)
	segmentSize := 4 * memory.KiB.Int64()
	pieceSize := eestream.CalcPieceSize(segmentSize, redundancy)
	timestamp := time.Now().Add(time.Hour)
	pointer := &pb.Pointer{
		CreationDate: time.Now(),
		Type:         pb.Pointer_REMOTE,
		SegmentSize:  segmentSize,
		Remote: &pb.RemoteSegment{
			Redundancy: rs,
			RemotePieces: []*pb.RemotePiece{
				{
					PieceNum: 0,
					Hash: &pb.PieceHash{
						PieceSize: pieceSize,
						Timestamp: timestamp,
					},
				},
				{
					PieceNum: 1,
					Hash: &pb.PieceHash{
						PieceSize: pieceSize,
						Timestamp: timestamp,
					},
				},
				{
					PieceNum: 2,
					Hash: &pb.PieceHash{
						PieceSize: pieceSize,
						Timestamp: timestamp,
					},
				},
			},
		},
		ExpirationDate: timestamp,
	}
	return pointer
}

func TestBucketNameValidation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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
			_, _, _, err = metainfo.CreateSegmentOld(ctx, name, "", -1, rs, 1, time.Now().Add(time.Hour))
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
			_, _, _, err = metainfo.CreateSegmentOld(ctx, name, "", -1, rs, 1, time.Now().Add(time.Hour))
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
			ExpiresAt:            time.Now().UTC().Add(24 * time.Hour),
		}
		streamID, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		segmentID, limits, _, err := metainfoClient.BeginSegment(ctx, metainfo.BeginSegmentParams{
			StreamID: streamID,
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

		metadata, err := proto.Marshal(&pb.StreamMeta{
			NumberOfSegments: 1,
		})
		require.NoError(t, err)
		err = metainfoClient.CommitObject(ctx, metainfo.CommitObjectParams{
			StreamID:          streamID,
			EncryptedMetadata: metadata,
		})
		require.NoError(t, err)

		objects, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte(bucket.Name),
		})
		require.NoError(t, err)
		require.Len(t, objects, 1)

		require.Equal(t, params.EncryptedPath, objects[0].EncryptedPath)
		require.Equal(t, params.ExpiresAt, objects[0].ExpiresAt)

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
			ExpiresAt:            time.Now().UTC().Add(24 * time.Hour),
		}
		streamID, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		segments := []int32{0, 1, 2, 3, 4, 5, 6}
		segmentsData := make([][]byte, len(segments))
		for i, segment := range segments {
			segmentsData[i] = testrand.Bytes(memory.KiB)
			err = metainfoClient.MakeInlineSegment(ctx, metainfo.MakeInlineSegmentParams{
				StreamID: streamID,
				Position: storj.SegmentPosition{
					Index: segment,
				},
				EncryptedInlineData: segmentsData[i],
			})
			require.NoError(t, err)
		}

		metadata, err := proto.Marshal(&pb.StreamMeta{
			NumberOfSegments: int64(len(segments)),
		})
		require.NoError(t, err)
		err = metainfoClient.CommitObject(ctx, metainfo.CommitObjectParams{
			StreamID:          streamID,
			EncryptedMetadata: metadata,
		})
		require.NoError(t, err)

		objects, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte(bucket.Name),
		})
		require.NoError(t, err)
		require.Len(t, objects, 1)

		require.Equal(t, params.EncryptedPath, objects[0].EncryptedPath)
		require.Equal(t, params.ExpiresAt, objects[0].ExpiresAt)

		object, err := metainfoClient.GetObject(ctx, metainfo.GetObjectParams{
			Bucket:        params.Bucket,
			EncryptedPath: params.EncryptedPath,
		})
		require.NoError(t, err)

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
			streamID, err = metainfoClient.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{
				Bucket:        params.Bucket,
				EncryptedPath: params.EncryptedPath,
			})
			require.NoError(t, err)

			items, _, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: streamID,
			})
			require.NoError(t, err)
			for _, item := range items {
				segmentID, limits, _, err := metainfoClient.BeginDeleteSegment(ctx, metainfo.BeginDeleteSegmentParams{
					StreamID: streamID,
					Position: storj.SegmentPosition{
						Index: item.Position.Index,
					},
				})
				require.NoError(t, err)
				require.Nil(t, limits)

				err = metainfoClient.FinishDeleteSegment(ctx, metainfo.FinishDeleteSegmentParams{
					SegmentID: segmentID,
				})
				require.NoError(t, err)
			}

			_, _, err = metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: streamID,
			})
			require.Error(t, err)
			require.True(t, storj.ErrObjectNotFound.Has(err))

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

			streamID, err := metainfoClient.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{
				Bucket:        []byte(expectedBucketName),
				EncryptedPath: items[0].EncryptedPath,
			})
			require.NoError(t, err)

			segments, _, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
				StreamID: streamID,
			})
			require.NoError(t, err)

			for _, segment := range segments {
				segmentID, limits, _, err := metainfoClient.BeginDeleteSegment(ctx, metainfo.BeginDeleteSegmentParams{
					StreamID: streamID,
					Position: storj.SegmentPosition{
						Index: segment.Position.Index,
					},
				})
				require.NoError(t, err)
				require.NotEmpty(t, limits)

				err = metainfoClient.FinishDeleteSegment(ctx, metainfo.FinishDeleteSegmentParams{
					SegmentID: segmentID,
				})
				require.NoError(t, err)
			}

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

			encodedStreamID, err := proto.Marshal(signedStreamID)
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

			encodedSegmentID, err := proto.Marshal(signedSegmentID)
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

			encodedSegmentID, err := proto.Marshal(signedSegmentID)
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

			metadata, err := proto.Marshal(&pb.StreamMeta{
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

			streamID, err := metainfoClient.BeginObject(ctx, metainfo.BeginObjectParams{
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
					StreamID: streamID,
					Position: storj.SegmentPosition{
						Index: int32(i),
					},
					EncryptedInlineData: expectedData[i],
				})
			}

			metadata, err := proto.Marshal(&pb.StreamMeta{
				NumberOfSegments: int64(numOfSegments),
			})
			require.NoError(t, err)
			requests = append(requests, &metainfo.CommitObjectParams{
				StreamID:          streamID,
				EncryptedMetadata: metadata,
			})

			responses, err := metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfSegments+1, len(responses))
		}
	})
}

func TestValidateRS(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.RS.MinTotalThreshold = 4
				config.Metainfo.RS.MaxTotalThreshold = 5
				config.Metainfo.RS.Validate = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		testData := testrand.Bytes(8 * memory.KiB)
		rs := &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 3,
			MaxThreshold:     3,
		}
		// test below permitted total value
		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path/below", testData)
		require.Error(t, err)

		// test above permitted total value
		rs.MaxThreshold = 6
		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path/above", testData)
		require.Error(t, err)

		// test minimum permitted total value
		rs.MaxThreshold = 4
		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path/min", testData)
		require.NoError(t, err)

		// test maximum permitted total value
		rs.MaxThreshold = 5
		err = ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path/max", testData)
		require.NoError(t, err)
	})
}
