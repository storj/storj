// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/nodetag"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/time2"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/contact"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/object"
	"storj.io/uplink/private/testuplink"
)

func assertRPCStatusCode(t *testing.T, actualError error, expectedStatusCode rpcstatus.StatusCode) {
	statusCode := rpcstatus.Code(actualError)
	require.NotEqual(t, rpcstatus.Unknown, statusCode, "expected rpcstatus error, got \"%v\"", actualError)
	require.Equal(t, expectedStatusCode, statusCode, "wrong %T, got %v", statusCode, actualError)
}

func TestEndpoint_Object_No_StorageNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxObjectKeyLength(1024),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		satellite := planet.Satellites[0]

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

			// check version validation
			_, err := satellite.Metainfo.Endpoint.GetObject(ctx, &pb.GetObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte("test-bucket"),
				EncryptedObjectKey: []byte("test-object"),
				ObjectVersion:      []byte("broken-version"),
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

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

				response, err := satellite.Metainfo.Endpoint.GetObject(ctx, &pb.GetObjectRequest{
					Header: &pb.RequestHeader{
						ApiKey: apiKey.SerializeRaw(),
					},
					Bucket:             []byte(expectedBucketName),
					EncryptedObjectKey: item.EncryptedObjectKey,
				})
				require.NoError(t, err)
				require.Equal(t, item.EncryptedObjectKey, response.Object.EncryptedObjectKey)
				require.NotEmpty(t, response.Object.StreamId)

				// get with version
				response, err = satellite.Metainfo.Endpoint.GetObject(ctx, &pb.GetObjectRequest{
					Header: &pb.RequestHeader{
						ApiKey: apiKey.SerializeRaw(),
					},
					Bucket:             []byte(expectedBucketName),
					EncryptedObjectKey: item.EncryptedObjectKey,
					ObjectVersion:      response.Object.ObjectVersion,
				})
				require.NoError(t, err)
				require.Equal(t, item.EncryptedObjectKey, response.Object.EncryptedObjectKey)

				// get with WRONG version, should return error
				object := metabase.Object{}
				object.Version = metabase.Version(response.Object.Version) + 10
				copy(object.StreamID[:], response.Object.StreamId[:])
				_, err = satellite.Metainfo.Endpoint.GetObject(ctx, &pb.GetObjectRequest{
					Header: &pb.RequestHeader{
						ApiKey: apiKey.SerializeRaw(),
					},
					Bucket:             []byte(expectedBucketName),
					EncryptedObjectKey: item.EncryptedObjectKey,
					ObjectVersion:      object.StreamVersionID().Bytes(),
				})
				require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
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

			expected := []metaclient.Object{
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

			expected = []metaclient.Object{
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
				EncryptedMetadataEncryptedKey: randomEncryptedKey,
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
				EncryptedMetadataEncryptedKey: randomEncryptedKey,
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
				Bucket:             []byte("testbucket"),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)

			testEncryptedMetadata := testrand.BytesInt(64)
			testEncryptedMetadataEncryptedKey := randomEncryptedKey
			testEncryptedMetadataNonce := testrand.Nonce()

			// update the object metadata
			_, err = satelliteSys.API.Metainfo.Endpoint.UpdateObjectMetadata(ctx, &pb.ObjectUpdateMetadataRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:                        getResp.Object.Bucket,
				EncryptedObjectKey:            getResp.Object.EncryptedObjectKey,
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
			satStreamID := &internalpb.StreamID{
				Bucket:             []byte(expectedBucketName),
				EncryptedObjectKey: []byte("bad path"),
				StreamId:           streamUUID[:],
				CreationDate:       time.Now(),
			}
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

		t.Run("get object", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			err := planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "object", testrand.Bytes(256))
			require.NoError(t, err)

			objects, err := satellite.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			committedObject := objects[0]

			pendingObject, err := satellite.API.Metainfo.Metabase.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  committedObject.ProjectID,
					BucketName: committedObject.BucketName,
					ObjectKey:  committedObject.ObjectKey,
					StreamID:   committedObject.StreamID,
				},
			})
			require.NoError(t, err)
			require.Equal(t, committedObject.Version+1, pendingObject.Version)

			getObjectResponse, err := satellite.API.Metainfo.Endpoint.GetObject(ctx, &pb.ObjectGetRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("testbucket"),
				EncryptedObjectKey: []byte(committedObject.ObjectKey),
			})
			require.NoError(t, err)
			require.EqualValues(t, committedObject.BucketName, getObjectResponse.Object.Bucket)
			require.EqualValues(t, committedObject.ObjectKey, getObjectResponse.Object.EncryptedObjectKey)
		})

		t.Run("download object", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			err := planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "object", testrand.Bytes(256))
			require.NoError(t, err)

			objects, err := satellite.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			committedObject := objects[0]

			pendingObject, err := satellite.API.Metainfo.Metabase.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  committedObject.ProjectID,
					BucketName: committedObject.BucketName,
					ObjectKey:  committedObject.ObjectKey,
					StreamID:   committedObject.StreamID,
				},
			})
			require.NoError(t, err)
			require.Equal(t, committedObject.Version+1, pendingObject.Version)

			downloadObjectResponse, err := satellite.API.Metainfo.Endpoint.DownloadObject(ctx, &pb.ObjectDownloadRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("testbucket"),
				EncryptedObjectKey: []byte(committedObject.ObjectKey),
			})
			require.NoError(t, err)
			require.EqualValues(t, committedObject.BucketName, downloadObjectResponse.Object.Bucket)
			require.EqualValues(t, committedObject.ObjectKey, downloadObjectResponse.Object.EncryptedObjectKey)
		})

		t.Run("begin expired object", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
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
				ExpiresAt: time.Now().Add(-24 * time.Hour),
			}

			_, err = metainfoClient.BeginObject(ctx, params)
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid expiration time")
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
		})

		t.Run("UploadID check", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			_, err = project.CreateBucket(ctx, bucketName)
			require.NoError(t, err)

			for _, tt := range []struct {
				expires time.Time
				options uplink.ListUploadsOptions
			}{
				{
					options: uplink.ListUploadsOptions{System: false, Custom: false},
				},
				{
					options: uplink.ListUploadsOptions{System: true, Custom: false},
				},
				{
					options: uplink.ListUploadsOptions{System: true, Custom: true},
				},
				{
					options: uplink.ListUploadsOptions{System: false, Custom: true},
				},
				{
					expires: time.Now().Add(24 * time.Hour),
					options: uplink.ListUploadsOptions{System: false, Custom: false},
				},
				{
					expires: time.Now().Add(24 * time.Hour),
					options: uplink.ListUploadsOptions{System: true, Custom: false},
				},
				{
					expires: time.Now().Add(24 * time.Hour),
					options: uplink.ListUploadsOptions{System: true, Custom: true},
				},
				{
					expires: time.Now().Add(24 * time.Hour),
					options: uplink.ListUploadsOptions{System: false, Custom: true},
				},
			} {
				t.Run(fmt.Sprintf("expires:%v;system:%v;custom:%v", !tt.expires.IsZero(), tt.options.System, tt.options.Custom), func(t *testing.T) {
					uploadInfo, err := project.BeginUpload(ctx, bucketName, "multipart-object", &uplink.UploadOptions{
						Expires: tt.expires,
					})
					require.NoError(t, err)

					iterator := project.ListUploads(ctx, bucketName, &tt.options)
					require.True(t, iterator.Next())
					require.Equal(t, uploadInfo.UploadID, iterator.Item().UploadID)
					require.NoError(t, iterator.Err())

					err = project.AbortUpload(ctx, bucketName, "multipart-object", iterator.Item().UploadID)
					require.NoError(t, err)
				})
			}
		})

		t.Run("download specific version", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			expectedData := testrand.Bytes(256)
			err := planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "object", expectedData)
			require.NoError(t, err)

			objects, err := satellite.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			committedObject := objects[0]

			// download without specifying version
			downloadObjectResponse, err := satellite.API.Metainfo.Endpoint.DownloadObject(ctx, &pb.ObjectDownloadRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("testbucket"),
				EncryptedObjectKey: []byte(committedObject.ObjectKey),
			})
			require.NoError(t, err)
			require.EqualValues(t, committedObject.BucketName, downloadObjectResponse.Object.Bucket)
			require.EqualValues(t, committedObject.ObjectKey, downloadObjectResponse.Object.EncryptedObjectKey)
			require.EqualValues(t, committedObject.StreamVersionID().Bytes(), downloadObjectResponse.Object.ObjectVersion)

			// download using explicit version
			downloadObjectResponse, err = satellite.API.Metainfo.Endpoint.DownloadObject(ctx, &pb.ObjectDownloadRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("testbucket"),
				EncryptedObjectKey: []byte(committedObject.ObjectKey),
				ObjectVersion:      committedObject.StreamVersionID().Bytes(),
			})
			require.NoError(t, err)
			require.EqualValues(t, committedObject.BucketName, downloadObjectResponse.Object.Bucket)
			require.EqualValues(t, committedObject.ObjectKey, downloadObjectResponse.Object.EncryptedObjectKey)
			require.EqualValues(t, committedObject.StreamVersionID().Bytes(), downloadObjectResponse.Object.ObjectVersion)

			// download using NON EXISTING version
			nonExistingObject := committedObject
			nonExistingObject.Version++
			_, err = satellite.API.Metainfo.Endpoint.DownloadObject(ctx, &pb.ObjectDownloadRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("testbucket"),
				EncryptedObjectKey: []byte(committedObject.ObjectKey),
				ObjectVersion:      nonExistingObject.StreamVersionID().Bytes(),
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		})

		t.Run("delete specific version", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "test-object", testrand.Bytes(100))
			require.NoError(t, err)

			// get encrypted object key and version
			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)

			endpoint := planet.Satellites[0].Metainfo.Endpoint

			// first try to delete not existing version
			nonExistingObject := objects[0]
			nonExistingObject.Version++
			response, err := endpoint.BeginDeleteObject(ctx, &pb.BeginDeleteObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				ObjectVersion:      nonExistingObject.StreamVersionID().Bytes(),
			})
			require.NoError(t, err)
			require.Nil(t, response.Object)

			// now delete using explicit version
			response, err = endpoint.BeginDeleteObject(ctx, &pb.BeginDeleteObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				ObjectVersion:      objects[0].StreamVersionID().Bytes(),
			})
			require.NoError(t, err)
			require.NotNil(t, response.Object)
			require.EqualValues(t, objects[0].ObjectKey, response.Object.EncryptedObjectKey)

			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "test-object", testrand.Bytes(100))
			require.NoError(t, err)

			// now delete using empty version (latest version)
			response, err = endpoint.BeginDeleteObject(ctx, &pb.BeginDeleteObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				ObjectVersion:      nil,
			})
			require.NoError(t, err)
			require.NotNil(t, response.Object)
			require.EqualValues(t, objects[0].ObjectKey, response.Object.EncryptedObjectKey)

			objects, err = planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Empty(t, objects)
		})
	})
}

