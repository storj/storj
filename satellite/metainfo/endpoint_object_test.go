// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/object"
	"storj.io/uplink/private/testuplink"
)

func TestObject_NoStorageNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxObjectKeyLength(1024),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		bucketName := "testbucket"
		deleteBucket := func() error {
			_, err := metainfoClient.DeleteBucket(ctx, metaclient.DeleteBucketParams{
				Name:      []byte(bucketName),
				DeleteAll: true,
			})
			return err
		}

		t.Run("get objects", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			files := make([]string, 10)
			data := testrand.Bytes(1 * memory.KiB)
			for i := 0; i < len(files); i++ {
				files[i] = "path" + strconv.Itoa(i)
				err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, files[i], data)
				require.NoError(t, err)
			}

			expectedBucketName := bucketName
			items, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket:                []byte(expectedBucketName),
				IncludeSystemMetadata: true,
			})
			require.NoError(t, err)
			require.Equal(t, len(files), len(items))
			for _, item := range items {
				require.NotEmpty(t, item.EncryptedObjectKey)
				require.True(t, item.CreatedAt.Before(time.Now()))

				object, err := metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
					Bucket:             []byte(expectedBucketName),
					EncryptedObjectKey: item.EncryptedObjectKey,
				})
				require.NoError(t, err)
				require.Equal(t, item.EncryptedObjectKey, object.EncryptedObjectKey)

				require.NotEmpty(t, object.StreamID)
			}

			items, _, err = metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket: []byte(expectedBucketName),
				Limit:  3,
			})
			require.NoError(t, err)
			require.Equal(t, 3, len(items))

		})

		t.Run("list service", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

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
				err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, item.Key, item.Value)
				assert.NoError(t, err)
			}

			project, err := planet.Uplinks[0].GetProject(ctx, planet.Satellites[0])
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			objects := project.ListObjects(ctx, "testbucket", &uplink.ListObjectsOptions{
				Recursive: true,
			})

			listItems := make([]*uplink.Object, 0)
			for objects.Next() {
				listItems = append(listItems, objects.Item())
			}
			require.NoError(t, objects.Err())

			expected := []storj.Object{
				{Path: "müsic"},
				{Path: "müsic/album/söng3.mp3"},
				{Path: "müsic/söng1.mp3"},
				{Path: "müsic/söng2.mp3"},
				{Path: "müsic/söng4.mp3"},
				{Path: "sample.😶"},
				{Path: "ビデオ/movie.mkv"},
			}

			require.Equal(t, len(expected), len(listItems))
			sort.Slice(listItems, func(i, k int) bool {
				return listItems[i].Key < listItems[k].Key
			})
			for i, item := range expected {
				require.Equal(t, item.Path, listItems[i].Key)
				require.Equal(t, item.IsPrefix, listItems[i].IsPrefix)
			}

			objects = project.ListObjects(ctx, bucketName, &uplink.ListObjectsOptions{
				Recursive: false,
			})

			listItems = make([]*uplink.Object, 0)
			for objects.Next() {
				listItems = append(listItems, objects.Item())
			}
			require.NoError(t, objects.Err())

			expected = []storj.Object{
				{Path: "müsic"},
				{Path: "müsic/", IsPrefix: true},
				{Path: "sample.😶"},
				{Path: "ビデオ/", IsPrefix: true},
			}

			require.Equal(t, len(expected), len(listItems))
			sort.Slice(listItems, func(i, k int) bool {
				return listItems[i].Key < listItems[k].Key
			})
			for i, item := range expected {
				t.Log(item.Path, listItems[i].Key)
				require.Equal(t, item.Path, listItems[i].Key)
				require.Equal(t, item.IsPrefix, listItems[i].IsPrefix)
			}
		})

		// ensures that CommitObject returns an error when the metadata provided by the user is too large.
		t.Run("validate metadata size", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			err = planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
			require.NoError(t, err)

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
					BlockSize:   256,
					CipherSuite: storj.EncNull,
				},
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}
			beginObjectResponse, err := metainfoClient.BeginObject(ctx, params)
			require.NoError(t, err)

			// 5KiB metadata should fail because it is too large.
			metadata, err := pb.Marshal(&pb.StreamMeta{
				EncryptedStreamInfo: testrand.Bytes(5 * memory.KiB),
				NumberOfSegments:    1,
			})
			require.NoError(t, err)
			err = metainfoClient.CommitObject(ctx, metaclient.CommitObjectParams{
				StreamID:                      beginObjectResponse.StreamID,
				EncryptedMetadata:             metadata,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: testrand.Bytes(32),
			})
			require.Error(t, err)
			assertInvalidArgument(t, err, true)

			// 1KiB metadata should not fail.
			metadata, err = pb.Marshal(&pb.StreamMeta{
				EncryptedStreamInfo: testrand.Bytes(1 * memory.KiB),
				NumberOfSegments:    1,
			})
			require.NoError(t, err)
			err = metainfoClient.CommitObject(ctx, metaclient.CommitObjectParams{
				StreamID:                      beginObjectResponse.StreamID,
				EncryptedMetadata:             metadata,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: testrand.Bytes(32),
			})
			require.NoError(t, err)
		})

		t.Run("update metadata", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			satelliteSys := planet.Satellites[0]

			// upload a small inline object
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "testobject", testrand.Bytes(1*memory.KiB))
			require.NoError(t, err)

			objects, err := satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			getResp, err := satelliteSys.API.Metainfo.Endpoint.GetObject(ctx, &pb.ObjectGetRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:        []byte("testbucket"),
				EncryptedPath: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)

			testEncryptedMetadata := testrand.BytesInt(64)
			testEncryptedMetadataEncryptedKey := testrand.BytesInt(32)
			testEncryptedMetadataNonce := testrand.Nonce()

			// update the object metadata
			_, err = satelliteSys.API.Metainfo.Endpoint.UpdateObjectMetadata(ctx, &pb.ObjectUpdateMetadataRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:                        getResp.Object.Bucket,
				EncryptedObjectKey:            getResp.Object.EncryptedPath,
				Version:                       getResp.Object.Version,
				StreamId:                      getResp.Object.StreamId,
				EncryptedMetadataNonce:        testEncryptedMetadataNonce,
				EncryptedMetadata:             testEncryptedMetadata,
				EncryptedMetadataEncryptedKey: testEncryptedMetadataEncryptedKey,
			})
			require.NoError(t, err)

			// assert the metadata has been updated
			objects, err = satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)
			assert.Equal(t, testEncryptedMetadata, objects[0].EncryptedMetadata)
			assert.Equal(t, testEncryptedMetadataEncryptedKey, objects[0].EncryptedMetadataEncryptedKey)
			assert.Equal(t, testEncryptedMetadataNonce[:], objects[0].EncryptedMetadataNonce)
		})

		t.Run("check delete rights on upload", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			up := planet.Uplinks[0]

			err := up.CreateBucket(ctx, planet.Satellites[0], bucketName)
			require.NoError(t, err)

			data := testrand.Bytes(1 * memory.KiB)
			err = up.Upload(ctx, planet.Satellites[0], bucketName, "test-key", data)
			require.NoError(t, err)

			access := up.Access[planet.Satellites[0].ID()]

			overwrite := func(allowDelete bool) error {
				permission := uplink.FullPermission()
				permission.AllowDelete = allowDelete

				sharedAccess, err := access.Share(permission)
				require.NoError(t, err)

				project, err := uplink.OpenProject(ctx, sharedAccess)
				require.NoError(t, err)
				defer ctx.Check(project.Close)

				upload, err := project.UploadObject(ctx, bucketName, "test-key", nil)
				require.NoError(t, err)

				_, err = upload.Write([]byte("new data"))
				require.NoError(t, err)

				return upload.Commit()
			}

			require.Error(t, overwrite(false))
			require.NoError(t, overwrite(true))
		})

		t.Run("immutable upload", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]

			permission := uplink.Permission{AllowUpload: true} // AllowDelete: false
			sharedAccess, err := access.Share(permission)
			require.NoError(t, err)

			project, err := uplink.OpenProject(ctx, sharedAccess)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			_, err = project.CreateBucket(ctx, bucketName)
			require.NoError(t, err)

			// Uploading the object for first time should be successful.
			upload, err := project.UploadObject(ctx, bucketName, "test-key", nil)
			require.NoError(t, err)

			_, err = upload.Write(testrand.Bytes(1 * memory.KiB))
			require.NoError(t, err)

			err = upload.Commit()
			require.NoError(t, err)

			// Overwriting the object should fail on Commit.
			upload, err = project.UploadObject(ctx, bucketName, "test-key", nil)
			require.NoError(t, err)

			_, err = upload.Write(testrand.Bytes(1 * memory.KiB))
			require.NoError(t, err)

			err = upload.Commit()
			require.Error(t, err)
		})

		t.Run("stable upload id", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			err = planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
			require.NoError(t, err)

			beginResp, err := metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("a/b/testobject"),
				EncryptionParameters: storj.EncryptionParameters{
					CipherSuite: storj.EncAESGCM,
					BlockSize:   256,
				},
			})
			require.NoError(t, err)

			// List the root of the bucket recursively
			listResp, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket:                []byte(bucketName),
				Status:                int32(metabase.Pending),
				Recursive:             true,
				IncludeSystemMetadata: true,
			})
			require.NoError(t, err)
			require.Len(t, listResp, 1)
			// check that BeginObject and ListObjects return the same StreamID.
			assert.Equal(t, beginResp.StreamID, listResp[0].StreamID)

			// List with prefix non-recursively
			listResp2, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket:                []byte(bucketName),
				Status:                int32(metabase.Pending),
				EncryptedPrefix:       []byte("a/b/"),
				IncludeSystemMetadata: true,
			})
			require.NoError(t, err)
			require.Len(t, listResp2, 1)
			// check that the StreamID is still the same.
			assert.Equal(t, listResp[0].StreamID, listResp2[0].StreamID)

			// List with prefix recursively
			listResp3, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket:                []byte(bucketName),
				Status:                int32(metabase.Pending),
				EncryptedPrefix:       []byte("a/b/"),
				Recursive:             true,
				IncludeSystemMetadata: true,
			})
			require.NoError(t, err)
			require.Len(t, listResp3, 1)
			// check that the StreamID is still the same.
			assert.Equal(t, listResp[0].StreamID, listResp3[0].StreamID)

			// List the pending object directly
			listResp4, err := metainfoClient.ListPendingObjectStreams(ctx, metaclient.ListPendingObjectStreamsParams{
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("a/b/testobject"),
			})
			require.NoError(t, err)
			require.Len(t, listResp4.Items, 1)
			// check that the StreamID is still the same.
			assert.Equal(t, listResp[0].StreamID, listResp4.Items[0].StreamID)
		})

		// ensures that BeginObject returns an error when the encrypted key provided by the user is too large.
		t.Run("validate encrypted object key length", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
			require.NoError(t, err)

			params := metaclient.BeginObjectParams{
				Bucket: []byte(bucketName),
				EncryptionParameters: storj.EncryptionParameters{
					BlockSize:   256,
					CipherSuite: storj.EncNull,
				},
			}

			params.EncryptedObjectKey = testrand.Bytes(500)
			_, err = metainfoClient.BeginObject(ctx, params)
			require.NoError(t, err)

			params.EncryptedObjectKey = testrand.Bytes(1024)
			_, err = metainfoClient.BeginObject(ctx, params)
			require.NoError(t, err)

			params.EncryptedObjectKey = testrand.Bytes(2048)
			_, err = metainfoClient.BeginObject(ctx, params)
			require.Error(t, err)
			require.True(t, rpcstatus.Code(err) == rpcstatus.InvalidArgument)
		})

		t.Run("delete not existing object", func(t *testing.T) {
			expectedBucketName := bucketName

			// non-pending non-existent objects return no error
			_, err = metainfoClient.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: []byte("bad path"),
			})
			require.NoError(t, err)

			// pending non-existent objects return an RPC error
			signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			streamUUID := testrand.UUID()
			satStreamID := &internalpb.StreamID{CreationDate: time.Now(), StreamId: streamUUID[:]}
			signedStreamID, err := metainfo.SignStreamID(ctx, signer, satStreamID)
			require.NoError(t, err)
			encodedStreamID, err := pb.Marshal(signedStreamID)
			require.NoError(t, err)
			streamID, err := storj.StreamIDFromBytes(encodedStreamID)
			require.NoError(t, err)
			_, err = metainfoClient.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: []byte("bad path"),
				Status:             int32(metabase.Pending),
				StreamID:           streamID,
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		})
	})
}

