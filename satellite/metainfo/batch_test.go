// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"crypto/tls"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/internalpb"
	"storj.io/uplink/private/metaclient"
)

func TestBatch(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		t.Run("create few buckets and list them in one batch", func(t *testing.T) {
			requests := make([]metaclient.BatchItem, 0)
			numOfBuckets := 5
			for i := 0; i < numOfBuckets; i++ {
				requests = append(requests, &metaclient.CreateBucketParams{
					Name: []byte("test-bucket-" + strconv.Itoa(i)),
				})
			}
			requests = append(requests, &metaclient.ListBucketsParams{
				ListOpts: metaclient.BucketListOptions{
					Cursor:    "",
					Direction: metaclient.After,
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
		})

		t.Run("create bucket, object, upload inline segments in batch, download inline segments in batch", func(t *testing.T) {
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "second-test-bucket"))

			requests := make([]metaclient.BatchItem, 0)
			requests = append(requests, &metaclient.BeginObjectParams{
				Bucket:             []byte("second-test-bucket"),
				EncryptedObjectKey: []byte("encrypted-path"),
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: storj.EncAESGCM,
					BlockSize:   256,
				},
			})
			numOfSegments := 10
			expectedData := make([][]byte, numOfSegments)
			for i := 0; i < numOfSegments; i++ {
				expectedData[i] = testrand.Bytes(memory.KiB)

				requests = append(requests, &metaclient.MakeInlineSegmentParams{
					Position: metaclient.SegmentPosition{
						Index: int32(i),
					},
					PlainSize:           int64(len(expectedData[i])),
					EncryptedInlineData: expectedData[i],
					Encryption: metaclient.SegmentEncryption{
						EncryptedKey: testrand.Bytes(256),
					},
				})
			}

			metadata, err := pb.Marshal(&pb.StreamMeta{
				NumberOfSegments: int64(numOfSegments),
			})
			require.NoError(t, err)
			requests = append(requests, &metaclient.CommitObjectParams{
				EncryptedUserData: metaclient.EncryptedUserData{
					EncryptedMetadata:             metadata,
					EncryptedMetadataNonce:        testrand.Nonce(),
					EncryptedMetadataEncryptedKey: randomEncryptedKey,
				},
			})

			responses, err := metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfSegments+2, len(responses))

			requests = make([]metaclient.BatchItem, 0)
			requests = append(requests, &metaclient.GetObjectParams{
				Bucket:             []byte("second-test-bucket"),
				EncryptedObjectKey: []byte("encrypted-path"),
			})

			for i := 0; i < numOfSegments-1; i++ {
				requests = append(requests, &metaclient.DownloadSegmentParams{
					Position: metaclient.SegmentPosition{
						Index: int32(i),
					},
				})
			}
			requests = append(requests, &metaclient.DownloadSegmentParams{
				Position: metaclient.SegmentPosition{
					Index: -1,
				},
			})
			responses, err = metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfSegments+1, len(responses))

			for i, response := range responses[1:] {
				downloadResponse, err := response.DownloadSegment()
				require.NoError(t, err)

				require.Equal(t, expectedData[i], downloadResponse.Info.EncryptedInlineData)
			}
		})

		t.Run("StreamID is not set automatically", func(t *testing.T) {
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "third-test-bucket"))

			beginObjectResp, err := metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte("third-test-bucket"),
				EncryptedObjectKey: []byte("encrypted-path"),
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: storj.EncAESGCM,
					BlockSize:   256,
				},
			})
			require.NoError(t, err)

			requests := make([]metaclient.BatchItem, 0)
			numOfSegments := 10
			expectedData := make([][]byte, numOfSegments)
			for i := 0; i < numOfSegments; i++ {
				expectedData[i] = testrand.Bytes(memory.KiB)

				requests = append(requests, &metaclient.MakeInlineSegmentParams{
					StreamID: beginObjectResp.StreamID,
					Position: metaclient.SegmentPosition{
						Index: int32(i),
					},
					PlainSize:           int64(len(expectedData[i])),
					EncryptedInlineData: expectedData[i],
					Encryption: metaclient.SegmentEncryption{
						EncryptedKey: testrand.Bytes(256),
					},
				})
			}

			metadata, err := pb.Marshal(&pb.StreamMeta{
				NumberOfSegments: int64(numOfSegments),
			})
			require.NoError(t, err)
			requests = append(requests, &metaclient.CommitObjectParams{
				StreamID: beginObjectResp.StreamID,
				EncryptedUserData: metaclient.EncryptedUserData{
					EncryptedMetadata:             metadata,
					EncryptedMetadataNonce:        testrand.Nonce(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(48),
				},
			})

			responses, err := metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfSegments+1, len(responses))
		})

		t.Run("retry segment pieces", func(t *testing.T) {
			bucketName := testrand.BucketName()
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

			endpoint := planet.Satellites[0].Metainfo.Endpoint

			beginObjectResp, err := endpoint.BeginObject(ctx, &pb.BeginObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("test-object"),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
				},
			})
			require.NoError(t, err)

			beginSegmentResp, err := endpoint.BeginSegment(peerctx, &pb.BeginSegmentRequest{
				Header:        &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				StreamId:      beginObjectResp.StreamId,
				Position:      &pb.SegmentPosition{},
				MaxOrderLimit: memory.MiB.Int64(),
			})
			require.NoError(t, err)

			_, err = endpoint.Batch(peerctx, &pb.BatchRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Requests: []*pb.BatchRequestItem{
					{
						Request: &pb.BatchRequestItem_SegmentBeginRetryPieces{
							SegmentBeginRetryPieces: &pb.RetryBeginSegmentPiecesRequest{
								Header:            &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								SegmentId:         beginSegmentResp.SegmentId,
								RetryPieceNumbers: []int32{0},
							},
						},
					},
				},
			})
			require.NoError(t, err)
		})
	})
}