func TestEndpoint_Object_Limit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UploadLimiter.SingleObjectLimit = 200 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		sat := planet.Satellites[0]
		endpoint := sat.Metainfo.Endpoint

		bucketName := "testbucket"

		project, err := sat.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.NotNil(t, project)

		err = planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		limit := 2 * memory.KB
		project.StorageLimit = &limit
		project.BandwidthLimit = &limit
		err = sat.DB.Console().Projects().Update(ctx, project)
		assert.NoError(t, err)

		t.Run("limit single object upload", func(t *testing.T) {

			now := time.Now()
			request := &pb.BeginObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("single-object"),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
				},
			}
			// upload to the same location one by one should fail
			_, err := endpoint.BeginObject(ctx, request)
			require.NoError(t, err)

			_, err = endpoint.BeginObject(ctx, request)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			ctx, _ := time2.WithNewMachine(ctx, time2.WithTimeAt(now.Add(250*time.Millisecond)))

			_, err = endpoint.BeginObject(ctx, request)
			require.NoError(t, err)

			// upload to different locations one by one should NOT fail
			request.EncryptedObjectKey = []byte("single-objectA")
			_, err = endpoint.BeginObject(ctx, request)
			require.NoError(t, err)

			request.EncryptedObjectKey = []byte("single-objectB")
			_, err = endpoint.BeginObject(ctx, request)
			require.NoError(t, err)
		})

		t.Run("user specified limit upload", func(t *testing.T) {
			request := &pb.BeginObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("some-object"),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
				},
			}
			_, err = endpoint.BeginObject(ctx, request)
			require.NoError(t, err)

			// zero user specified storage limit
			project.UserSpecifiedStorageLimit = new(memory.Size)
			err = sat.DB.Console().Projects().Update(ctx, project)
			assert.NoError(t, err)

			request.EncryptedObjectKey = []byte("another-object")
			// must fail because user specified storage limit is zero
			_, err = endpoint.BeginObject(ctx, request)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			project.UserSpecifiedStorageLimit = project.StorageLimit
			err = sat.DB.Console().Projects().Update(ctx, project)
			assert.NoError(t, err)

			request.EncryptedObjectKey = []byte("yet-another-object")
			_, err = endpoint.BeginObject(ctx, request)
			require.NoError(t, err)
		})

		t.Run("user specified limit download", func(t *testing.T) {
			err = planet.Uplinks[0].Upload(ctx, sat, bucketName, "some-object", testrand.Bytes(100))
			require.NoError(t, err)

			objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, objects)

			request := &pb.DownloadObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			}
			_, err = endpoint.DownloadObject(ctx, request)
			require.NoError(t, err)

			// zero user specified bandwidth limit
			project.UserSpecifiedBandwidthLimit = new(memory.Size)
			err = sat.DB.Console().Projects().Update(ctx, project)
			assert.NoError(t, err)

			// must fail because user specified bandwidth limit is zero
			_, err = endpoint.DownloadObject(ctx, request)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			project.UserSpecifiedBandwidthLimit = project.BandwidthLimit
			err = sat.DB.Console().Projects().Update(ctx, project)
			assert.NoError(t, err)

			_, err = endpoint.DownloadObject(ctx, request)
			require.NoError(t, err)
		})
	})
}

func TestEndpoint_BeginObject_MaxObjectTTL(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		endpoint := planet.Satellites[0].Metainfo.Endpoint

		bucketName := "testbucket"

		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		t.Run("object upload with max object ttl", func(t *testing.T) {
			now := time.Now()

			zero := 0 * time.Hour
			oneHour := time.Hour
			minusOneHour := -oneHour

			type TestCases struct {
				maxObjectTTL       *time.Duration
				expiresAt          time.Time
				expectedExpiration time.Time
				expectedErr        bool
			}

			for _, tc := range []TestCases{
				{
					maxObjectTTL:       nil,
					expiresAt:          time.Time{},
					expectedExpiration: time.Time{},
					expectedErr:        false,
				},
				{
					maxObjectTTL:       &oneHour,
					expiresAt:          time.Time{},
					expectedExpiration: now.Add(time.Hour),
					expectedErr:        false,
				},
				{
					maxObjectTTL:       &oneHour,
					expiresAt:          now.Add(30 * time.Minute),
					expectedExpiration: now.Add(30 * time.Minute),
					expectedErr:        false,
				},
				{
					maxObjectTTL:       &oneHour,
					expiresAt:          now.Add(2 * time.Hour),
					expectedExpiration: time.Time{},
					expectedErr:        true,
				},
				{
					maxObjectTTL:       &zero,
					expiresAt:          time.Time{},
					expectedExpiration: time.Time{},
					expectedErr:        true,
				},
				{
					maxObjectTTL:       &minusOneHour,
					expiresAt:          time.Time{},
					expectedExpiration: time.Time{},
					expectedErr:        true,
				},
			} {
				t.Run("", func(t *testing.T) {
					restrictedAPIKey := apiKey
					if tc.maxObjectTTL != nil {
						restrictedAPIKey, err = restrictedAPIKey.Restrict(macaroon.Caveat{
							MaxObjectTtl: tc.maxObjectTTL,
						})
					}
					require.NoError(t, err)

					objectKey := testrand.Bytes(10)

					beginResp, err := endpoint.BeginObject(ctx, &pb.BeginObjectRequest{
						Header: &pb.RequestHeader{
							ApiKey: restrictedAPIKey.SerializeRaw(),
						},
						Bucket:             []byte(bucketName),
						EncryptedObjectKey: objectKey,
						ExpiresAt:          tc.expiresAt,
						EncryptionParameters: &pb.EncryptionParameters{
							CipherSuite: pb.CipherSuite_ENC_AESGCM,
						},
					})
					if tc.expectedErr {
						require.Error(t, err)
						return
					}
					require.NoError(t, err)

					satStreamID := &internalpb.StreamID{}
					err = pb.Unmarshal(beginResp.StreamId, satStreamID)
					require.NoError(t, err)
					require.WithinDuration(t, tc.expectedExpiration, satStreamID.ExpirationDate, time.Minute)

					listResp, err := endpoint.ListPendingObjectStreams(ctx, &pb.ListPendingObjectStreamsRequest{
						Header: &pb.RequestHeader{
							ApiKey: restrictedAPIKey.SerializeRaw(),
						},
						Bucket:             []byte(bucketName),
						EncryptedObjectKey: objectKey,
					})
					require.NoError(t, err)
					require.Len(t, listResp.Items, 1)
					require.WithinDuration(t, tc.expectedExpiration, listResp.Items[0].ExpiresAt, time.Minute)
				})
			}
		})
	})
}