func TestEndpoint_Object_With_StorageNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.GeoIP.MockCountries = []string{"DE"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		bucketName := "testbucket"
		deleteBucket := func(bucketName string) func() error {
			return func() error {
				_, err := metainfoClient.DeleteBucket(ctx, metaclient.DeleteBucketParams{
					Name:      []byte(bucketName),
					DeleteAll: true,
				})
				return err
			}
		}

		t.Run("begin commit", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			buckets := planet.Satellites[0].API.Buckets.Service

			bucket := storj.Bucket{
				Name:      bucketName,
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Placement: storj.EU,
			}

			_, err := buckets.CreateBucket(ctx, bucket)
			require.NoError(t, err)

			params := metaclient.BeginObjectParams{
				Bucket:             []byte(bucket.Name),
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
			beginObjectResponse, err := metainfoClient.BeginObject(ctx, params)
			require.NoError(t, err)

			streamID := internalpb.StreamID{}
			err = pb.Unmarshal(beginObjectResponse.StreamID.Bytes(), &streamID)
			require.NoError(t, err)
			require.Equal(t, int32(storj.EU), streamID.Placement)

			response, err := metainfoClient.BeginSegment(ctx, metaclient.BeginSegmentParams{
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
				nodeID := response.Limits[num].Limit.StorageNodeId
				hash := &pb.PieceHash{
					PieceId:   response.Limits[num].Limit.PieceId,
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
			err = metainfoClient.CommitSegment(ctx, metaclient.CommitSegmentParams{
				SegmentID: response.SegmentID,
				Encryption: storj.SegmentEncryption{
					EncryptedKey: testrand.Bytes(256),
				},
				PlainSize:         5000,
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
			err = metainfoClient.CommitObject(ctx, metaclient.CommitObjectParams{
				StreamID:                      beginObjectResponse.StreamID,
				EncryptedMetadata:             metadata,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: testrand.Bytes(32),
			})
			require.NoError(t, err)

			objects, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
				Bucket:                []byte(bucket.Name),
				IncludeSystemMetadata: true,
			})
			require.NoError(t, err)
			require.Len(t, objects, 1)
			require.Equal(t, params.EncryptedObjectKey, objects[0].EncryptedObjectKey)
			// TODO find better way to compare (one ExpiresAt contains time zone informations)
			require.Equal(t, params.ExpiresAt.Unix(), objects[0].ExpiresAt.Unix())

			object, err := metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte(bucket.Name),
				EncryptedObjectKey: objects[0].EncryptedObjectKey,
			})
			require.NoError(t, err)

			project := planet.Uplinks[0].Projects[0]
			allObjects, err := planet.Satellites[0].Metabase.DB.TestingAllCommittedObjects(ctx, project.ID, object.Bucket)
			require.NoError(t, err)
			require.Len(t, allObjects, 1)
		})

		t.Run("get object IP", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
			uplnk := planet.Uplinks[0]
			uplinkCtx := testuplink.WithMaxSegmentSize(ctx, 5*memory.KB)
			sat := planet.Satellites[0]

			require.NoError(t, uplnk.CreateBucket(uplinkCtx, sat, bucketName))
			require.NoError(t, uplnk.Upload(uplinkCtx, sat, bucketName, "jones", testrand.Bytes(20*memory.KB)))
			ips, err := object.GetObjectIPs(ctx, uplink.Config{}, access, bucketName, "jones")
			require.NoError(t, err)
			require.True(t, len(ips) > 0)

			// verify it's a real IP with valid host and port
			for _, ip := range ips {
				host, port, err := net.SplitHostPort(string(ip))
				require.NoError(t, err)
				netIP := net.ParseIP(host)
				require.NotNil(t, netIP)
				_, err = strconv.Atoi(port)
				require.NoError(t, err)
			}
		})

		t.Run("multipart object download rejection", func(t *testing.T) {
			defer ctx.Check(deleteBucket("pip-first"))
			defer ctx.Check(deleteBucket("pip-second"))
			defer ctx.Check(deleteBucket("pip-third"))

			data := testrand.Bytes(20 * memory.KB)
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "pip-first", "non-multipart-object", data)
			require.NoError(t, err)

			project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
			require.NoError(t, err)

			_, err = project.EnsureBucket(ctx, "pip-second")
			require.NoError(t, err)
			info, err := project.BeginUpload(ctx, "pip-second", "multipart-object", nil)
			require.NoError(t, err)
			upload, err := project.UploadPart(ctx, "pip-second", "multipart-object", info.UploadID, 1)
			require.NoError(t, err)
			_, err = upload.Write(data)
			require.NoError(t, err)
			require.NoError(t, upload.Commit())
			_, err = project.CommitUpload(ctx, "pip-second", "multipart-object", info.UploadID, nil)
			require.NoError(t, err)

			_, err = project.EnsureBucket(ctx, "pip-third")
			require.NoError(t, err)
			info, err = project.BeginUpload(ctx, "pip-third", "multipart-object-third", nil)
			require.NoError(t, err)
			for i := 0; i < 4; i++ {
				upload, err := project.UploadPart(ctx, "pip-third", "multipart-object-third", info.UploadID, uint32(i+1))
				require.NoError(t, err)
				_, err = upload.Write(data)
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}
			_, err = project.CommitUpload(ctx, "pip-third", "multipart-object-third", info.UploadID, nil)
			require.NoError(t, err)

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllCommittedObjects(ctx, planet.Uplinks[0].Projects[0].ID, "pip-first")
			require.NoError(t, err)
			require.Len(t, objects, 1)

			// verify that standard objects can be downloaded in an old way (index = -1 as last segment)
			object, err := metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte("pip-first"),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)
			_, err = metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: storj.SegmentPosition{
					Index: -1,
				},
			})
			require.NoError(t, err)

			objects, err = planet.Satellites[0].Metabase.DB.TestingAllCommittedObjects(ctx, planet.Uplinks[0].Projects[0].ID, "pip-second")
			require.NoError(t, err)
			require.Len(t, objects, 1)

			// verify that multipart objects (single segment) CANNOT be downloaded in an old way (index = -1 as last segment)
			object, err = metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte("pip-second"),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)
			_, err = metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: storj.SegmentPosition{
					Index: -1,
				},
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "Used uplink version cannot download multipart objects.")

			objects, err = planet.Satellites[0].Metabase.DB.TestingAllCommittedObjects(ctx, planet.Uplinks[0].Projects[0].ID, "pip-third")
			require.NoError(t, err)
			require.Len(t, objects, 1)

			// verify that multipart objects (multiple segments) CANNOT be downloaded in an old way (index = -1 as last segment)
			object, err = metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte("pip-third"),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)
			_, err = metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: storj.SegmentPosition{
					Index: -1,
				},
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "Used uplink version cannot download multipart objects.")
		})

		t.Run("object override on upload", func(t *testing.T) {
			defer ctx.Check(deleteBucket("pip-first"))

			initialData := testrand.Bytes(20 * memory.KB)
			overrideData := testrand.Bytes(25 * memory.KB)

			{ // committed object

				// upload committed object
				err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "pip-first", "committed-object", initialData)
				require.NoError(t, err)

				// upload once again to override
				err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "pip-first", "committed-object", overrideData)
				require.NoError(t, err)

				data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "pip-first", "committed-object")
				require.NoError(t, err)
				require.Equal(t, overrideData, data)
			}

			{ // pending object
				project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
				require.NoError(t, err)
				defer ctx.Check(project.Close)

				// upload pending object
				info, err := project.BeginUpload(ctx, "pip-first", "pending-object", nil)
				require.NoError(t, err)
				upload, err := project.UploadPart(ctx, "pip-first", "pending-object", info.UploadID, 1)
				require.NoError(t, err)
				_, err = upload.Write(initialData)
				require.NoError(t, err)
				require.NoError(t, upload.Commit())

				// upload once again to override
				err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "pip-first", "pending-object", overrideData)
				require.NoError(t, err)

				data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "pip-first", "pending-object")
				require.NoError(t, err)
				require.Equal(t, overrideData, data)
			}
		})

		t.Run("upload with placement", func(t *testing.T) {
			defer ctx.Check(deleteBucket("initial-bucket"))

			bucketName := "initial-bucket"
			objectName := "file1"

			apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
			fmt.Println(apiKey)
			buckets := planet.Satellites[0].API.Buckets.Service

			bucket := storj.Bucket{
				Name:      bucketName,
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Placement: storj.EU,
			}
			_, err := buckets.CreateBucket(ctx, bucket)
			require.NoError(t, err)

			// this should be bigger than the max inline segment
			content := make([]byte, 5000)
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucket.Name, objectName, content)
			require.NoError(t, err)

			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, len(segments))
			require.Equal(t, storj.EU, segments[0].Placement)
		})
	})
}

