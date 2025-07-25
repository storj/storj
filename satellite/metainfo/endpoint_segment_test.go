// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/rpc/rpctest"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/piecestore"
)

func TestExpirationTimeSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketName := testrand.BucketName()
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
		metainfoClient := createMetainfoClient(ctx, t, planet)

		for i, r := range []struct {
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
			_, err := metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("path" + strconv.Itoa(i)),
				ExpiresAt:          r.expirationDate,
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: storj.EncAESGCM,
					BlockSize:   256,
				},
			})
			if r.errFlag {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	})
}

func TestInlineSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
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

		bucketName := testrand.BucketName()
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
		metainfoClient := createMetainfoClient(ctx, t, planet)

		params := metaclient.BeginObjectParams{
			Bucket:             []byte(bucketName),
			EncryptedObjectKey: []byte("encrypted-path"),
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      256,
				RequiredShares: 1,
				RepairShares:   1,
				OptimalShares:  3,
				TotalShares:    4,
			},
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   256,
			},

			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		beginObjectResp, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		segments := []int32{0, 1, 2, 3, 4, 5, 6}
		segmentsData := make([][]byte, len(segments))
		for i, segment := range segments {
			segmentsData[i] = testrand.Bytes(memory.KiB)
			err = metainfoClient.MakeInlineSegment(ctx, metaclient.MakeInlineSegmentParams{
				StreamID: beginObjectResp.StreamID,
				Position: metaclient.SegmentPosition{
					Index: segment,
				},
				PlainSize:           1024,
				EncryptedInlineData: segmentsData[i],
				Encryption: metaclient.SegmentEncryption{
					EncryptedKey: testrand.Bytes(256),
				},
			})
			require.NoError(t, err)
		}

		metadata, err := pb.Marshal(&pb.StreamMeta{
			NumberOfSegments: int64(len(segments)),
		})
		require.NoError(t, err)
		err = metainfoClient.CommitObject(ctx, metaclient.CommitObjectParams{
			StreamID: beginObjectResp.StreamID,
			EncryptedUserData: metaclient.EncryptedUserData{
				EncryptedMetadata:             metadata,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: randomEncryptedKey,
			},
		})
		require.NoError(t, err)

		objects, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
			Bucket:                []byte(bucketName),
			IncludeSystemMetadata: true,
		})
		require.NoError(t, err)
		require.Len(t, objects, 1)

		require.Equal(t, params.EncryptedObjectKey, objects[0].EncryptedObjectKey)
		// TODO find better way to compare (one ExpiresAt contains time zone information)
		require.Equal(t, params.ExpiresAt.Unix(), objects[0].ExpiresAt.Unix())

		object, err := metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
			Bucket:             params.Bucket,
			EncryptedObjectKey: params.EncryptedObjectKey,
		})
		require.NoError(t, err)

		{ // Confirm data larger than our configured max inline segment size of 4 KiB cannot be inlined
			beginObjectResp, err := metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("too-large-inline-segment"),
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: storj.EncAESGCM,
					BlockSize:   256,
				},
			})
			require.NoError(t, err)

			data := testrand.Bytes(10 * memory.KiB)
			err = metainfoClient.MakeInlineSegment(ctx, metaclient.MakeInlineSegmentParams{
				StreamID: beginObjectResp.StreamID,
				Position: metaclient.SegmentPosition{
					Index: 0,
				},
				EncryptedInlineData: data,
				Encryption: metaclient.SegmentEncryption{
					EncryptedKey: testrand.Bytes(256),
				},
			})
			require.Error(t, err)
		}

		{ // test download inline segments
			existingSegments := []int32{0, 1, 2, 3, 4, 5, -1}

			for i, index := range existingSegments {
				response, err := metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
					StreamID: object.StreamID,
					Position: metaclient.SegmentPosition{
						Index: index,
					},
				})
				require.NoError(t, err)
				require.Nil(t, response.Limits)
				require.Equal(t, segmentsData[i], response.Info.EncryptedInlineData)
			}
		}

		{ // test deleting segments
			_, err := metainfoClient.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{
				Bucket:             params.Bucket,
				EncryptedObjectKey: params.EncryptedObjectKey,
			})
			require.NoError(t, err)

			_, err = metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             params.Bucket,
				EncryptedObjectKey: params.EncryptedObjectKey,
			})
			require.Error(t, err)
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
		err := uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "file-object", testrand.Bytes(50*memory.KiB))
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		items, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
			Bucket: []byte(expectedBucketName),
		})
		require.NoError(t, err)
		require.Len(t, items, 1)

		{
			// Get object
			// Download segment

			object, err := metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: items[0].EncryptedObjectKey,
			})
			require.NoError(t, err)

			response, err := metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: metaclient.SegmentPosition{
					Index: -1,
				},
			})
			require.NoError(t, err)
			require.NotEmpty(t, response.Limits)
		}

		{
			// Download Object
			download, err := metainfoClient.DownloadObject(ctx, metaclient.DownloadObjectParams{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: items[0].EncryptedObjectKey,
				Range: metaclient.StreamRange{
					Mode:  metaclient.StreamRangeStartLimit,
					Start: 1,
					Limit: 2,
				},
			})
			require.NoError(t, err)
			require.Len(t, download.DownloadedSegments, 1)
			require.NotEmpty(t, download.DownloadedSegments[0].Limits)
			for _, limit := range download.DownloadedSegments[0].Limits {
				if limit == nil {
					continue
				}
				// requested download size is
				//      [1:2}
				// calculating encryption input block size (7408) indices gives us:
				//      0 and 1
				// converting these into output block size (7424), gives us:
				//      [0:7424}
				// this aligned to stripe size (256), gives us:
				//      [0:7424}
				require.Equal(t, int64(7424), limit.Limit.Limit)
			}
		}

		{
			// Begin deleting object
			// List objects

			_, err := metainfoClient.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: items[0].EncryptedObjectKey,
			})
			require.NoError(t, err)

			items, _, err = metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket: []byte(expectedBucketName),
			})
			require.NoError(t, err)
			require.Len(t, items, 0)
		}
	})
}