// TODO remove when listing query tests feature flag is removed.
func TestEndpoint_Object_No_StorageNodes_TestListingQuery(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(testplanet.MaxObjectKeyLength(1024), func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.TestListingQuery = true
			}),
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

		t.Run("list service with listing query test", func(t *testing.T) {
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

			expected := []metaclient.Object{
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

			expected = []metaclient.Object{
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

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

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

			bucketsService := planet.Satellites[0].API.Buckets.Service

			bucket := buckets.Bucket{
				Name:      bucketName,
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Placement: storj.EU,
			}

			_, err := bucketsService.CreateBucket(ctx, bucket)
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
				Position: metaclient.SegmentPosition{
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
				Encryption: metaclient.SegmentEncryption{
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

			endpoint := planet.Satellites[0].Metainfo.Endpoint
			coResponse, err := endpoint.CommitObject(ctx, &pb.CommitObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:                      beginObjectResponse.StreamID,
				EncryptedMetadata:             metadata,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: randomEncryptedKey,
			})
			require.NoError(t, err)
			require.NotNil(t, coResponse.Object)
			require.NotEmpty(t, coResponse.Object.ObjectVersion)

			// TODO(ver): add tests more detailed tests for returning object on commit, including returned version

			listResponse, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket: []byte(bucket.Name),
				ObjectIncludes: &pb.ObjectListItemIncludes{
					ExcludeSystemMetadata: false,
					Metadata:              false,
				},
			})

			require.NoError(t, err)
			require.Len(t, listResponse.Items, 1)
			require.Equal(t, params.EncryptedObjectKey, listResponse.Items[0].EncryptedObjectKey)
			require.Equal(t, params.ExpiresAt.Truncate(time.Millisecond), params.ExpiresAt.Truncate(time.Millisecond))
			require.Equal(t, coResponse.Object.ObjectVersion, listResponse.Items[0].ObjectVersion)

			allObjects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, allObjects, 1)
			require.Equal(t, listResponse.Items[0].ObjectVersion, allObjects[0].StreamVersionID().Bytes())
		})

		t.Run("get object IP", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
			uplnk := planet.Uplinks[0]
			uplinkCtx := testuplink.WithMaxSegmentSize(ctx, 5*memory.KB)
			sat := planet.Satellites[0]

			require.NoError(t, uplnk.CreateBucket(uplinkCtx, sat, bucketName))
			require.NoError(t, uplnk.Upload(uplinkCtx, sat, bucketName, "jones", testrand.Bytes(20*memory.KB)))

			jonesSegments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)

			project, err := uplnk.OpenProject(ctx, planet.Satellites[0])
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			// make a copy
			_, err = project.CopyObject(ctx, bucketName, "jones", bucketName, "jones_copy", nil)
			require.NoError(t, err)

			ips, err := object.GetObjectIPs(ctx, uplink.Config{}, access, bucketName, "jones")
			require.NoError(t, err)
			require.True(t, len(ips) > 0)

			copyIPs, err := object.GetObjectIPs(ctx, uplink.Config{}, access, bucketName, "jones_copy")
			require.NoError(t, err)

			// verify that orignal and copy has the same results
			require.ElementsMatch(t, ips, copyIPs)

			expectedIPsMap := map[string]struct{}{}
			for _, segment := range jonesSegments {
				for _, piece := range segment.Pieces {
					node, err := planet.Satellites[0].Overlay.Service.Get(ctx, piece.StorageNode)
					require.NoError(t, err)
					expectedIPsMap[node.LastIPPort] = struct{}{}
				}
			}

			expectedIPs := [][]byte{}
			for _, ip := range maps.Keys(expectedIPsMap) {
				expectedIPs = append(expectedIPs, []byte(ip))
			}
			require.ElementsMatch(t, expectedIPs, ips)

			// set bucket geofencing
			_, err = planet.Satellites[0].DB.Buckets().UpdateBucket(ctx, buckets.Bucket{
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Name:      bucketName,
				Placement: storj.EU,
			})
			require.NoError(t, err)

			// set one node to US to filter it out from IP results
			usNode := planet.FindNode(jonesSegments[0].Pieces[0].StorageNode)
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestNodeCountryCode(ctx, usNode.ID(), "US"))
			require.NoError(t, planet.Satellites[0].API.Overlay.Service.DownloadSelectionCache.Refresh(ctx))

			geoFencedIPs, err := object.GetObjectIPs(ctx, uplink.Config{}, access, bucketName, "jones")
			require.NoError(t, err)

			require.Len(t, geoFencedIPs, len(expectedIPs)-1)
			for _, ip := range geoFencedIPs {
				if string(ip) == usNode.Addr() {
					t.Fatal("this IP should be removed from results because of geofencing")
				}
			}
		})

		t.Run("get object IP with same location committed and pending status", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
			uplnk := planet.Uplinks[0]
			sat := planet.Satellites[0]

			require.NoError(t, uplnk.Upload(ctx, sat, bucketName, "jones", testrand.Bytes(20*memory.KB)))

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

			objects, err := sat.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			committedObject := objects[0]

			pendingObject, err := sat.Metabase.DB.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  committedObject.ProjectID,
					BucketName: committedObject.BucketName,
					ObjectKey:  committedObject.ObjectKey,
					StreamID:   committedObject.StreamID,
				},
			})
			require.NoError(t, err)
			require.Equal(t, committedObject.Version+1, pendingObject.Version)

			newIps, err := object.GetObjectIPs(ctx, uplink.Config{}, access, bucketName, "jones")
			require.NoError(t, err)

			sort.Slice(ips, func(i, j int) bool {
				return bytes.Compare(ips[i], ips[j]) < 0
			})
			sort.Slice(newIps, func(i, j int) bool {
				return bytes.Compare(newIps[i], newIps[j]) < 0
			})
			require.Equal(t, ips, newIps)
		})

		t.Run("get object IP with version != 1", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
			uplnk := planet.Uplinks[0]
			sat := planet.Satellites[0]

			require.NoError(t, uplnk.Upload(ctx, sat, bucketName, "jones", testrand.Bytes(20*memory.KB)))

			objects, err := sat.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)

			committedObject := objects[0]
			randomVersion := metabase.Version(2 + testrand.Intn(9))

			// atm there's no better way to change object's version
			res, err := planet.Satellites[0].Metabase.DB.UnderlyingTagSQL().Exec(ctx,
				"UPDATE objects SET version = $1 WHERE project_id = $2 AND bucket_name = $3 AND object_key = $4 AND stream_id = $5",
				randomVersion, committedObject.ProjectID, committedObject.BucketName, committedObject.ObjectKey, committedObject.StreamID,
			)
			require.NoError(t, err)

			affected, err := res.RowsAffected()
			require.NoError(t, err)
			require.EqualValues(t, 1, affected)

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
			defer ctx.Check(deleteBucket("pip-a"))
			defer ctx.Check(deleteBucket("pip-b"))
			defer ctx.Check(deleteBucket("pip-c"))

			data := testrand.Bytes(20 * memory.KB)
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "pip-a", "non-multipart-object", data)
			require.NoError(t, err)

			project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			_, err = project.EnsureBucket(ctx, "pip-b")
			require.NoError(t, err)
			info, err := project.BeginUpload(ctx, "pip-b", "multipart-object", nil)
			require.NoError(t, err)
			upload, err := project.UploadPart(ctx, "pip-b", "multipart-object", info.UploadID, 1)
			require.NoError(t, err)
			_, err = upload.Write(data)
			require.NoError(t, err)
			require.NoError(t, upload.Commit())
			_, err = project.CommitUpload(ctx, "pip-b", "multipart-object", info.UploadID, nil)
			require.NoError(t, err)

			_, err = project.EnsureBucket(ctx, "pip-c")
			require.NoError(t, err)
			info, err = project.BeginUpload(ctx, "pip-c", "multipart-object-third", nil)
			require.NoError(t, err)
			for i := 0; i < 4; i++ {
				upload, err := project.UploadPart(ctx, "pip-c", "multipart-object-third", info.UploadID, uint32(i+1))
				require.NoError(t, err)
				_, err = upload.Write(data)
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}
			_, err = project.CommitUpload(ctx, "pip-c", "multipart-object-third", info.UploadID, nil)
			require.NoError(t, err)

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 3)
			slices.SortFunc(objects, func(a, b metabase.Object) int {
				return strings.Compare(a.BucketName, b.BucketName)
			})

			// verify that standard objects can be downloaded in an old way (index = -1 as last segment)
			object, err := metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte(objects[0].BucketName), // pip-a
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)
			_, err = metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: metaclient.SegmentPosition{
					Index: -1,
				},
			})
			require.NoError(t, err)

			// verify that multipart objects (single segment) CANNOT be downloaded in an old way (index = -1 as last segment)
			object, err = metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte(objects[1].BucketName), // pip-b
				EncryptedObjectKey: []byte(objects[1].ObjectKey),
			})
			require.NoError(t, err)
			_, err = metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: metaclient.SegmentPosition{
					Index: -1,
				},
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "Used uplink version cannot download multipart objects.")

			// verify that multipart objects (multiple segments) CANNOT be downloaded in an old way (index = -1 as last segment)
			object, err = metainfoClient.GetObject(ctx, metaclient.GetObjectParams{
				Bucket:             []byte(objects[2].BucketName), // pip-c
				EncryptedObjectKey: []byte(objects[2].ObjectKey),
			})
			require.NoError(t, err)
			_, err = metainfoClient.DownloadSegmentWithRS(ctx, metaclient.DownloadSegmentParams{
				StreamID: object.StreamID,
				Position: metaclient.SegmentPosition{
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
			t.Log(apiKey)
			bucketsService := planet.Satellites[0].API.Buckets.Service

			bucket := buckets.Bucket{
				Name:      bucketName,
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Placement: storj.EU,
			}
			_, err := bucketsService.CreateBucket(ctx, bucket)
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

		t.Run("multiple versions", func(t *testing.T) {
			defer ctx.Check(deleteBucket("multipleversions"))

			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "object", testrand.Bytes(10*memory.MiB))
			require.NoError(t, err)

			// override object to have it with version 2
			expectedData := testrand.Bytes(11 * memory.KiB)
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "object", expectedData)
			require.NoError(t, err)

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)
			require.EqualValues(t, 2, objects[0].Version)

			// add some pending uploads, each will have version higher then 2
			uploadIDs := []string{}
			for i := 0; i < 10; i++ {
				info, err := project.BeginUpload(ctx, "multipleversions", "object", nil)
				require.NoError(t, err)
				uploadIDs = append(uploadIDs, info.UploadID)
			}

			checkDownload := func(objectKey string, expectedData []byte) {
				data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "multipleversions", objectKey)
				require.NoError(t, err)
				require.Equal(t, expectedData, data)
			}

			checkDownload("object", expectedData)

			err = project.MoveObject(ctx, "multipleversions", "object", "multipleversions", "object_moved", nil)
			require.NoError(t, err)

			checkDownload("object_moved", expectedData)

			err = project.MoveObject(ctx, "multipleversions", "object_moved", "multipleversions", "object", nil)
			require.NoError(t, err)

			checkDownload("object", expectedData)

			iterator := project.ListObjects(ctx, "multipleversions", nil)
			require.True(t, iterator.Next())
			require.Equal(t, "object", iterator.Item().Key)
			require.NoError(t, iterator.Err())

			// upload multipleversions/object once again as we just moved it
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "object", expectedData)
			require.NoError(t, err)

			checkDownload("object", expectedData)

			{ // server side copy
				_, err = project.CopyObject(ctx, "multipleversions", "object", "multipleversions", "object_copy", nil)
				require.NoError(t, err)

				checkDownload("object_copy", expectedData)

				_, err = project.DeleteObject(ctx, "multipleversions", "object")
				require.NoError(t, err)

				_, err = project.CopyObject(ctx, "multipleversions", "object_copy", "multipleversions", "object", nil)
				require.NoError(t, err)

				checkDownload("object", expectedData)

				_, err = project.DeleteObject(ctx, "multipleversions", "object_copy")
				require.NoError(t, err)

				checkDownload("object", expectedData)
			}

			err = project.AbortUpload(ctx, "multipleversions", "object", uploadIDs[0])
			require.NoError(t, err)
			checkDownload("object", expectedData)

			expectedData = testrand.Bytes(12 * memory.KiB)
			upload, err := project.UploadPart(ctx, "multipleversions", "object", uploadIDs[1], 1)
			require.NoError(t, err)
			_, err = upload.Write(expectedData)
			require.NoError(t, err)
			require.NoError(t, upload.Commit())
			_, err = project.CommitUpload(ctx, "multipleversions", "object", uploadIDs[1], nil)
			require.NoError(t, err)

			checkDownload("object", expectedData)

			_, err = project.DeleteObject(ctx, "multipleversions", "object")
			require.NoError(t, err)

			_, err = project.DeleteObject(ctx, "multipleversions", "object_moved")
			require.NoError(t, err)

			iterator = project.ListObjects(ctx, "multipleversions", nil)
			require.False(t, iterator.Next())
			require.NoError(t, iterator.Err())

			// use next available pending upload
			upload, err = project.UploadPart(ctx, "multipleversions", "object", uploadIDs[2], 1)
			require.NoError(t, err)
			_, err = upload.Write(expectedData)
			require.NoError(t, err)
			require.NoError(t, upload.Commit())
			_, err = project.CommitUpload(ctx, "multipleversions", "object", uploadIDs[2], nil)
			require.NoError(t, err)

			checkDownload("object", expectedData)

			uploads := project.ListUploads(ctx, "multipleversions", nil)
			count := 0
			for uploads.Next() {
				require.Equal(t, "object", uploads.Item().Key)
				count++
			}
			// we started with 10 pending object and during test we abort/commit 3 objects
			pendingUploadsLeft := 7
			require.Equal(t, pendingUploadsLeft, count)
		})

		t.Run("override object", func(t *testing.T) {
			defer ctx.Check(deleteBucket("bucket"))

			bucketName := "bucket"
			objectName := "file1"

			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, objectName, testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)

			expectedData := testrand.Bytes(5 * memory.KiB)
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, objectName, expectedData)
			require.NoError(t, err)

			data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], bucketName, objectName)
			require.NoError(t, err)
			require.Equal(t, expectedData, data)
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