func TestMoveObject_Geofencing(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			buckets := satellite.API.Buckets.Service
			uplink := planet.Uplinks[0]
			projectID := uplink.Projects[0].ID

			// create buckets with different placement
			createGeofencedBucket(t, ctx, buckets, projectID, "global1", storj.EveryCountry)
			createGeofencedBucket(t, ctx, buckets, projectID, "global2", storj.EveryCountry)
			createGeofencedBucket(t, ctx, buckets, projectID, "us1", storj.US)
			createGeofencedBucket(t, ctx, buckets, projectID, "us2", storj.US)
			createGeofencedBucket(t, ctx, buckets, projectID, "eu1", storj.EU)

			// upload an object to one of the global buckets
			err := uplink.Upload(ctx, satellite, "global1", "testobject", []byte{})
			require.NoError(t, err)

			project, err := uplink.GetProject(ctx, satellite)
			require.NoError(t, err)

			// move the object to a new key within the same bucket
			err = project.MoveObject(ctx, "global1", "testobject", "global1", "movedobject", nil)
			require.NoError(t, err)

			// move the object to the other global bucket
			err = project.MoveObject(ctx, "global1", "movedobject", "global2", "movedobject", nil)
			require.NoError(t, err)

			// move the object to a geofenced bucket - should fail
			err = project.MoveObject(ctx, "global2", "movedobject", "us1", "movedobject", nil)
			require.Error(t, err)

			// upload an object to one of the US-geofenced buckets
			err = uplink.Upload(ctx, satellite, "us1", "testobject", []byte{})
			require.NoError(t, err)

			// move the object to a new key within the same bucket
			err = project.MoveObject(ctx, "us1", "testobject", "us1", "movedobject", nil)
			require.NoError(t, err)

			// move the object to the other US-geofenced bucket
			err = project.MoveObject(ctx, "us1", "movedobject", "us2", "movedobject", nil)
			require.NoError(t, err)

			// move the object to the EU-geofenced bucket - should fail
			err = project.MoveObject(ctx, "us2", "movedobject", "eu1", "movedobject", nil)
			require.Error(t, err)

			// move the object to a non-geofenced bucket - should fail
			err = project.MoveObject(ctx, "us2", "movedobject", "global1", "movedobject", nil)
			require.Error(t, err)
		},
	)
}