func TestInlineSegmentThreshold(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		{ // limit is max inline segment size + encryption overhead
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket-inline", "inline-object", testrand.Bytes(4*memory.KiB))
			require.NoError(t, err)

			// we don't know encrypted path
			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 1)
			require.Zero(t, segments[0].Redundancy)
			require.NotEmpty(t, segments[0].InlineData)

			// clean up - delete the uploaded object
			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)
			_, err = planet.Satellites[0].Metabase.DB.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
				ObjectLocation: objects[0].Location(),
				Version:        metabase.DefaultVersion,
			})
			require.NoError(t, err)
		}

		{ // one more byte over limit should enough to create remote segment
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket-remote", "remote-object", testrand.Bytes(4*memory.KiB+1))
			require.NoError(t, err)

			// we don't know encrypted path
			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 1)
			require.NotZero(t, segments[0].Redundancy)
			require.Empty(t, segments[0].InlineData)

			// clean up - delete the uploaded object
			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)
			_, err = planet.Satellites[0].Metabase.DB.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
				ObjectLocation: objects[0].Location(),
				Version:        metabase.DefaultVersion,
			})
			require.NoError(t, err)
		}
	})
}

func TestObjectSegmentExpiresAt(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		inlineData := testrand.Bytes(1 * memory.KiB)
		inlineExpiration := time.Now().Add(2 * time.Hour)
		err := planet.Uplinks[0].UploadWithExpiration(ctx, planet.Satellites[0], "hohoho", "inline_object", inlineData, inlineExpiration)
		require.NoError(t, err)

		remoteData := testrand.Bytes(10 * memory.KiB)
		remoteExpiration := time.Now().Add(4 * time.Hour)
		err = planet.Uplinks[0].UploadWithExpiration(ctx, planet.Satellites[0], "hohoho", "remote_object", remoteData, remoteExpiration)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 2)

		for _, segment := range segments {
			if int(segment.PlainSize) == len(inlineData) {
				require.Equal(t, inlineExpiration.Unix(), segment.ExpiresAt.Unix())
			} else if int(segment.PlainSize) == len(remoteData) {
				require.Equal(t, remoteExpiration.Unix(), segment.ExpiresAt.Unix())
			} else {
				t.Fail()
			}
		}
	})
}