func createGeofencedBucket(t *testing.T, ctx *testcontext.Context, service *buckets.Service, projectID uuid.UUID, bucketName string, placement storj.PlacementConstraint) {
	// generate the bucket id
	bucketID, err := uuid.New()
	require.NoError(t, err)

	// create the bucket
	_, err = service.CreateBucket(ctx, buckets.Bucket{
		ID:        bucketID,
		Name:      bucketName,
		ProjectID: projectID,
		Placement: placement,
	})
	require.NoError(t, err)

	// check that the bucket placement is correct
	bucket, err := service.GetBucket(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	require.Equal(t, placement, bucket.Placement)
}

func TestEndpoint_DeleteCommittedObject(t *testing.T) {
	createObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, key string, data []byte) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucket, key, data)
		require.NoError(t, err)
	}
	deleteObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, encryptedKey string, streamID uuid.UUID) {
		projectID := planet.Uplinks[0].Projects[0].ID

		_, err := planet.Satellites[0].Metainfo.Endpoint.DeleteCommittedObject(ctx, projectID, bucket, metabase.ObjectKey(encryptedKey), []byte{})
		require.NoError(t, err)
	}
	testDeleteObject(t, createObject, deleteObject)
}

func testDeleteObject(t *testing.T,
	createObject func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, key string, data []byte),
	deleteObject func(ctx context.Context, t *testing.T, planet *testplanet.Planet, bucket, encryptedKey string, streamID uuid.UUID),
) {
	bucketName := "deleteobjects"
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
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.caseDescription, func(t *testing.T) {

					createObject(ctx, t, planet, bucketName, tc.caseDescription, tc.objData)

					// calculate the SNs total used space after data upload
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
					require.NoError(t, err)
					for _, object := range objects {
						deleteObject(ctx, t, planet, bucketName, string(object.ObjectKey), object.StreamID)
					}

					planet.WaitForStorageNodeDeleters(ctx)

					// calculate the SNs used space after delete the pieces
					var totalUsedSpaceAfterDelete int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpaceAfterDelete += piecesTotal
					}

					// we are not deleting data from SN right away so used space should be the same
					require.Equal(t, totalUsedSpace, totalUsedSpaceAfterDelete)
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
				createObject(ctx, t, planet, bucketName, tc.caseDescription, tc.objData)
			}

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

			// Shutdown the first numToShutdown storage nodes before we delete the pieces
			// and collect used space values for those nodes
			snUsedSpace := make([]int64, len(planet.StorageNodes))
			for i, node := range planet.StorageNodes {
				var err error
				snUsedSpace[i], _, err = node.Storage2.Store.SpaceUsedForPieces(ctx)
				require.NoError(t, err)

				if i < numToShutdown {
					require.NoError(t, planet.StopPeer(node))
				}
			}

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, object := range objects {
				deleteObject(ctx, t, planet, bucketName, string(object.ObjectKey), object.StreamID)
			}

			planet.WaitForStorageNodeDeleters(ctx)

			// we are not deleting data from SN right away so used space should be the same
			// for online and shutdown/offline node
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
				createObject(ctx, t, planet, bucketName, tc.caseDescription, tc.objData)
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

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, object := range objects {
				deleteObject(ctx, t, planet, bucketName, string(object.ObjectKey), object.StreamID)
			}

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
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 4,
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
			Bucket:             []byte("testbucket"),
			EncryptedObjectKey: []byte(objects[0].ObjectKey),
		})
		require.NoError(t, err)

		testEncryptedMetadataNonce := testrand.Nonce()
		// update the object metadata
		beginResp, err := satelliteSys.API.Metainfo.Endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:                getResp.Object.Bucket,
			EncryptedObjectKey:    getResp.Object.EncryptedObjectKey,
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

		{
			// metadata too large
			_, err = satelliteSys.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.ObjectFinishCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:                     getResp.Object.StreamId,
				NewBucket:                    []byte("testbucket"),
				NewEncryptedObjectKey:        []byte("newobjectkey"),
				NewEncryptedMetadata:         testrand.Bytes(satelliteSys.Config.Metainfo.MaxMetadataSize + 1),
				NewEncryptedMetadataKeyNonce: testEncryptedMetadataNonce,
				NewEncryptedMetadataKey:      []byte("encryptedmetadatakey"),
				NewSegmentKeys:               []*pb.EncryptedKeyAndNonce{&segmentKeys},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

			// invalid encrypted metadata key
			_, err = satelliteSys.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.ObjectFinishCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:                     getResp.Object.StreamId,
				NewBucket:                    []byte("testbucket"),
				NewEncryptedObjectKey:        []byte("newobjectkey"),
				NewEncryptedMetadata:         testrand.Bytes(satelliteSys.Config.Metainfo.MaxMetadataSize),
				NewEncryptedMetadataKeyNonce: testEncryptedMetadataNonce,
				NewEncryptedMetadataKey:      []byte("encryptedmetadatakey"),
				NewSegmentKeys:               []*pb.EncryptedKeyAndNonce{&segmentKeys},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
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
			Bucket:             []byte("testbucket"),
			EncryptedObjectKey: []byte("newobjectkey"),
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

		{ // test copy respects project storage size limit
			// set storage limit
			err = planet.Satellites[0].DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, planet.Uplinks[1].Projects[0].ID, 1000)
			require.NoError(t, err)

			// test object below the limit when copied
			err = planet.Uplinks[1].Upload(ctx, planet.Satellites[0], "testbucket", "testobject", testrand.Bytes(100))
			require.NoError(t, err)
			objects, err = satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)

			_, err = satelliteSys.API.Metainfo.Endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: planet.Uplinks[1].APIKey[planet.Satellites[0].ID()].SerializeRaw(),
				},
				Bucket:                []byte("testbucket"),
				EncryptedObjectKey:    []byte(objects[0].ObjectKey),
				NewBucket:             []byte("testbucket"),
				NewEncryptedObjectKey: []byte("newencryptedobjectkey"),
			})
			require.NoError(t, err)
			err = satelliteSys.API.Metainfo.Metabase.TestingDeleteAll(ctx)
			require.NoError(t, err)

			// set storage limit
			err = planet.Satellites[0].DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, planet.Uplinks[2].Projects[0].ID, 1000)
			require.NoError(t, err)

			// test object exceeding the limit when copied
			err = planet.Uplinks[2].Upload(ctx, planet.Satellites[0], "testbucket", "testobject", testrand.Bytes(400))
			require.NoError(t, err)
			objects, err = satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)

			err = planet.Uplinks[2].CopyObject(ctx, planet.Satellites[0], "testbucket", "testobject", "testbucket", "testobject1")
			require.NoError(t, err)

			_, err = satelliteSys.API.Metainfo.Endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: planet.Uplinks[2].APIKey[planet.Satellites[0].ID()].SerializeRaw(),
				},
				Bucket:                []byte("testbucket"),
				EncryptedObjectKey:    []byte(objects[0].ObjectKey),
				NewBucket:             []byte("testbucket"),
				NewEncryptedObjectKey: []byte("newencryptedobjectkey"),
			})
			assertRPCStatusCode(t, err, rpcstatus.ResourceExhausted)
			assert.EqualError(t, err, "Exceeded Storage Limit")

			// metabaseObjects, err := satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
			// require.NoError(t, err)
			// metabaseObj := metabaseObjects[0]

			// randomEncKey := testrand.Key()

			// somehow triggers error "proto: can't skip unknown wire type 7" in endpoint.unmarshalSatStreamID

			// _, err = satelliteSys.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.ObjectFinishCopyRequest{
			//	Header: &pb.RequestHeader{
			//		ApiKey: planet.Uplinks[2].APIKey[planet.Satellites[0].ID()].SerializeRaw(),
			//	},
			//	StreamId:                     metabaseObj.ObjectStream.StreamID.Bytes(),
			//	NewBucket:                    []byte("testbucket"),
			//	NewEncryptedObjectKey:        []byte("newencryptedobjectkey"),
			//	NewEncryptedMetadata:         testrand.Bytes(10),
			//	NewEncryptedMetadataKey:      randomEncKey.Raw()[:],
			//	NewEncryptedMetadataKeyNonce: testrand.Nonce(),
			//	NewSegmentKeys: []*pb.EncryptedKeyAndNonce{
			//		{
			//			Position: &pb.SegmentPosition{
			//				PartNumber: 0,
			//				Index:      0,
			//			},
			//			EncryptedKeyNonce: testrand.Nonce(),
			//			EncryptedKey:      randomEncKey.Raw()[:],
			//		},
			//	},
			// })
			// assertRPCStatusCode(t, err, rpcstatus.ResourceExhausted)
			// assert.EqualError(t, err, "Exceeded Storage Limit")

			// test that a smaller object can still be uploaded and copied
			err = planet.Uplinks[2].Upload(ctx, planet.Satellites[0], "testbucket", "testobject2", testrand.Bytes(10))
			require.NoError(t, err)

			err = planet.Uplinks[2].CopyObject(ctx, planet.Satellites[0], "testbucket", "testobject2", "testbucket", "testobject2copy")
			require.NoError(t, err)

			err = satelliteSys.API.Metainfo.Metabase.TestingDeleteAll(ctx)
			require.NoError(t, err)
		}

		{ // test copy respects project segment limit
			// set segment limit
			err = planet.Satellites[0].DB.ProjectAccounting().UpdateProjectSegmentLimit(ctx, planet.Uplinks[3].Projects[0].ID, 2)
			require.NoError(t, err)

			err = planet.Uplinks[3].Upload(ctx, planet.Satellites[0], "testbucket", "testobject", testrand.Bytes(100))
			require.NoError(t, err)
			objects, err = satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)

			err = planet.Uplinks[3].CopyObject(ctx, planet.Satellites[0], "testbucket", "testobject", "testbucket", "testobject1")
			require.NoError(t, err)

			_, err = satelliteSys.API.Metainfo.Endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: planet.Uplinks[3].APIKey[planet.Satellites[0].ID()].SerializeRaw(),
				},
				Bucket:                []byte("testbucket"),
				EncryptedObjectKey:    []byte(objects[0].ObjectKey),
				NewBucket:             []byte("testbucket"),
				NewEncryptedObjectKey: []byte("newencryptedobjectkey1"),
			})
			assertRPCStatusCode(t, err, rpcstatus.ResourceExhausted)
			assert.EqualError(t, err, "Exceeded Segments Limit")
		}
	})
}

