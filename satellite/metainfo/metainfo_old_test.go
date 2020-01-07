// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"testing"
	"time"

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
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/metainfo"
)

func TestInvalidAPIKeyOld(t *testing.T) {
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

func TestExpirationTimeSegmentOld(t *testing.T) {
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