func TestCommitSegment_Validation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(1, 1, 1, 1),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketName := testrand.BucketName()
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
		client := createMetainfoClient(ctx, t, planet)

		beginObjectResponse, err := client.BeginObject(ctx, metaclient.BeginObjectParams{
			Bucket:             []byte(bucketName),
			EncryptedObjectKey: []byte("a/b/testobject"),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   256,
			},
		})
		require.NoError(t, err)

		response, err := client.BeginSegment(ctx, metaclient.BeginSegmentParams{
			StreamID: beginObjectResponse.StreamID,
			Position: metaclient.SegmentPosition{
				Index: 0,
			},
			MaxOrderLimit: memory.MiB.Int64(),
		})
		require.NoError(t, err)

		// the number of results of uploaded pieces (0) is below the optimal threshold (1)
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID:    response.SegmentID,
			UploadResult: []*pb.SegmentPieceUploadResult{},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// piece sizes are invalid: pointer verification: no remote pieces
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID: response.SegmentID,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// piece sizes are invalid: pointer verification: size is invalid (-1)
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID: response.SegmentID,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{
					Hash: &pb.PieceHash{
						PieceSize: -1,
					},
				},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// piece sizes are invalid: pointer verification: expected size is different from provided (768 != 10000)
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID:         response.SegmentID,
			SizeEncryptedData: 512,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{
					Hash: &pb.PieceHash{
						PieceSize: 10000,
					},
				},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// piece sizes are invalid: pointer verification: sizes do not match (10000 != 9000)
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID: response.SegmentID,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{
					Hash: &pb.PieceHash{
						PieceSize: 10000,
					},
				},
				{
					Hash: &pb.PieceHash{
						PieceSize: 9000,
					},
				},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// pointer verification failed: pointer verification: invalid piece number
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID:         response.SegmentID,
			SizeEncryptedData: 512,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{
					PieceNum: 10,
					NodeId:   response.Limits[0].Limit.StorageNodeId,
					Hash: &pb.PieceHash{
						PieceSize: 768,
						PieceId:   response.Limits[0].Limit.PieceId,
						Timestamp: time.Now(),
					},
				},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// signature verification error (no signature)
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID:         response.SegmentID,
			SizeEncryptedData: 512,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{
					PieceNum: 0,
					NodeId:   response.Limits[0].Limit.StorageNodeId,
					Hash: &pb.PieceHash{
						PieceSize: 768,
						PieceId:   response.Limits[0].Limit.PieceId,
						Timestamp: time.Now(),
					},
				},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		signer := signing.SignerFromFullIdentity(planet.StorageNodes[0].Identity)
		signedHash, err := signing.SignPieceHash(ctx, signer, &pb.PieceHash{
			PieceSize: 768,
			PieceId:   response.Limits[0].Limit.PieceId,
			Timestamp: time.Now(),
		})
		require.NoError(t, err)

		// pointer verification failed: pointer verification: nil identity returned
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID:         response.SegmentID,
			SizeEncryptedData: 512,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{
					PieceNum: 0,
					NodeId:   testrand.NodeID(), // random node ID
					Hash:     signedHash,
				},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// plain segment size 513 is out of range, maximum allowed is 512
		err = client.CommitSegment(ctx, metaclient.CommitSegmentParams{
			SegmentID:         response.SegmentID,
			PlainSize:         513,
			SizeEncryptedData: 512,
			UploadResult: []*pb.SegmentPieceUploadResult{
				{
					PieceNum: 0,
					NodeId:   response.Limits[0].Limit.StorageNodeId,
					Hash:     signedHash,
				},
			},
		})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
	})
}