func TestEndpoint_ParallelDeletes(t *testing.T) {
	t.Skip("to be fixed - creating deadlocks")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)
		testData := testrand.Bytes(5 * memory.KiB)
		for i := 0; i < 50; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "object"+strconv.Itoa(i), testData)
			require.NoError(t, err)
			_, err = project.CopyObject(ctx, "bucket", "object"+strconv.Itoa(i), "bucket", "object"+strconv.Itoa(i)+"copy", nil)
			require.NoError(t, err)
		}
		list := project.ListObjects(ctx, "bucket", nil)
		keys := []string{}
		for list.Next() {
			item := list.Item()
			keys = append(keys, item.Key)
		}
		require.NoError(t, list.Err())
		var wg sync.WaitGroup
		wg.Add(len(keys))
		var errlist errs.Group

		for i, name := range keys {
			name := name
			go func(toDelete string, index int) {
				_, err := project.DeleteObject(ctx, "bucket", toDelete)
				errlist.Add(err)
				wg.Done()
			}(name, i)
		}
		wg.Wait()

		require.NoError(t, errlist.Err())

		// check all objects have been deleted
		listAfterDelete := project.ListObjects(ctx, "bucket", nil)
		require.False(t, listAfterDelete.Next())
		require.NoError(t, listAfterDelete.Err())

		_, err = project.DeleteBucket(ctx, "bucket")
		require.NoError(t, err)
	})
}