func createGeofencedBucket(t *testing.T, ctx *testcontext.Context, buckets *buckets.Service, projectID uuid.UUID, bucketName string, placement storj.PlacementConstraint) {
	// generate the bucket id
	bucketID, err := uuid.New()
	require.NoError(t, err)

	// create the bucket
	_, err = buckets.CreateBucket(ctx, storj.Bucket{
		ID:        bucketID,
		Name:      bucketName,
		ProjectID: projectID,
		Placement: placement,
	})
	require.NoError(t, err)

	// check that the bucket placement is correct
	bucket, err := buckets.GetBucket(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	require.Equal(t, placement, bucket.Placement)
}

func TestEndpoint_DeleteCommittedObject(t *testing.T) {
	bucketName := "a-bucket"
	createObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "object-filename", data)
		require.NoError(t, err)
	}
	deleteAllObjects := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metabase.DB.TestingAllCommittedObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(items), 1)

		for _, item := range items {
			_, err = planet.Satellites[0].Metainfo.Endpoint.DeleteCommittedObject(ctx, projectID, bucketName, item.ObjectKey)
			require.NoError(t, err)
		}

		items, err = planet.Satellites[0].Metabase.DB.TestingAllCommittedObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}
	testDeleteObject(t, createObject, deleteAllObjects)
}