func TestRetryBeginSegmentPieces(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketName := testrand.BucketName()
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
		metainfoClient := createMetainfoClient(ctx, t, planet)

		params := metaclient.BeginObjectParams{
			Bucket:             []byte(bucketName),
			EncryptedObjectKey: []byte("encrypted-path"),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   256,
			},
		}

		beginObjectResp, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		beginSegmentResp, err := metainfoClient.BeginSegment(ctx, metaclient.BeginSegmentParams{
			StreamID:      beginObjectResp.StreamID,
			Position:      metaclient.SegmentPosition{},
			MaxOrderLimit: 1024,
		})
		require.NoError(t, err)

		// This call should fail, since there will not be enough unique nodes
		// available to replace all of the piece orders.
		_, err = metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
			SegmentID:         beginSegmentResp.SegmentID,
			RetryPieceNumbers: []int{0, 1, 2, 3, 4, 5, 6},
		})
		rpctest.RequireStatus(t, err, rpcstatus.FailedPrecondition, "metaclient: not enough nodes: not enough nodes: requested from cache 7, found 2")

		// This exchange should succeed.
		exchangeSegmentPieceOrdersResp, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
			SegmentID:         beginSegmentResp.SegmentID,
			RetryPieceNumbers: []int{0, 2},
		})
		require.NoError(t, err)

		makeResult := func(i int) *pb.SegmentPieceUploadResult {
			limit := exchangeSegmentPieceOrdersResp.Limits[i].Limit
			node := planet.FindNode(limit.StorageNodeId)
			require.NotNil(t, node, "failed to locate node to sign hash for piece %d", i)
			signer := signing.SignerFromFullIdentity(node.Identity)
			hash, err := signing.SignPieceHash(ctx, signer, &pb.PieceHash{
				PieceSize: 512,
				PieceId:   limit.PieceId,
				Timestamp: time.Now(),
			})
			require.NoError(t, err)
			return &pb.SegmentPieceUploadResult{
				PieceNum: int32(i),
				NodeId:   limit.StorageNodeId,
				Hash:     hash,
			}
		}

		// Commit with only 6 successful uploads, otherwise, the original
		// limits will still be valid. We want to test that the exchange
		// replaced the order limits.
		commitSegmentParams := metaclient.CommitSegmentParams{
			SegmentID:         beginSegmentResp.SegmentID,
			PlainSize:         512,
			SizeEncryptedData: 512,
			Encryption: metaclient.SegmentEncryption{
				EncryptedKey: testrand.Bytes(256),
			},
			UploadResult: []*pb.SegmentPieceUploadResult{
				makeResult(0),
				makeResult(1),
				makeResult(2),
				makeResult(3),
				makeResult(4),
				makeResult(5),
			},
		}

		// This call should fail since we are not using the segment ID
		// augmented by RetryBeginSegmentPieces
		err = metainfoClient.CommitSegment(ctx, commitSegmentParams)
		rpctest.RequireStatusContains(t, err, rpcstatus.InvalidArgument, "metaclient: Number of valid pieces (4) is less than the success threshold (6).")

		// This call should succeed.
		commitSegmentParams.SegmentID = exchangeSegmentPieceOrdersResp.SegmentID
		err = metainfoClient.CommitSegment(ctx, commitSegmentParams)
		require.NoError(t, err)
	})
}

func TestRetryBeginSegmentPieces_Validation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketName := testrand.BucketName()
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
		metainfoClient := createMetainfoClient(ctx, t, planet)

		params := metaclient.BeginObjectParams{
			Bucket:             []byte(bucketName),
			EncryptedObjectKey: []byte("encrypted-path"),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   256,
			},
		}

		beginObjectResp, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		beginSegmentResp, err := metainfoClient.BeginSegment(ctx, metaclient.BeginSegmentParams{
			StreamID:      beginObjectResp.StreamID,
			Position:      metaclient.SegmentPosition{},
			MaxOrderLimit: 1024,
		})
		require.NoError(t, err)

		t.Run("segment ID missing", func(t *testing.T) {
			_, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
				SegmentID:         nil,
				RetryPieceNumbers: []int{0, 1},
			})
			rpctest.RequireStatus(t, err, rpcstatus.InvalidArgument, "metaclient: segment ID missing")
		})
		t.Run("piece numbers is empty", func(t *testing.T) {
			_, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
				SegmentID:         beginSegmentResp.SegmentID,
				RetryPieceNumbers: nil,
			})
			rpctest.RequireStatus(t, err, rpcstatus.InvalidArgument, "metaclient: piece numbers to exchange cannot be empty")
		})
		t.Run("piece numbers are less than zero", func(t *testing.T) {
			_, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
				SegmentID:         beginSegmentResp.SegmentID,
				RetryPieceNumbers: []int{-1},
			})
			rpctest.RequireStatus(t, err, rpcstatus.InvalidArgument, "metaclient: piece number -1 must be within range [0,7]")
		})
		t.Run("piece numbers are larger than expected", func(t *testing.T) {
			_, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
				SegmentID:         beginSegmentResp.SegmentID,
				RetryPieceNumbers: []int{len(beginSegmentResp.Limits)},
			})
			rpctest.RequireStatus(t, err, rpcstatus.InvalidArgument, "metaclient: piece number 8 must be within range [0,7]")
		})
		t.Run("piece numbers are duplicate", func(t *testing.T) {
			_, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
				SegmentID:         beginSegmentResp.SegmentID,
				RetryPieceNumbers: []int{0, 0},
			})
			rpctest.RequireStatus(t, err, rpcstatus.InvalidArgument, "metaclient: piece number 0 is duplicated")
		})

		t.Run("success", func(t *testing.T) {
			_, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
				SegmentID:         beginSegmentResp.SegmentID,
				RetryPieceNumbers: []int{0, 1},
			})
			require.NoError(t, err)
		})
	})
}