func TestEndpoint_ParallelDeletesSameAncestor(t *testing.T) {
	t.Skip("to be fixed - creating deadlocks")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)
		testData := testrand.Bytes(5 * memory.KiB)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "original-object", testData)
		require.NoError(t, err)
		for i := 0; i < 50; i++ {
			_, err = project.CopyObject(ctx, "bucket", "original-object", "bucket", "copy"+strconv.Itoa(i), nil)
			require.NoError(t, err)
		}
		list := project.ListObjects(ctx, "bucket", nil)
		keys := []string{}
		for list.Next() {
			item := list.Item()
			keys = append(keys, item.Key)
		}
		require.NoError(t, list.Err())
		var wg sync.WaitGroup
		wg.Add(len(keys))
		var errlist errs.Group

		for i, name := range keys {
			name := name
			go func(toDelete string, index int) {
				_, err := project.DeleteObject(ctx, "bucket", toDelete)
				errlist.Add(err)
				wg.Done()
			}(name, i)
		}
		wg.Wait()

		require.NoError(t, errlist.Err())

		// check all objects have been deleted
		listAfterDelete := project.ListObjects(ctx, "bucket", nil)
		require.False(t, listAfterDelete.Next())
		require.NoError(t, listAfterDelete.Err())

		_, err = project.DeleteBucket(ctx, "bucket")
		require.NoError(t, err)
	})
}

func TestEndpoint_UpdateObjectMetadata(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].SerializeRaw()
		err := planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "object", testrand.Bytes(256))
		require.NoError(t, err)

		objects, err := satellite.API.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)

		validMetadata := testrand.Bytes(satellite.Config.Metainfo.MaxMetadataSize)
		validKey := randomEncryptedKey

		getObjectResponse, err := satellite.API.Metainfo.Endpoint.GetObject(ctx, &pb.ObjectGetRequest{
			Header:             &pb.RequestHeader{ApiKey: apiKey},
			Bucket:             []byte("testbucket"),
			EncryptedObjectKey: []byte(objects[0].ObjectKey),
		})
		require.NoError(t, err)

		_, err = satellite.API.Metainfo.Endpoint.UpdateObjectMetadata(ctx, &pb.ObjectUpdateMetadataRequest{
			Header:                        &pb.RequestHeader{ApiKey: apiKey},
			Bucket:                        []byte("testbucket"),
			EncryptedObjectKey:            []byte(objects[0].ObjectKey),
			StreamId:                      getObjectResponse.Object.StreamId,
			EncryptedMetadata:             validMetadata,
			EncryptedMetadataEncryptedKey: validKey,
		})
		require.NoError(t, err)

		// too large metadata
		_, err = satellite.API.Metainfo.Endpoint.UpdateObjectMetadata(ctx, &pb.ObjectUpdateMetadataRequest{
			Header:             &pb.RequestHeader{ApiKey: apiKey},
			Bucket:             []byte("testbucket"),
			EncryptedObjectKey: []byte(objects[0].ObjectKey),

			EncryptedMetadata:             testrand.Bytes(satellite.Config.Metainfo.MaxMetadataSize + 1),
			EncryptedMetadataEncryptedKey: validKey,
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// invalid encrypted metadata key
		_, err = satellite.API.Metainfo.Endpoint.UpdateObjectMetadata(ctx, &pb.ObjectUpdateMetadataRequest{
			Header:             &pb.RequestHeader{ApiKey: apiKey},
			Bucket:             []byte("testbucket"),
			EncryptedObjectKey: []byte(objects[0].ObjectKey),

			EncryptedMetadata:             validMetadata,
			EncryptedMetadataEncryptedKey: testrand.Bytes(16),
		})
		require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

		// verify that metadata didn't change with rejected requests
		objects, err = satellite.API.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Equal(t, validMetadata, objects[0].EncryptedMetadata)
		require.Equal(t, validKey, objects[0].EncryptedMetadataEncryptedKey)
	})
}

func TestEndpoint_Object_CopyObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checkDownload := func(objectKey string, expectedData []byte) {
			data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "multipleversions", objectKey)
			require.NoError(t, err)
			require.Equal(t, expectedData, data)
		}

		expectedDataA := testrand.Bytes(7 * memory.KiB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectA", expectedDataA)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectInline", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectRemote", testrand.Bytes(10*memory.KiB))
		require.NoError(t, err)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		_, err = project.CopyObject(ctx, "multipleversions", "objectA", "multipleversions", "objectInline", nil)
		require.NoError(t, err)

		_, err = project.CopyObject(ctx, "multipleversions", "objectA", "multipleversions", "objectRemote", nil)
		require.NoError(t, err)

		checkDownload("objectInline", expectedDataA)
		checkDownload("objectRemote", expectedDataA)

		expectedDataB := testrand.Bytes(8 * memory.KiB)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectInline", expectedDataB)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectRemote", expectedDataB)
		require.NoError(t, err)

		checkDownload("objectInline", expectedDataB)
		checkDownload("objectRemote", expectedDataB)
		checkDownload("objectA", expectedDataA)

		expectedDataD := testrand.Bytes(6 * memory.KiB)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectA", expectedDataD)
		require.NoError(t, err)

		checkDownload("objectInline", expectedDataB)
		checkDownload("objectRemote", expectedDataB)
		checkDownload("objectA", expectedDataD)

		objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 3)

		for _, object := range objects {
			require.Greater(t, int64(object.Version), int64(1))
		}

		_, err = project.CopyObject(ctx, "multipleversions", "objectInline", "multipleversions", "objectInlineCopy", nil)
		require.NoError(t, err)

		checkDownload("objectInlineCopy", expectedDataB)

		iterator := project.ListObjects(ctx, "multipleversions", nil)

		items := []string{}
		for iterator.Next() {
			items = append(items, iterator.Item().Key)
		}
		require.NoError(t, iterator.Err())

		sort.Strings(items)
		require.Equal(t, []string{
			"objectA", "objectInline", "objectInlineCopy", "objectRemote",
		}, items)
	})
}

func TestEndpoint_Object_MoveObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedDataA := testrand.Bytes(7 * memory.KiB)

		// upload objectA twice to have to have version different than 1
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectA", expectedDataA)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectA", expectedDataA)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectB", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		// move is not possible because we have committed object under target location
		err = project.MoveObject(ctx, "multipleversions", "objectA", "multipleversions", "objectB", nil)
		require.Error(t, err)
	})
}