func TestEndpoint_DeletePendingObject(t *testing.T) {
	bucketName := "a-bucket"
	createObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		// TODO This should be replaced by a call to testplanet.Uplink.MultipartUpload when available.
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err, "failed to retrieve project")

		_, err = project.EnsureBucket(ctx, bucketName)
		require.NoError(t, err, "failed to create bucket")

		info, err := project.BeginUpload(ctx, bucketName, "object-filename", &uplink.UploadOptions{})
		require.NoError(t, err, "failed to start multipart upload")

		upload, err := project.UploadPart(ctx, bucketName, bucketName, info.UploadID, 1)
		require.NoError(t, err, "failed to put object part")
		_, err = upload.Write(data)
		require.NoError(t, err, "failed to put object part")
		require.NoError(t, upload.Commit(), "failed to put object part")
	}
	deleteAllObjects := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metabase.DB.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(items), 1)

		for _, item := range items {
			deletedObjects, err := planet.Satellites[0].Metainfo.Endpoint.DeletePendingObject(ctx,
				metabase.ObjectStream{
					ProjectID:  projectID,
					BucketName: bucketName,
					ObjectKey:  item.ObjectKey,
					Version:    metabase.DefaultVersion,
					StreamID:   item.StreamID,
				})
			require.NoError(t, err)
			require.Len(t, deletedObjects, 1)
		}

		items, err = planet.Satellites[0].Metabase.DB.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}
	testDeleteObject(t, createObject, deleteAllObjects)
}