func TestCommitSegment_RejectRetryDuplicate(t *testing.T) {
	// this test is going to make sure commit segment won't allow someone to commit a segment with
	// piece 2 and retried piece 2 together.
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketName := testrand.BucketName()
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
		metainfoClient := createMetainfoClient(ctx, t, planet)

		params := metaclient.BeginObjectParams{
			Bucket:             []byte(bucketName),
			EncryptedObjectKey: []byte("encrypted-path"),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   256,
			},
		}

		beginObjectResp, err := metainfoClient.BeginObject(ctx, params)
		require.NoError(t, err)

		beginSegmentResp, err := metainfoClient.BeginSegment(ctx, metaclient.BeginSegmentParams{
			StreamID:      beginObjectResp.StreamID,
			Position:      metaclient.SegmentPosition{},
			MaxOrderLimit: 1024,
		})
		require.NoError(t, err)

		exchangeSegmentPieceOrdersResp, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
			SegmentID:         beginSegmentResp.SegmentID,
			RetryPieceNumbers: []int{2},
		})
		require.NoError(t, err)

		var nodes = make(map[string]bool)
		makeResult := func(i int, limits []*pb.AddressedOrderLimit) *pb.SegmentPieceUploadResult {
			limit := limits[i].Limit
			node := planet.FindNode(limit.StorageNodeId)
			nodes[node.Addr()] = true
			require.NotNil(t, node, "failed to locate node to sign hash for piece %d", i)
			signer := signing.SignerFromFullIdentity(node.Identity)
			hash, err := signing.SignPieceHash(ctx, signer, &pb.PieceHash{
				PieceSize: 512,
				PieceId:   limit.PieceId,
				Timestamp: time.Now(),
			})
			require.NoError(t, err)
			return &pb.SegmentPieceUploadResult{
				PieceNum: int32(i),
				NodeId:   limit.StorageNodeId,
				Hash:     hash,
			}
		}

		commitSegmentParams := metaclient.CommitSegmentParams{
			SegmentID:         beginSegmentResp.SegmentID,
			PlainSize:         512,
			SizeEncryptedData: 512,
			Encryption: metaclient.SegmentEncryption{
				EncryptedKey: testrand.Bytes(256),
			},
			UploadResult: []*pb.SegmentPieceUploadResult{
				makeResult(0, beginSegmentResp.Limits),
				makeResult(1, beginSegmentResp.Limits),
				makeResult(2, beginSegmentResp.Limits),
				makeResult(2, exchangeSegmentPieceOrdersResp.Limits),
				makeResult(3, beginSegmentResp.Limits),
				makeResult(4, beginSegmentResp.Limits),
			},
		}
		require.Equal(t, 6, len(nodes))

		err = metainfoClient.CommitSegment(ctx, commitSegmentParams)
		rpctest.RequireStatusContains(t, err, rpcstatus.InvalidArgument, "metaclient: Number of valid pieces (5) is less than the success threshold (6).")

		commitSegmentParams.SegmentID = exchangeSegmentPieceOrdersResp.SegmentID
		err = metainfoClient.CommitSegment(ctx, commitSegmentParams)
		rpctest.RequireStatusContains(t, err, rpcstatus.InvalidArgument, "metaclient: Number of valid pieces (5) is less than the success threshold (6).")

		commitSegmentParams.UploadResult = append(commitSegmentParams.UploadResult, makeResult(5, beginSegmentResp.Limits))
		require.Equal(t, 7, len(nodes))
		err = metainfoClient.CommitSegment(ctx, commitSegmentParams)
		require.NoError(t, err)
		err = metainfoClient.CommitObject(ctx, metaclient.CommitObjectParams{
			StreamID: beginObjectResp.StreamID,
		})
		require.NoError(t, err)

		// now we need to make sure that what just happened only used 6 pieces. it would be nice if
		// we had an API that was more direct than this:
		resp, err := metainfoClient.GetObjectIPs(ctx, metaclient.GetObjectIPsParams{
			Bucket:             []byte(bucketName),
			EncryptedObjectKey: []byte("encrypted-path"),
		})
		require.NoError(t, err)

		require.Equal(t, int64(1), resp.SegmentCount)
		require.Equal(t, int64(6), resp.PieceCount)
		piece2Count := 0
		for _, addr := range resp.IPPorts {
			switch string(addr) {
			case planet.FindNode(beginSegmentResp.Limits[2].Limit.StorageNodeId).Addr():
				piece2Count++
			case planet.FindNode(exchangeSegmentPieceOrdersResp.Limits[2].Limit.StorageNodeId).Addr():
				piece2Count++
			}
		}
		require.Equal(t, 1, piece2Count)
	})
}