func TestListObjectDuplicates(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		u := planet.Uplinks[0]
		s := planet.Satellites[0]

		const amount = 23

		require.NoError(t, u.CreateBucket(ctx, s, "test"))

		prefixes := []string{"", "aprefix/"}

		// reupload some objects many times to force different
		// object versions internally
		for _, prefix := range prefixes {
			for i := 0; i < amount; i++ {
				version := 1
				if i%2 == 0 {
					version = 2
				} else if i%3 == 0 {
					version = 3
				}

				for v := 0; v < version; v++ {
					require.NoError(t, u.Upload(ctx, s, "test", prefix+fmt.Sprintf("file-%d", i), nil))
				}
			}
		}

		project, err := u.GetProject(ctx, s)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		for _, prefix := range prefixes {
			prefixLabel := prefix
			if prefixLabel == "" {
				prefixLabel = "empty"
			}

			for _, listLimit := range []int{0, 1, 2, 3, 7, amount - 1, amount} {
				t.Run(fmt.Sprintf("prefix %s limit %d", prefixLabel, listLimit), func(t *testing.T) {
					limitCtx := testuplink.WithListLimit(ctx, listLimit)

					keys := make(map[string]struct{})
					iter := project.ListObjects(limitCtx, "test", &uplink.ListObjectsOptions{
						Prefix: prefix,
					})
					for iter.Next() {
						if iter.Item().IsPrefix {
							continue
						}

						if _, ok := keys[iter.Item().Key]; ok {
							t.Fatal("duplicate", iter.Item().Key, len(keys))
						}
						keys[iter.Item().Key] = struct{}{}
					}
					require.NoError(t, iter.Err())
					require.Equal(t, amount, len(keys))
				})
			}
		}
	})
}

func TestListUploads(t *testing.T) {
	t.Skip() // see TODO at the bottom. this test is now failing.
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// basic ListUploads tests, more tests are on storj/uplink side
		u := planet.Uplinks[0]
		s := planet.Satellites[0]

		project, err := u.OpenProject(ctx, s)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		require.NoError(t, u.CreateBucket(ctx, s, "testbucket"))

		// TODO number of objects created can be limited when uplink will
		// have an option to control listing limit value for ListUploads
		for i := 0; i < 1001; i++ {
			_, err := project.BeginUpload(ctx, "testbucket", "object"+strconv.Itoa(i), nil)
			require.NoError(t, err)
		}

		list := project.ListUploads(ctx, "testbucket", nil)
		items := 0
		for list.Next() {
			items++
		}
		require.NoError(t, list.Err())
		// TODO result should be 1001 but we have bug in libuplink
		// were it's not possible to get second page of results for
		// pending objets.
		// test will fail when we will fix uplink and we will need to adjust this test
		require.Equal(t, 1000, items)
	})
}

func TestNodeTagPlacement(t *testing.T) {
	ctx := testcontext.New(t)

	satelliteIdentity := signing.SignerFromFullIdentity(testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()))

	placementRules := nodeselection.ConfigurablePlacementRule{}
	tag := fmt.Sprintf(`tag("%s", "certified","true")`, satelliteIdentity.ID())
	err := placementRules.Set(fmt.Sprintf(`0:exclude(%s);16:%s`, tag, tag))
	require.NoError(t, err)

	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount:   1,
			StorageNodeCount: 12,
			UplinkCount:      1,
			Reconfigure: testplanet.Reconfigure{
				Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
					config.Metainfo.RS.Min = 3
					config.Metainfo.RS.Repair = 4
					config.Metainfo.RS.Success = 5
					config.Metainfo.RS.Total = 6
					config.Metainfo.MaxInlineSegmentSize = 1
					config.Placement = placementRules
				},
				StorageNode: func(index int, config *storagenode.Config) {
					if index%2 == 0 {
						tags := &pb.NodeTagSet{
							NodeId:   testidentity.MustPregeneratedSignedIdentity(index+1, storj.LatestIDVersion()).ID.Bytes(),
							SignedAt: time.Now().Unix(),
							Tags: []*pb.Tag{
								{
									Name:  "certified",
									Value: []byte("true"),
								},
							},
						}
						signed, err := nodetag.Sign(ctx, tags, satelliteIdentity)
						require.NoError(t, err)

						config.Contact.Tags = contact.SignedTags(pb.SignedNodeTagSets{
							Tags: []*pb.SignedNodeTagSet{
								signed,
							},
						})
					}

				},
			},
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			buckets := satellite.API.Buckets.Service
			uplink := planet.Uplinks[0]
			projectID := uplink.Projects[0].ID

			apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
			metainfoClient, err := uplink.DialMetainfo(ctx, satellite, apiKey)
			require.NoError(t, err)
			defer func() {
				_ = metainfoClient.Close()
			}()

			nodeIndex := map[storj.NodeID]int{}
			for ix, node := range planet.StorageNodes {
				nodeIndex[node.Identity.ID] = ix
			}
			testPlacement := func(bucketName string, placement int, allowedNodes func(int) bool) {

				createGeofencedBucket(t, ctx, buckets, projectID, bucketName, storj.PlacementConstraint(placement))

				objectNo := 10
				for i := 0; i < objectNo; i++ {

					err := uplink.Upload(ctx, satellite, bucketName, "testobject"+strconv.Itoa(i), make([]byte, 10240))
					require.NoError(t, err)
				}

				objects, _, err := metainfoClient.ListObjects(ctx, metaclient.ListObjectsParams{
					Bucket: []byte(bucketName),
				})
				require.NoError(t, err)
				require.Len(t, objects, objectNo)

				for _, listedObject := range objects {
					for i := 0; i < 5; i++ {
						o, err := metainfoClient.DownloadObject(ctx, metaclient.DownloadObjectParams{
							Bucket:             []byte(bucketName),
							EncryptedObjectKey: listedObject.EncryptedObjectKey,
						})
						require.NoError(t, err)

						for _, limit := range o.DownloadedSegments[0].Limits {
							if limit != nil {
								ix := nodeIndex[limit.Limit.StorageNodeId]
								require.True(t, allowedNodes(ix))
							}
						}
					}
				}
			}
			t.Run("upload to constrained", func(t *testing.T) {
				testPlacement("constrained", 16, func(i int) bool {
					return i%2 == 0
				})
			})
			t.Run("upload to generic excluding constrained", func(t *testing.T) {
				testPlacement("generic", 0, func(i int) bool {
					return i%2 == 1
				})
			})

		},
	)
}

