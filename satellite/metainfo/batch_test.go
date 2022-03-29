// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/uplink/private/metaclient"
)

func TestBatch(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		{ // create few buckets and list them in one batch
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
		}

		{ // create bucket, object, upload inline segments in batch, download inline segments in batch
			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "second-test-bucket")
			require.NoError(t, err)

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
				EncryptedMetadata:             metadata,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: testrand.Bytes(32),
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
		}

		{ // test case when StreamID is not set automatically
			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "third-test-bucket")
			require.NoError(t, err)

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
				StreamID:                      beginObjectResp.StreamID,
				EncryptedMetadata:             metadata,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: testrand.Bytes(32),
			})

			responses, err := metainfoClient.Batch(ctx, requests...)
			require.NoError(t, err)
			require.Equal(t, numOfSegments+1, len(responses))
		}
	})
}

func TestDeleteBatchWithoutPermission(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
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