func TestEndpoint_DeleteObjectAnyStatus(t *testing.T) {
	bucketName := "a-bucket"
	createCommittedObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "object-filename", data)
		require.NoError(t, err)
	}
	deleteAllCommittedObjects := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metabase.DB.TestingAllCommittedObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(items), 1)

		for _, item := range items {
			deletedObjects, err := planet.Satellites[0].Metainfo.Endpoint.DeleteObjectAnyStatus(ctx, metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: bucketName,
				ObjectKey:  item.ObjectKey,
			})
			require.NoError(t, err)
			require.Len(t, deletedObjects, 1)
		}

		items, err = planet.Satellites[0].Metabase.DB.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}
	testDeleteObject(t, createCommittedObject, deleteAllCommittedObjects)

	createPendingObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		// TODO This should be replaced by a call to testplanet.Uplink.MultipartUpload when available.
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err, "failed to retrieve project")

		_, err = project.EnsureBucket(ctx, bucketName)
		require.NoError(t, err, "failed to create bucket")

		info, err := project.BeginUpload(ctx, bucketName, "object-filename", &uplink.UploadOptions{})
		require.NoError(t, err, "failed to start multipart upload")

		upload, err := project.UploadPart(ctx, bucketName, bucketName, info.UploadID, 1)
		require.NoError(t, err, "failed to put object part")
		_, err = upload.Write(data)
		require.NoError(t, err, "failed to start multipart upload")
		require.NoError(t, upload.Commit(), "failed to start multipart upload")
	}

	deleteAllPendingObjects := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metabase.DB.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(items), 1)

		for _, item := range items {
			deletedObjects, err := planet.Satellites[0].Metainfo.Endpoint.DeleteObjectAnyStatus(ctx, metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: bucketName,
				ObjectKey:  item.ObjectKey,
			})
			require.NoError(t, err)
			require.Len(t, deletedObjects, 1)
		}

		items, err = planet.Satellites[0].Metabase.DB.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}

	testDeleteObject(t, createPendingObject, deleteAllPendingObjects)
}