func TestEndpoint_Object_No_StorageNodes_Versioning(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satelliteSys := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].SerializeRaw()
		projectID := planet.Uplinks[0].Projects[0].ID

		bucketName := "versioned-bucket"
		objectKey := "versioned-object"

		project, err := planet.Uplinks[0].OpenProject(ctx, satelliteSys)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		createBucket := func(name string) error {
			_, err := satelliteSys.API.Metainfo.Endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey},
				Name:   []byte(name),
			})
			return err
		}

		deleteBucket := func(name string) func() error {
			return func() error {
				_, err := satelliteSys.API.Metainfo.Endpoint.DeleteBucket(ctx, &pb.DeleteBucketRequest{
					Header:    &pb.RequestHeader{ApiKey: apiKey},
					Name:      []byte(name),
					DeleteAll: true,
				})
				return err
			}
		}

		t.Run("object with 2 versions", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			require.NoError(t, createBucket(bucketName))

			require.NoError(t, planet.Satellites[0].API.Buckets.Service.EnableBucketVersioning(ctx, []byte(bucketName), projectID))

			state, err := planet.Satellites[0].API.Buckets.Service.GetBucketVersioningState(ctx, []byte(bucketName), projectID)
			require.NoError(t, err)
			require.Equal(t, buckets.VersioningEnabled, state)

			err = planet.Uplinks[0].Upload(ctx, satelliteSys, bucketName, objectKey, testrand.Bytes(100))
			require.NoError(t, err)

			err = planet.Uplinks[0].Upload(ctx, satelliteSys, bucketName, objectKey, testrand.Bytes(100))
			require.NoError(t, err)

			objects, err := satelliteSys.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 2)

			response, err := satelliteSys.API.Metainfo.Endpoint.BeginDeleteObject(ctx, &pb.BeginDeleteObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)
			require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, response.Object.Status)

			objects, err = satelliteSys.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 3)

			// version is not set, object not found error
			_, err = satelliteSys.API.Metainfo.Endpoint.GetObject(ctx, &pb.GetObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))

			// with version set we should get MethodNotAllowed error
			_, err = satelliteSys.API.Metainfo.Endpoint.GetObject(ctx, &pb.GetObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				ObjectVersion:      response.Object.ObjectVersion,
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.MethodNotAllowed))

			// with version set we should get MethodNotAllowed error
			_, err = satelliteSys.API.Metainfo.Endpoint.DownloadObject(ctx, &pb.DownloadObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				ObjectVersion:      response.Object.ObjectVersion,
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.MethodNotAllowed))
		})

		t.Run("listing objects, different versioning state", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			require.NoError(t, createBucket(bucketName))

			err = planet.Uplinks[0].Upload(ctx, satelliteSys, bucketName, "objectA", testrand.Bytes(100))
			require.NoError(t, err)

			err = planet.Uplinks[0].Upload(ctx, satelliteSys, bucketName, "objectB", testrand.Bytes(100))
			require.NoError(t, err)

			checkListing := func(expectedItems int, includeAllVersions bool) {
				response, err := satelliteSys.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey},
					Bucket:             []byte(bucketName),
					IncludeAllVersions: includeAllVersions,
				})
				require.NoError(t, err)
				require.Len(t, response.Items, expectedItems)
			}

			checkListing(2, false)

			require.NoError(t, planet.Satellites[0].API.Buckets.Service.EnableBucketVersioning(ctx, []byte(bucketName), projectID))

			// upload second version of objectA
			err = planet.Uplinks[0].Upload(ctx, satelliteSys, bucketName, "objectA", testrand.Bytes(100))
			require.NoError(t, err)

			checkListing(2, false)
			checkListing(3, true)

			require.NoError(t, planet.Satellites[0].API.Buckets.Service.SuspendBucketVersioning(ctx, []byte(bucketName), projectID))

			checkListing(2, false)
			checkListing(3, true)
		})

		t.Run("check UploadID for versioned bucket", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			require.NoError(t, createBucket(bucketName))
			require.NoError(t, planet.Satellites[0].API.Buckets.Service.EnableBucketVersioning(ctx, []byte(bucketName), projectID))

			response, err := satelliteSys.API.Metainfo.Endpoint.BeginObject(ctx, &pb.BeginObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objectKey),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
				},
			})
			require.NoError(t, err)

			listResponse, err := satelliteSys.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header: &pb.RequestHeader{ApiKey: apiKey},
				Bucket: []byte(bucketName),
				Status: pb.Object_UPLOADING,
			})
			require.NoError(t, err)
			require.Len(t, listResponse.Items, 1)
			// StreamId is encoded into UploadID on libuplink side
			// require.Equal(t, response.StreamId.Bytes(), listResponse.Items[0].StreamId.Bytes())

			lposResponse, err := satelliteSys.API.Metainfo.Endpoint.ListPendingObjectStreams(ctx, &pb.ListPendingObjectStreamsRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: response.EncryptedObjectKey,
			})
			require.NoError(t, err)
			require.Len(t, lposResponse.Items, 1)
			require.Equal(t, response.StreamId.Bytes(), lposResponse.Items[0].StreamId.Bytes())
		})

		t.Run("listing objects, all versions, version cursor handling", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			require.NoError(t, createBucket(bucketName))
			require.NoError(t, planet.Satellites[0].API.Buckets.Service.EnableBucketVersioning(ctx, []byte(bucketName), projectID))

			expectedVersions := [][]byte{}
			for i := 0; i < 5; i++ {
				object, err := planet.Uplinks[0].UploadWithOptions(ctx, satelliteSys, bucketName, "objectA", testrand.Bytes(100), nil)
				require.NoError(t, err)
				expectedVersions = append(expectedVersions, object.Version)
			}

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, objects)

			listObjectVersions := func(version []byte, limit int) *pb.ListObjectsResponse {
				response, err := satelliteSys.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
					Header: &pb.RequestHeader{ApiKey: apiKey},
					Bucket: []byte(bucketName),
					// all objects have the same key but different versions
					EncryptedCursor:    []byte(objects[0].ObjectKey),
					VersionCursor:      version,
					IncludeAllVersions: true,
					Limit:              int32(limit),
				})
				require.NoError(t, err)
				return response
			}

			for i, version := range expectedVersions {
				response := listObjectVersions(version, 0)
				require.Len(t, response.Items, i)

				versions := [][]byte{}
				for i := len(response.Items) - 1; i >= 0; i-- {
					item := response.Items[i]
					versions = append(versions, item.ObjectVersion)
				}

				require.Equal(t, expectedVersions[:i], versions)
			}

			response := listObjectVersions(expectedVersions[4], 2)
			require.NoError(t, err)
			require.Len(t, response.Items, 2)
			require.True(t, response.More)

			response = listObjectVersions(expectedVersions[2], 2)
			require.Len(t, response.Items, 2)
			require.False(t, response.More)
		})

		t.Run("get objects with delete marker", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			require.NoError(t, createBucket(bucketName))
			require.NoError(t, planet.Satellites[0].API.Buckets.Service.EnableBucketVersioning(ctx, []byte(bucketName), projectID))

			// upload first version of the item
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "objectA", testrand.Bytes(100))
			require.NoError(t, err)

			// upload second version of the item
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "objectA", testrand.Bytes(100))
			require.NoError(t, err)

			// delete the second version (latest). Should create a delete marker.
			deleteResponse, err := planet.Satellites[0].Metainfo.Endpoint.BeginDeleteObject(ctx, &pb.BeginDeleteObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey,
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("objectA"),
				ObjectVersion:      nil,
			})
			require.NoError(t, err)
			require.NotEmpty(t, deleteResponse)
			require.Equal(t, pb.Object_DELETE_MARKER_VERSIONED, deleteResponse.Object.Status)

			// get the delete marker (latest), should return error
			_, err = planet.Satellites[0].Metainfo.Endpoint.GetObject(ctx, &pb.GetObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey,
				},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte("objectA"),
				ObjectVersion:      nil,
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		})

		t.Run("begin copy object from older version", func(t *testing.T) {
			defer ctx.Check(deleteBucket(bucketName))

			require.NoError(t, createBucket(bucketName))
			require.NoError(t, planet.Satellites[0].API.Buckets.Service.EnableBucketVersioning(ctx, []byte(bucketName), projectID))

			objectKeyA := "test-object-a"
			versionIDs := make([][]byte, 2)
			for i := range versionIDs {
				object, err := planet.Uplinks[0].UploadWithOptions(ctx, satelliteSys, bucketName, objectKeyA, testrand.Bytes(100), nil)
				require.NoError(t, err)

				versionIDs[i] = object.Version
			}

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 2)

			_, err = planet.Satellites[0].Metainfo.Endpoint.BeginCopyObject(ctx, &pb.BeginCopyObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				ObjectVersion:      versionIDs[0],

				NewBucket:             []byte(bucketName),
				NewEncryptedObjectKey: []byte("copied-object"),
			})
			require.NoError(t, err)

			// NOT existing source version
			nonExistingVersion := versionIDs[0]
			nonExistingVersion[0]++
			_, err = planet.Satellites[0].Metainfo.Endpoint.BeginCopyObject(ctx, &pb.BeginCopyObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				ObjectVersion:      nonExistingVersion,

				NewBucket:             []byte(bucketName),
				NewEncryptedObjectKey: []byte("copied-object"),
			})
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.NotFound))
		})

		t.Run("finish copy object, unversioned into versioned bucket", func(t *testing.T) {
			defer ctx.Check(deleteBucket("unversioned"))
			defer ctx.Check(deleteBucket("versioned"))

			require.NoError(t, createBucket("unversioned"))
			require.NoError(t, createBucket("versioned"))
			require.NoError(t, planet.Satellites[0].API.Buckets.Service.EnableBucketVersioning(ctx, []byte("versioned"), projectID))

			_, err := planet.Uplinks[0].UploadWithOptions(ctx, satelliteSys, "unversioned", "object-key", testrand.Bytes(100), nil)
			require.NoError(t, err)

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)
			require.Equal(t, metabase.CommittedUnversioned, objects[0].Status)

			beginResponse, err := planet.Satellites[0].Metainfo.Endpoint.BeginCopyObject(ctx, &pb.BeginCopyObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte("unversioned"),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),

				NewBucket:             []byte("versioned"),
				NewEncryptedObjectKey: []byte("copy"),
			})
			require.NoError(t, err)

			finishResponse, err := planet.Satellites[0].Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header:                &pb.RequestHeader{ApiKey: apiKey},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte("versioned"),
				NewEncryptedObjectKey: []byte("copy"),
				NewSegmentKeys:        beginResponse.SegmentKeys,
			})
			require.NoError(t, err)

			require.Equal(t, pb.Object_COMMITTED_VERSIONED, finishResponse.Object.Status)
		})
	})
}