func TestSegmentPlacementConstraints(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		uplink := planet.Uplinks[0]

		expectedBucketName := "some-bucket"
		err := uplink.Upload(ctx, satellite, expectedBucketName, "file-object", testrand.Bytes(50*memory.KiB))
		require.NoError(t, err)

		metainfoClient, err := uplink.DialMetainfo(ctx, satellite, apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		items, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
			Bucket: []byte(expectedBucketName),
		})
		require.NoError(t, err)
		require.Len(t, items, 1)

		{ // download should succeed because placement allows any node
			_, err := metainfoClient.DownloadObject(ctx, metaclient.DownloadObjectParams{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: items[0].EncryptedObjectKey,
			})
			require.NoError(t, err)
		}

		err = satellite.Metabase.DB.TestingSetPlacementAllSegments(ctx, 1)
		require.NoError(t, err)

		{ // download should fail because non-zero placement and nodes have no country codes
			_, err := metainfoClient.DownloadObject(ctx, metaclient.DownloadObjectParams{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: items[0].EncryptedObjectKey,
			})
			require.Error(t, err)
		}
	})
}

func TestRetryBeginSegmentPieces_EndToEnd(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		metainfoClient := createMetainfoClient(ctx, t, planet)

		data := testrand.Bytes(512)

		err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test")
		require.NoError(t, err)

		beginObjectResp, err := metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
			Bucket:             []byte("test"),
			EncryptedObjectKey: []byte("test"),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
			},
		})
		require.NoError(t, err)

		beginSegmentResp, err := metainfoClient.BeginSegment(ctx, metaclient.BeginSegmentParams{
			StreamID:      beginObjectResp.StreamID,
			Position:      metaclient.SegmentPosition{},
			MaxOrderLimit: 1024,
		})
		require.NoError(t, err)

		retryResponse, err := metainfoClient.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
			SegmentID:         beginSegmentResp.SegmentID,
			RetryPieceNumbers: []int{1, 3},
		})
		require.NoError(t, err)

		// try to use limits from RetryBeginSegmentPieces and upload data
		limits := retryResponse.Limits
		for i, orderLimit := range limits {
			require.NotNil(t, orderLimit.Limit)
			require.NotNil(t, orderLimit.StorageNodeAddress)

			func() {
				nodeURL := (&pb.Node{
					Id:      orderLimit.GetLimit().StorageNodeId,
					Address: orderLimit.GetStorageNodeAddress(),
				}).NodeURL()
				client, err := piecestore.DialReplaySafe(ctx, planet.Uplinks[0].Dialer, nodeURL, piecestore.DefaultConfig)
				require.NoError(t, err)
				defer ctx.Check(client.Close)

				_, err = client.UploadReader(ctx, orderLimit.Limit, beginSegmentResp.PiecePrivateKey, bytes.NewReader(data))
				require.NoError(t, err, "%v", i)
			}()
		}

		// send all oder limits to verify satellite can decode them
		for _, node := range planet.StorageNodes {
			node.Storage2.Orders.SendOrders(ctx, time.Now().Add(24*time.Hour))

			unsent, err := node.DB.Orders().ListUnsent(ctx, 100)
			require.NoError(t, err)
			require.Empty(t, unsent)
		}

		now := time.Now()
		err = planet.Satellites[0].DB.StoragenodeAccounting().GetBandwidthSince(ctx, now.Add(-time.Hour), func(ctx context.Context, sbr *accounting.StoragenodeBandwidthRollup) error {
			require.NotZero(t, sbr.Settled)
			return nil
		})
		require.NoError(t, err)
	})
}

func createMetainfoClient(ctx *testcontext.Context, tb testing.TB, planet *testplanet.Planet) *metaclient.Client {
	apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
	metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
	require.NoError(tb, err)
	tb.Cleanup(func() { ctx.Check(metainfoClient.Close) })
	return metainfoClient
}