func testDeleteObject(t *testing.T, createObject func(ctx context.Context, t *testing.T, planet *testplanet.Planet,
	data []byte), deleteAllObjects func(ctx context.Context, t *testing.T, planet *testplanet.Planet)) {
	t.Run("all nodes up", func(t *testing.T) {
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
			hasRemote       bool
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "one inline segment", objData: testrand.Bytes(3 * memory.KiB)},
			{caseDescription: "several segments (all remote)", objData: testrand.Bytes(50 * memory.KiB)},
			{caseDescription: "several segments (remote + inline)", objData: testrand.Bytes(33 * memory.KiB)},
		}

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.Combine(
					testplanet.ReconfigureRS(2, 2, 4, 4),
					testplanet.MaxSegmentSize(13*memory.KiB),
				),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			var (
				percentExp = 0.75
			)
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.caseDescription, func(t *testing.T) {

					createObject(ctx, t, planet, tc.objData)

					// calculate the SNs total used space after data upload
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					deleteAllObjects(ctx, t, planet)

					planet.WaitForStorageNodeDeleters(ctx)

					// calculate the SNs used space after delete the pieces
					var totalUsedSpaceAfterDelete int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpaceAfterDelete += piecesTotal
					}

					// At this point we can only guarantee that the 75% of the SNs pieces
					// are delete due to the success threshold
					deletedUsedSpace := float64(totalUsedSpace-totalUsedSpaceAfterDelete) / float64(totalUsedSpace)
					if deletedUsedSpace < percentExp {
						t.Fatalf("deleted used space is less than %f%%. Got %f", percentExp, deletedUsedSpace)
					}
				})
			}
		})

	})

	t.Run("some nodes down", func(t *testing.T) {
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "several segments (all remote)", objData: testrand.Bytes(50 * memory.KiB)},
			{caseDescription: "several segments (remote + inline)", objData: testrand.Bytes(33 * memory.KiB)},
		}

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.Combine(
					testplanet.ReconfigureRS(2, 2, 4, 4),
					testplanet.MaxSegmentSize(13*memory.KiB),
				),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			numToShutdown := 2

			for _, tc := range testCases {
				createObject(ctx, t, planet, tc.objData)
			}

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

			// Shutdown the first numToShutdown storage nodes before we delete the pieces
			// and collect used space values for those nodes
			snUsedSpace := make([]int64, len(planet.StorageNodes))
			for i := 0; i < numToShutdown; i++ {
				var err error
				snUsedSpace[i], _, err = planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
				require.NoError(t, err)

				require.NoError(t, planet.StopPeer(planet.StorageNodes[i]))
			}

			deleteAllObjects(ctx, t, planet)

			planet.WaitForStorageNodeDeleters(ctx)

			// Check that storage nodes that were offline when deleting the pieces
			// they are still holding data
			// Check that storage nodes which are online when deleting pieces don't
			// hold any piece
			// We are comparing used space from before deletion for nodes that were
			// offline, values for available nodes are 0
			for i, sn := range planet.StorageNodes {
				usedSpace, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
				require.NoError(t, err)

				require.Equal(t, snUsedSpace[i], usedSpace, "StorageNode #%d", i)
			}
		})
	})

	t.Run("all nodes down", func(t *testing.T) {
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "several segments (all remote)", objData: testrand.Bytes(50 * memory.KiB)},
			{caseDescription: "several segments (remote + inline)", objData: testrand.Bytes(33 * memory.KiB)},
		}

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.Combine(
					testplanet.ReconfigureRS(2, 2, 4, 4),
					testplanet.MaxSegmentSize(13*memory.KiB),
				),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			for _, tc := range testCases {
				createObject(ctx, t, planet, tc.objData)
			}

			// calculate the SNs total used space after data upload
			var usedSpaceBeforeDelete int64
			for _, sn := range planet.StorageNodes {
				piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
				require.NoError(t, err)
				usedSpaceBeforeDelete += piecesTotal
			}

			// Shutdown all the storage nodes before we delete the pieces
			for _, sn := range planet.StorageNodes {
				require.NoError(t, planet.StopPeer(sn))
			}

			deleteAllObjects(ctx, t, planet)

			// Check that storage nodes that were offline when deleting the pieces
			// they are still holding data
			var totalUsedSpace int64
			for _, sn := range planet.StorageNodes {
				piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
				require.NoError(t, err)
				totalUsedSpace += piecesTotal
			}

			require.Equal(t, usedSpaceBeforeDelete, totalUsedSpace, "totalUsedSpace")
		})
	})
}