func TestBatchBeginObjectMultipartDetection(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.TestingNoPendingObjectUpload = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		bucketName := testrand.BucketName()
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

		endpoint := planet.Satellites[0].Metainfo.Endpoint

		t.Run("multipart", func(t *testing.T) {
			response, err := endpoint.Batch(peerctx, &pb.BatchRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Requests: []*pb.BatchRequestItem{
					{
						Request: &pb.BatchRequestItem_ObjectBegin{
							ObjectBegin: &pb.BeginObjectRequest{
								Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Bucket:             []byte(bucketName),
								EncryptedObjectKey: []byte("some-key"),
								EncryptionParameters: &pb.EncryptionParameters{
									CipherSuite: pb.CipherSuite_ENC_AESGCM,
									BlockSize:   256,
								},
							},
						},
					},
				},
			})
			require.NoError(t, err)
			require.Len(t, response.Responses, 1)

			satStreamID := &internalpb.StreamID{}
			err = pb.Unmarshal(response.Responses[0].GetObjectBegin().StreamId.Bytes(), satStreamID)
			require.NoError(t, err)
			require.True(t, satStreamID.MultipartObject)
		})

		t.Run("non multipart with remote segment", func(t *testing.T) {
			response, err := endpoint.Batch(peerctx, &pb.BatchRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Requests: []*pb.BatchRequestItem{
					{
						Request: &pb.BatchRequestItem_ObjectBegin{
							ObjectBegin: &pb.BeginObjectRequest{
								Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Bucket:             []byte(bucketName),
								EncryptedObjectKey: []byte("some-key"),
								EncryptionParameters: &pb.EncryptionParameters{
									CipherSuite: pb.CipherSuite_ENC_AESGCM,
									BlockSize:   256,
								},
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_SegmentBegin{
							SegmentBegin: &pb.BeginSegmentRequest{
								Header:        &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Position:      &pb.SegmentPosition{},
								MaxOrderLimit: memory.MiB.Int64(),
							},
						},
					},
				},
			})
			require.NoError(t, err)
			require.Len(t, response.Responses, 2)

			satStreamID := &internalpb.StreamID{}
			err = pb.Unmarshal(response.Responses[0].GetObjectBegin().StreamId.Bytes(), satStreamID)
			require.NoError(t, err)
			require.False(t, satStreamID.MultipartObject)

			// upload around other requests
			response, err = endpoint.Batch(peerctx, &pb.BatchRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Requests: []*pb.BatchRequestItem{
					{
						Request: &pb.BatchRequestItem_BucketList{
							BucketList: &pb.ListBucketsRequest{
								Header:    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Direction: pb.ListDirection_AFTER,
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_ObjectBegin{
							ObjectBegin: &pb.BeginObjectRequest{
								Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Bucket:             []byte(bucketName),
								EncryptedObjectKey: []byte("some-key"),
								EncryptionParameters: &pb.EncryptionParameters{
									CipherSuite: pb.CipherSuite_ENC_AESGCM,
									BlockSize:   256,
								},
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_SegmentBegin{
							SegmentBegin: &pb.BeginSegmentRequest{
								Header:        &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Position:      &pb.SegmentPosition{},
								MaxOrderLimit: memory.MiB.Int64(),
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_BucketList{
							BucketList: &pb.ListBucketsRequest{
								Header:    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Direction: pb.ListDirection_AFTER,
							},
						},
					},
				},
			})
			require.NoError(t, err)
			require.Len(t, response.Responses, 4)

			satStreamID = &internalpb.StreamID{}
			err = pb.Unmarshal(response.Responses[1].GetObjectBegin().StreamId.Bytes(), satStreamID)
			require.NoError(t, err)
			require.False(t, satStreamID.MultipartObject)
		})

		t.Run("non multipart with inline segment", func(t *testing.T) {
			response, err := endpoint.Batch(peerctx, &pb.BatchRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Requests: []*pb.BatchRequestItem{
					{
						Request: &pb.BatchRequestItem_ObjectBegin{
							ObjectBegin: &pb.BeginObjectRequest{
								Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Bucket:             []byte(bucketName),
								EncryptedObjectKey: []byte("inline-key"),
								EncryptionParameters: &pb.EncryptionParameters{
									CipherSuite: pb.CipherSuite_ENC_AESGCM,
									BlockSize:   256,
								},
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_SegmentMakeInline{
							SegmentMakeInline: &pb.SegmentMakeInlineRequest{
								Header:              &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Position:            &pb.SegmentPosition{},
								EncryptedInlineData: testrand.Bytes(memory.KiB),
								PlainSize:           memory.KiB.Int64(),
								EncryptedKey:        testrand.Bytes(32),
							},
						},
					},
				},
			})
			require.NoError(t, err)
			require.Len(t, response.Responses, 2)

			satStreamID := &internalpb.StreamID{}
			err = pb.Unmarshal(response.Responses[0].GetObjectBegin().StreamId.Bytes(), satStreamID)
			require.NoError(t, err)
			require.False(t, satStreamID.MultipartObject)

			// upload around other requests
			response, err = endpoint.Batch(peerctx, &pb.BatchRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Requests: []*pb.BatchRequestItem{
					{
						Request: &pb.BatchRequestItem_BucketList{
							BucketList: &pb.ListBucketsRequest{
								Header:    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Direction: pb.ListDirection_AFTER,
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_ObjectBegin{
							ObjectBegin: &pb.BeginObjectRequest{
								Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Bucket:             []byte(bucketName),
								EncryptedObjectKey: []byte("inline-key-2"),
								EncryptionParameters: &pb.EncryptionParameters{
									CipherSuite: pb.CipherSuite_ENC_AESGCM,
									BlockSize:   256,
								},
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_SegmentMakeInline{
							SegmentMakeInline: &pb.SegmentMakeInlineRequest{
								Header:              &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Position:            &pb.SegmentPosition{},
								EncryptedInlineData: testrand.Bytes(memory.KiB),
								PlainSize:           memory.KiB.Int64(),
								EncryptedKey:        testrand.Bytes(32),
							},
						},
					},
					{
						Request: &pb.BatchRequestItem_BucketList{
							BucketList: &pb.ListBucketsRequest{
								Header:    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
								Direction: pb.ListDirection_AFTER,
							},
						},
					},
				},
			})
			require.NoError(t, err)
			require.Len(t, response.Responses, 4)

			satStreamID = &internalpb.StreamID{}
			err = pb.Unmarshal(response.Responses[1].GetObjectBegin().StreamId.Bytes(), satStreamID)
			require.NoError(t, err)
			require.False(t, satStreamID.MultipartObject)
		})
	})
}

func TestDeleteBatchWithoutPermission(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		apiKey, err = apiKey.Restrict(macaroon.WithNonce(macaroon.Caveat{
			DisallowLists: true,
			DisallowReads: true,
		}))
		require.NoError(t, err)

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		responses, err := metainfoClient.Batch(ctx,
			// this request was causing panic becase for deleting object
			// its possible to return no error and empty response for
			// specific set of permissions, see `apiKey.Restrict` from above
			&metaclient.BeginDeleteObjectParams{
				Bucket:             []byte("test-bucket"),
				EncryptedObjectKey: []byte("not-existing-object"),
			},

			// TODO this code should be enabled then issue with read permissions in
			// DeleteBucket method currently user have always permission to read bucket
			// https://storjlabs.atlassian.net/browse/USR-603
			// when it will be fixed commented code from bellow should replace existing DeleteBucketParams
			// the same situation like above
			// &metaclient.DeleteBucketParams{
			// 	Name: []byte("not-existing-bucket"),
			// },

			&metaclient.DeleteBucketParams{
				Name: []byte("test-bucket"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, 2, len(responses))
	})
}