func TestEndpoint_CopyObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		satelliteSys := planet.Satellites[0]
		uplnk := planet.Uplinks[0]

		// upload a small inline object
		err := uplnk.Upload(ctx, planet.Satellites[0], "testbucket", "testobject", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)
		objects, err := satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 1)

		getResp, err := satelliteSys.API.Metainfo.Endpoint.GetObject(ctx, &pb.ObjectGetRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:        []byte("testbucket"),
			EncryptedPath: []byte(objects[0].ObjectKey),
		})
		require.NoError(t, err)

		testEncryptedMetadataNonce := testrand.Nonce()
		// update the object metadata
		beginResp, err := satelliteSys.API.Metainfo.Endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:                getResp.Object.Bucket,
			EncryptedObjectKey:    getResp.Object.EncryptedPath,
			NewBucket:             []byte("testbucket"),
			NewEncryptedObjectKey: []byte("newencryptedkey"),
		})
		require.NoError(t, err)
		assert.Len(t, beginResp.SegmentKeys, 1)
		assert.Equal(t, beginResp.EncryptedMetadataKey, objects[0].EncryptedMetadataEncryptedKey)
		assert.Equal(t, beginResp.EncryptedMetadataKeyNonce.Bytes(), objects[0].EncryptedMetadataNonce)

		segmentKeys := pb.EncryptedKeyAndNonce{
			Position:          beginResp.SegmentKeys[0].Position,
			EncryptedKeyNonce: testrand.Nonce(),
			EncryptedKey:      []byte("newencryptedkey"),
		}

		_, err = satelliteSys.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.ObjectFinishCopyRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			StreamId:                     getResp.Object.StreamId,
			NewBucket:                    []byte("testbucket"),
			NewEncryptedObjectKey:        []byte("newobjectkey"),
			NewEncryptedMetadataKeyNonce: testEncryptedMetadataNonce,
			NewEncryptedMetadataKey:      []byte("encryptedmetadatakey"),
			NewSegmentKeys:               []*pb.EncryptedKeyAndNonce{&segmentKeys},
		})
		require.NoError(t, err)

		objectsAfterCopy, err := satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objectsAfterCopy, 2)

		getCopyResp, err := satelliteSys.API.Metainfo.Endpoint.GetObject(ctx, &pb.ObjectGetRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:        []byte("testbucket"),
			EncryptedPath: []byte("newobjectkey"),
		})
		require.NoError(t, err, objectsAfterCopy[1])
		require.NotEqual(t, getResp.Object.StreamId, getCopyResp.Object.StreamId)
		require.NotZero(t, getCopyResp.Object.StreamId)
		require.Equal(t, getResp.Object.InlineSize, getCopyResp.Object.InlineSize)

		// compare segments
		originalSegment, err := satelliteSys.API.Metainfo.Endpoint.DownloadSegment(ctx, &pb.SegmentDownloadRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			StreamId:       getResp.Object.StreamId,
			CursorPosition: segmentKeys.Position,
		})
		require.NoError(t, err)
		copiedSegment, err := satelliteSys.API.Metainfo.Endpoint.DownloadSegment(ctx, &pb.SegmentDownloadRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			StreamId:       getCopyResp.Object.StreamId,
			CursorPosition: segmentKeys.Position,
		})
		require.NoError(t, err)
		require.Equal(t, originalSegment.EncryptedInlineData, copiedSegment.EncryptedInlineData)
	})
}
