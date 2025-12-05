// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/sudo"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"storj.io/common/encryption"
	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/rpc/rpctest"
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
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/nodetag"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/contact"
	"storj.io/uplink"
	"storj.io/uplink/private/bucket"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/object"
	"storj.io/uplink/private/piecestore"
	"storj.io/uplink/private/testuplink"
)

const (
	objectLockedErrMsg       = "object is protected by Object Lock settings"
	objectInvalidStateErrMsg = "The operation is not permitted for this object"
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

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		bucketName := "testbucket"
		deleteBucket := func() error {
			if err := planet.Satellites[0].Metabase.DB.TestingDeleteAll(ctx); err != nil {
				return err
			}

			return planet.Satellites[0].DB.Buckets().DeleteBucket(ctx, []byte(bucketName), planet.Uplinks[0].Projects[0].ID)
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

			err = planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
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
				StreamID: beginObjectResponse.StreamID,
				EncryptedUserData: metaclient.EncryptedUserData{
					EncryptedMetadata:             metadata,
					EncryptedMetadataNonce:        testrand.Nonce(),
					EncryptedMetadataEncryptedKey: randomEncryptedKey,
				},
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
				StreamID: beginObjectResponse.StreamID,
				EncryptedUserData: metaclient.EncryptedUserData{
					EncryptedMetadata:             metadata,
					EncryptedMetadataNonce:        testrand.Nonce(),
					EncryptedMetadataEncryptedKey: randomEncryptedKey,
				},
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

			encryptedUserData := metabasetest.RandEncryptedUserData()
			metadata, err := pb.Marshal(&pb.StreamMeta{
				NumberOfSegments:    1,
				EncryptionBlockSize: int32(getResp.Object.EncryptionParameters.BlockSize),
				EncryptionType:      int32(getResp.Object.EncryptionParameters.CipherSuite),
				LastSegmentMeta: &pb.SegmentMeta{
					EncryptedKey: encryptedUserData.EncryptedMetadata,
					KeyNonce:     encryptedUserData.EncryptedMetadataNonce,
				},
			})
			require.NoError(t, err)
			encryptedUserData.EncryptedMetadata = metadata

			// update the object metadata
			_, err = satelliteSys.API.Metainfo.Endpoint.UpdateObjectMetadata(ctx, &pb.ObjectUpdateMetadataRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:                        getResp.Object.Bucket,
				EncryptedObjectKey:            getResp.Object.EncryptedObjectKey,
				StreamId:                      getResp.Object.StreamId,
				EncryptedMetadataNonce:        pb.Nonce(encryptedUserData.EncryptedMetadataNonce),
				EncryptedMetadata:             encryptedUserData.EncryptedMetadata,
				EncryptedMetadataEncryptedKey: encryptedUserData.EncryptedMetadataEncryptedKey,
				EncryptedEtag:                 encryptedUserData.EncryptedETag,
				SetEncryptedEtag:              true,
			})
			require.NoError(t, err)

			// assert the metadata has been updated
			objects, err = satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)
			assert.Equal(t, encryptedUserData, objects[0].EncryptedUserData)

			// ensure we get that information from get object as well
			resp, err := satelliteSys.API.Metainfo.Endpoint.GetObject(ctx, &pb.ObjectGetRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             getResp.Object.Bucket,
				EncryptedObjectKey: getResp.Object.EncryptedObjectKey,
			})
			require.NoError(t, err)

			require.Equal(t, encryptedUserData.EncryptedMetadata, resp.Object.EncryptedMetadata)
			require.Equal(t, encryptedUserData.EncryptedMetadataEncryptedKey, resp.Object.EncryptedMetadataEncryptedKey)
			require.Equal(t, encryptedUserData.EncryptedETag, resp.Object.EncryptedEtag)
			require.Equal(t, encryptedUserData.EncryptedMetadataNonce, resp.Object.EncryptedMetadataNonce.Bytes())
		})

		t.Run("check delete rights on upload", func(t *testing.T) {
			defer ctx.Check(deleteBucket)

			up := planet.Uplinks[0]

			err := up.TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
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

			err = planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
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

			err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
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

			downloadObjectResponse, err := satellite.API.Metainfo.Endpoint.DownloadObject(peerctx, &pb.ObjectDownloadRequest{
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

			err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
			require.NoError(t, err)

			params := metaclient.BeginObjectParams{
				Bucket: []byte(bucketName),
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
			downloadObjectResponse, err := satellite.API.Metainfo.Endpoint.DownloadObject(peerctx, &pb.ObjectDownloadRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("testbucket"),
				EncryptedObjectKey: []byte(committedObject.ObjectKey),
			})
			require.NoError(t, err)
			require.EqualValues(t, committedObject.BucketName, downloadObjectResponse.Object.Bucket)
			require.EqualValues(t, committedObject.ObjectKey, downloadObjectResponse.Object.EncryptedObjectKey)
			require.EqualValues(t, committedObject.StreamVersionID().Bytes(), downloadObjectResponse.Object.ObjectVersion)

			// download using explicit version
			downloadObjectResponse, err = satellite.API.Metainfo.Endpoint.DownloadObject(peerctx, &pb.ObjectDownloadRequest{
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
			_, err = satellite.API.Metainfo.Endpoint.DownloadObject(peerctx, &pb.ObjectDownloadRequest{
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
	// Spanner could be quite slow sometimes, 1 second seems quite enough for the last test failures
	// that we got due to not hitting the limit.
	const uploadLimitSingleObject = 1 * time.Second
	const uploadLimitSingleObjectBurst = 3

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Metainfo.UploadLimiter.SingleObjectLimit = uploadLimitSingleObject
			},
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		sat := planet.Satellites[0]
		endpoint := sat.Metainfo.Endpoint

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		bucketName := "testbucket"

		project, err := sat.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.NotNil(t, project)

		err = planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		limit := 2 * memory.KB
		project.StorageLimit = &limit
		project.BandwidthLimit = &limit
		err = sat.DB.Console().Projects().Update(ctx, project)
		assert.NoError(t, err)

		t.Run("limit single object upload", func(t *testing.T) {

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

			now := time.Now()
			ctx, machine := time2.WithNewMachine(ctx, time2.WithTimeAt(now))

			// upload to the burst limit
			for i := 0; i < uploadLimitSingleObjectBurst; i++ {
				_, err := endpoint.BeginObject(ctx, request)
				require.NoError(t, err)
			}

			_, err = endpoint.BeginObject(ctx, request)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			// upload to different locations one by one should NOT fail
			request.EncryptedObjectKey = []byte("single-objectA")
			_, err = endpoint.BeginObject(ctx, request)
			require.NoError(t, err)

			request.EncryptedObjectKey = []byte("single-objectB")
			_, err = endpoint.BeginObject(ctx, request)
			require.NoError(t, err)

			// Set the context clock enough in the future to ensure that the rate limit allows another request through.
			machine.Advance(uploadLimitSingleObject + time.Millisecond)
			request.EncryptedObjectKey = []byte("single-object")

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
			_, err = endpoint.DownloadObject(peerctx, request)
			require.NoError(t, err)

			// zero user specified bandwidth limit
			project.UserSpecifiedBandwidthLimit = new(memory.Size)
			err = sat.DB.Console().Projects().Update(ctx, project)
			assert.NoError(t, err)

			// must fail because user specified bandwidth limit is zero
			_, err = endpoint.DownloadObject(peerctx, request)
			require.Error(t, err)
			require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

			project.UserSpecifiedBandwidthLimit = project.BandwidthLimit
			err = sat.DB.Console().Projects().Update(ctx, project)
			assert.NoError(t, err)

			_, err = endpoint.DownloadObject(peerctx, request)
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

		err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName)
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
			if err := planet.Satellites[0].Metabase.DB.TestingDeleteAll(ctx); err != nil {
				return err
			}

			return planet.Satellites[0].DB.Buckets().DeleteBucket(ctx, []byte(bucketName), planet.Uplinks[0].Projects[0].ID)
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

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		require.NoError(t, planet.Satellites[0].API.DB.Console().Projects().UpdateDefaultPlacement(ctx, planet.Uplinks[0].Projects[0].ID, storj.EU))

		bucketName := "testbucket"
		deleteBucket := func(bucketName string) func() error {
			return func() error {
				if err := planet.Satellites[0].Metabase.DB.TestingDeleteAll(ctx); err != nil {
					return err
				}

				return planet.Satellites[0].DB.Buckets().DeleteBucket(ctx, []byte(bucketName), planet.Uplinks[0].Projects[0].ID)
			}
		}

		fullIDMap := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			fullIDMap[node.ID()] = node.Identity
		}

		makeResult := func(num int32, limits []*pb.AddressedOrderLimit) *pb.SegmentPieceUploadResult {
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

			err = metainfoClient.CommitSegment(ctx, metaclient.CommitSegmentParams{
				SegmentID: response.SegmentID,
				Encryption: metaclient.SegmentEncryption{
					EncryptedKey: testrand.Bytes(256),
				},
				PlainSize:         5000,
				SizeEncryptedData: memory.MiB.Int64(),
				UploadResult: []*pb.SegmentPieceUploadResult{
					makeResult(0, response.Limits),
					makeResult(1, response.Limits),
					makeResult(2, response.Limits),
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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
			uplnk := planet.Uplinks[0]
			uplinkCtx := testuplink.WithMaxSegmentSize(ctx, 5*memory.KB)
			sat := planet.Satellites[0]

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
			usNode.Contact.Chore.Pause(ctx)
			defer usNode.Contact.Chore.Restart(ctx)

			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, usNode.ID(), "US"))
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

			affected, err := planet.Satellites[0].Metabase.DB.TestingSetObjectVersion(ctx, committedObject.ObjectStream, randomVersion)
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
				return a.BucketName.Compare(b.BucketName)
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

			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "object", testrand.Bytes(10*memory.KiB))
			require.NoError(t, err)

			// override object
			expectedData := testrand.Bytes(11 * memory.KiB)
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "object", expectedData)
			require.NoError(t, err)

			afterObjects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, afterObjects, 1)

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

		t.Run("DownloadObject no lite request", func(t *testing.T) {
			defer ctx.Check(deleteBucket("bucket"))
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "bucket"))

			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "lite-object", testrand.Bytes(11*memory.KiB))
			require.NoError(t, err)

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			endpoint := planet.Satellites[0].Metainfo.Endpoint

			response, err := endpoint.DownloadObject(peerctx, &pb.DownloadObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("bucket"),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
			})
			require.NoError(t, err)

			require.Len(t, response.SegmentDownload, 1)

			// verify that signatures are not generated
			checked := 0
			for _, limit := range response.SegmentDownload[0].AddressedLimits {
				if limit.Limit != nil {
					require.NotEmpty(t, limit.Limit.SatelliteSignature)
					checked++
				}
			}
			require.NotZero(t, checked)

			// verify root piece ID is returned
			for _, sd := range response.SegmentDownload {
				require.Empty(t, sd.SegmentId, "segment ID must not be set")
			}
		})

		t.Run("DownloadObject lite request", func(t *testing.T) {
			defer ctx.Check(deleteBucket("bucket"))
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "bucket"))

			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "lite-object", testrand.Bytes(11*memory.KiB))
			require.NoError(t, err)

			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			endpoint := planet.Satellites[0].Metainfo.Endpoint

			response, err := endpoint.DownloadObject(peerctx, &pb.DownloadObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("bucket"),
				EncryptedObjectKey: []byte(objects[0].ObjectKey),
				LiteRequest:        true,
			})
			require.NoError(t, err)

			require.Len(t, response.SegmentDownload, 1)

			// verify that signatures are not generated
			checked := 0
			for _, limit := range response.SegmentDownload[0].AddressedLimits {
				if limit.Limit != nil {
					require.Empty(t, limit.Limit.SatelliteSignature)
					checked++
				}
			}
			require.NotZero(t, checked)

			// verify root piece ID is returned
			for _, sd := range response.SegmentDownload {
				require.NotEmpty(t, sd.SegmentId, "segment ID must be set")

				encodedSegID, err := storj.SegmentIDFromBytes(sd.SegmentId)
				require.NoError(t, err)
				require.False(t, encodedSegID.IsZero(), "segment ID cannot be 0")

				var segID internalpb.SegmentID
				err = pb.Unmarshal(encodedSegID, &segID)
				require.NoError(t, err)
				require.False(t, segID.RootPieceId.IsZero(), "segments must have the root piece ID")
			}
		})

		t.Run("upload while RS changes", func(t *testing.T) {
			defer ctx.Check(deleteBucket("bucket"))

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "bucket"))

			endpoint := planet.Satellites[0].Metainfo.Endpoint

			beginResp, err := endpoint.BeginObject(ctx, &pb.BeginObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte("bucket"),
				EncryptedObjectKey: []byte("test-object"),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
				},
			})
			require.NoError(t, err)

			beginSegResp, err := endpoint.BeginSegment(peerctx, &pb.BeginSegmentRequest{
				Header:        &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Position:      &pb.SegmentPosition{},
				StreamId:      beginResp.StreamId,
				MaxOrderLimit: memory.MiB.Int64(),
			})
			require.NoError(t, err)

			// change RS values in between begin and commit requests. RS values should be passed between
			// requests and not use current endpoint defaults (or placement).
			planet.Satellites[0].Metainfo.Endpoint.TestingSetRSConfig(metainfo.RSConfig{
				Min:     1,
				Repair:  2,
				Success: 10,
				Total:   10,
			})

			_, err = endpoint.CommitSegment(peerctx, &pb.CommitSegmentRequest{
				Header:    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				SegmentId: beginSegResp.SegmentId,
				UploadResult: []*pb.SegmentPieceUploadResult{
					makeResult(0, beginSegResp.AddressedLimits),
					makeResult(1, beginSegResp.AddressedLimits),
				},
				EncryptedKey:      testrand.Bytes(32),
				EncryptedKeyNonce: testrand.Nonce(),
				PlainSize:         512,
				SizeEncryptedData: memory.MiB.Int64(),
			})
			require.NoError(t, err)
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

func TestEndpoint_GetObjectIPs_With_Placement(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplnk := planet.Uplinks[0]
		apiKey := uplnk.APIKey[planet.Satellites[0].ID()]
		sat := planet.Satellites[0]

		bucketName := "test-bucket"
		customPlacement := storj.PlacementConstraint(5)
		createGeofencedBucket(t, ctx, sat.API.Buckets.Service, uplnk.Projects[0].ID, bucketName, customPlacement)

		require.NoError(t, uplnk.Upload(ctx, sat, bucketName, "test-placement", testrand.Bytes(3*memory.KB)))

		objects, err := sat.API.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 1)

		getResp, err := sat.Metainfo.Endpoint.GetObjectIPs(ctx, &pb.ObjectGetIPsRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:             []byte(bucketName),
			EncryptedObjectKey: []byte(objects[0].ObjectKey),
		})
		require.NoError(t, err)
		require.Equal(t, customPlacement, storj.PlacementConstraint(getResp.PlacementConstraint))
	})
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

func TestEndpoint_CopyObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 4,
		Reconfigure: testplanet.Reconfigure{
			Uplink: func(log *zap.Logger, index int, config *testplanet.UplinkConfig) {
				config.DefaultPathCipher = storj.EncNull
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[sat.ID()]
		uplnk := planet.Uplinks[0]
		endpoint := sat.API.Metainfo.Endpoint

		// If an object is uploaded during a run of the tally loop, the storage usage
		// in the live accounting cache will be inaccurate.
		sat.Accounting.Tally.Loop.Pause()

		// upload a small inline object
		err := uplnk.Upload(ctx, sat, "testbucket", "testobject", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)

		getResp, err := endpoint.GetObject(ctx, &pb.ObjectGetRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:             []byte("testbucket"),
			EncryptedObjectKey: []byte("testobject"),
		})
		require.NoError(t, err)

		testEncryptedMetadataNonce := testrand.Nonce()
		// copy the object and apply new metadata
		beginResp, err := endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
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
		assert.Equal(t, getResp.Object.EncryptedMetadataEncryptedKey, beginResp.EncryptedMetadataKey)
		assert.Equal(t, getResp.Object.EncryptedMetadataNonce, beginResp.EncryptedMetadataKeyNonce)

		segmentKeys := pb.EncryptedKeyAndNonce{
			Position:          beginResp.SegmentKeys[0].Position,
			EncryptedKeyNonce: testrand.Nonce(),
			EncryptedKey:      []byte("newencryptedkey"),
		}

		{
			// metadata too large
			_, err = endpoint.FinishCopyObject(ctx, &pb.ObjectFinishCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:                     getResp.Object.StreamId,
				NewBucket:                    []byte("testbucket"),
				NewEncryptedObjectKey:        []byte("newobjectkey"),
				NewEncryptedMetadata:         testrand.Bytes(sat.Config.Metainfo.MaxMetadataSize + 1),
				NewEncryptedMetadataKeyNonce: testEncryptedMetadataNonce,
				NewEncryptedMetadataKey:      []byte("encryptedmetadatakey"),
				NewSegmentKeys:               []*pb.EncryptedKeyAndNonce{&segmentKeys},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))

			// invalid encrypted metadata key
			_, err = endpoint.FinishCopyObject(ctx, &pb.ObjectFinishCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:                     getResp.Object.StreamId,
				NewBucket:                    []byte("testbucket"),
				NewEncryptedObjectKey:        []byte("newobjectkey"),
				NewEncryptedMetadata:         testrand.Bytes(sat.Config.Metainfo.MaxMetadataSize),
				NewEncryptedMetadataKeyNonce: testEncryptedMetadataNonce,
				NewEncryptedMetadataKey:      []byte("encryptedmetadatakey"),
				NewSegmentKeys:               []*pb.EncryptedKeyAndNonce{&segmentKeys},
			})
			require.True(t, errs2.IsRPC(err, rpcstatus.InvalidArgument))
		}

		_, err = endpoint.FinishCopyObject(ctx, &pb.ObjectFinishCopyRequest{
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

		getCopyResp, err := endpoint.GetObject(ctx, &pb.ObjectGetRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket:             []byte("testbucket"),
			EncryptedObjectKey: []byte("newobjectkey"),
		})
		require.NoError(t, err)
		require.NotEqual(t, getResp.Object.StreamId, getCopyResp.Object.StreamId)
		require.NotZero(t, getCopyResp.Object.StreamId)
		require.Equal(t, getResp.Object.InlineSize, getCopyResp.Object.InlineSize)

		// compare segments
		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: uplnk.Identity.Chain(),
			}})
		originalSegment, err := endpoint.DownloadSegment(peerctx, &pb.SegmentDownloadRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			StreamId:       getResp.Object.StreamId,
			CursorPosition: segmentKeys.Position,
		})
		require.NoError(t, err)
		copiedSegment, err := endpoint.DownloadSegment(peerctx, &pb.SegmentDownloadRequest{
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
			err = sat.DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, planet.Uplinks[1].Projects[0].ID, 1000)
			require.NoError(t, err)

			// test object below the limit when copied
			err = planet.Uplinks[1].Upload(ctx, sat, "testbucket", "testobject", testrand.Bytes(100))
			require.NoError(t, err)

			_, err = endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: planet.Uplinks[1].APIKey[sat.ID()].SerializeRaw(),
				},
				Bucket:                []byte("testbucket"),
				EncryptedObjectKey:    []byte("testobject"),
				NewBucket:             []byte("testbucket"),
				NewEncryptedObjectKey: []byte("newencryptedobjectkey"),
			})
			require.NoError(t, err)
			err = sat.API.Metainfo.Metabase.TestingDeleteAll(ctx)
			require.NoError(t, err)

			// set storage limit
			err = sat.DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, planet.Uplinks[2].Projects[0].ID, 1000)
			require.NoError(t, err)

			// test object exceeding the limit when copied
			err = planet.Uplinks[2].Upload(ctx, sat, "testbucket", "testobject", testrand.Bytes(400))
			require.NoError(t, err)

			err = planet.Uplinks[2].CopyObject(ctx, sat, "testbucket", "testobject", "testbucket", "testobject1")
			require.NoError(t, err)

			_, err = endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: planet.Uplinks[2].APIKey[sat.ID()].SerializeRaw(),
				},
				Bucket:                []byte("testbucket"),
				EncryptedObjectKey:    []byte("testobject"),
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
			//	NewEncryptedMetadataEncryptedKey:      randomEncKey.Raw()[:],
			//	NewEncryptedMetadataNonce: testrand.Nonce(),
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
			err = planet.Uplinks[2].Upload(ctx, sat, "testbucket", "testobject2", testrand.Bytes(10))
			require.NoError(t, err)

			err = planet.Uplinks[2].CopyObject(ctx, sat, "testbucket", "testobject2", "testbucket", "testobject2copy")
			require.NoError(t, err)

			err = sat.API.Metainfo.Metabase.TestingDeleteAll(ctx)
			require.NoError(t, err)
		}

		{ // test copy respects project segment limit
			// set segment limit
			err = sat.DB.ProjectAccounting().UpdateProjectSegmentLimit(ctx, planet.Uplinks[3].Projects[0].ID, 2)
			require.NoError(t, err)

			err = planet.Uplinks[3].Upload(ctx, sat, "testbucket", "testobject", testrand.Bytes(100))
			require.NoError(t, err)

			err = planet.Uplinks[3].CopyObject(ctx, sat, "testbucket", "testobject", "testbucket", "testobject1")
			require.NoError(t, err)

			_, err = endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: planet.Uplinks[3].APIKey[sat.ID()].SerializeRaw(),
				},
				Bucket:                []byte("testbucket"),
				EncryptedObjectKey:    []byte("testobject"),
				NewBucket:             []byte("testbucket"),
				NewEncryptedObjectKey: []byte("newencryptedobjectkey1"),
			})
			assertRPCStatusCode(t, err, rpcstatus.ResourceExhausted)
			assert.EqualError(t, err, "Exceeded Segments Limit")
		}
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
			require.Greater(t, int64(object.Version), int64(0))
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

		err = project.MoveObject(ctx, "multipleversions", "objectA", "multipleversions", "objectB", nil)
		require.NoError(t, err)
	})
}

func TestEndpoint_Object_MoveObject_PrefixRestricted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedDataA := testrand.Bytes(7 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "prefix1/prefix2/objectA", expectedDataA)
		require.NoError(t, err)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "prefix4/prefix2/objectA", expectedDataA)
		require.NoError(t, err)

		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
		restricted, err := access.Share(uplink.FullPermission(), uplink.SharePrefix{
			Bucket: "testbucket",
			Prefix: "prefix1/",
		})
		require.NoError(t, err)

		// make sure we have an access grant with a full encryption store
		// so that we're testing the satellite rejection and not the uplink
		// rejection. we actually can't do this with exported types and must use
		// sudo and reflect, because uplink.ParseAccess automatically restricts
		// the parsed encryption store to whatever is in the macaroon, and there
		// is no other way to make an access grant accepted by uplink.OpenProject
		// besides through uplink.ParseAccess.
		sudo.Sudo(reflect.ValueOf(restricted).Elem().FieldByName("encAccess")).Set(
			sudo.Sudo(reflect.ValueOf(access).Elem().FieldByName("encAccess")))

		project, err := uplink.OpenProject(ctx, restricted)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		// make sure that the prefix restriction restricts the destination
		err = project.MoveObject(ctx, "testbucket", "prefix1/prefix2/objectA", "testbucket", "prefix4/prefix3/objectB", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, uplink.ErrPermissionDenied)
		require.False(t, encryption.ErrMissingDecryptionBase.Has(err))
		require.False(t, encryption.ErrMissingEncryptionBase.Has(err))

		// make sure that the prefix restriction restricts the source
		err = project.MoveObject(ctx, "testbucket", "prefix4/prefix2/objectA", "testbucket", "prefix1/prefix3/objectB", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, uplink.ErrPermissionDenied)
		require.False(t, encryption.ErrMissingDecryptionBase.Has(err))
		require.False(t, encryption.ErrMissingEncryptionBase.Has(err))

		err = project.MoveObject(ctx, "testbucket", "prefix1/prefix2/objectA", "testbucket", "prefix1/prefix3/objectB", nil)
		require.NoError(t, err)
	})
}

func TestEndpoint_Object_CopyObject_PrefixRestricted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedDataA := testrand.Bytes(7 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "prefix1/prefix2/objectA", expectedDataA)
		require.NoError(t, err)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "prefix4/prefix2/objectA", expectedDataA)
		require.NoError(t, err)

		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
		restricted, err := access.Share(uplink.FullPermission(), uplink.SharePrefix{
			Bucket: "testbucket",
			Prefix: "prefix1/",
		})
		require.NoError(t, err)

		// make sure we have an access grant with a full encryption store
		// so that we're testing the satellite rejection and not the uplink
		// rejection. we actually can't do this with exported types and must use
		// sudo and reflect, because uplink.ParseAccess automatically restricts
		// the parsed encryption store to whatever is in the macaroon, and there
		// is no other way to make an access grant accepted by uplink.OpenProject
		// besides through uplink.ParseAccess.
		sudo.Sudo(reflect.ValueOf(restricted).Elem().FieldByName("encAccess")).Set(
			sudo.Sudo(reflect.ValueOf(access).Elem().FieldByName("encAccess")))

		project, err := uplink.OpenProject(ctx, restricted)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		// make sure that the prefix restriction restricts the destination
		_, err = project.CopyObject(ctx, "testbucket", "prefix1/prefix2/objectA", "testbucket", "prefix4/prefix3/objectB", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, uplink.ErrPermissionDenied)
		require.False(t, encryption.ErrMissingDecryptionBase.Has(err))
		require.False(t, encryption.ErrMissingEncryptionBase.Has(err))

		// make sure that the prefix restriction restricts the source
		_, err = project.CopyObject(ctx, "testbucket", "prefix4/prefix2/objectA", "testbucket", "prefix1/prefix3/objectB", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, uplink.ErrPermissionDenied)
		require.False(t, encryption.ErrMissingDecryptionBase.Has(err))
		require.False(t, encryption.ErrMissingEncryptionBase.Has(err))

		_, err = project.CopyObject(ctx, "testbucket", "prefix1/prefix2/objectA", "testbucket", "prefix1/prefix3/objectB", nil)
		require.NoError(t, err)
	})
}

func TestEndpoint_Object_MoveObjectWithDisallowedDeletes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		oldAPIKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		unrestrictedAccess, err := uplink.RequestAccessWithPassphrase(ctx, planet.Satellites[0].URL(), oldAPIKey.Serialize(), "")
		require.NoError(t, err)

		apiKey, err := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].Restrict(macaroon.Caveat{
			DisallowDeletes: true,
		})
		require.NoError(t, err)

		restrictedAccess, err := uplink.RequestAccessWithPassphrase(ctx, planet.Satellites[0].URL(), apiKey.Serialize(), "")
		require.NoError(t, err)

		planet.Uplinks[0].Access[planet.Satellites[0].ID()] = unrestrictedAccess

		expectedDataA := testrand.Bytes(7 * memory.KiB)

		// upload objectA twice to have to have version different than 1
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectA", expectedDataA)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectA", expectedDataA)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "multipleversions", "objectB", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)

		planet.Uplinks[0].Access[planet.Satellites[0].ID()] = restrictedAccess

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

		const amount = 11

		require.NoError(t, u.TestingCreateBucket(ctx, s, "test"))

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

func TestListObjects_Cursor(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		t.Run("committed", func(t *testing.T) {
			project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			_, err = project.EnsureBucket(ctx, "committed")
			require.NoError(t, err)

			expectedObjects := map[string]bool{
				"test1.dat": true,
				"test2.dat": true,
			}

			for object := range expectedObjects {
				upload, err := project.UploadObject(ctx, "committed", object, nil)
				require.NoError(t, err)
				_, err = upload.Write(make([]byte, 256))
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}

			list := project.ListObjects(ctx, "committed", nil)

			// get the first list item and make it a cursor for the next list request
			more := list.Next()
			require.True(t, more)
			require.NoError(t, list.Err())
			delete(expectedObjects, list.Item().Key)
			cursor := list.Item().Key

			// list again with cursor set to the first item from previous list request
			list = project.ListObjects(ctx, "committed", &uplink.ListObjectsOptions{Cursor: cursor})

			// expect the second item as the first item in this new list request
			more = list.Next()
			require.True(t, more)
			require.NoError(t, list.Err())
			require.NotNil(t, list.Item())
			require.False(t, list.Item().IsPrefix)
			delete(expectedObjects, list.Item().Key)

			require.Empty(t, expectedObjects)
			require.False(t, list.Next())
		})

		t.Run("pending", func(t *testing.T) {
			project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			_, err = project.EnsureBucket(ctx, "pending")
			require.NoError(t, err)

			expectedObjects := map[string]bool{
				"test1.dat": true,
				"test2.dat": true,
			}

			for object := range expectedObjects {
				_, err := project.BeginUpload(ctx, "pending", object, nil)
				require.NoError(t, err)
			}

			list := project.ListUploads(ctx, "pending", nil)

			// get the first list item and make it a cursor for the next list request
			more := list.Next()
			require.True(t, more)
			require.NoError(t, list.Err())
			delete(expectedObjects, list.Item().Key)
			cursor := list.Item().Key

			// list again with cursor set to the first item from previous list request
			list = project.ListUploads(ctx, "pending", &uplink.ListUploadsOptions{Cursor: cursor})

			// expect the second item as the first item in this new list request
			more = list.Next()
			require.True(t, more)
			require.NoError(t, list.Err())
			require.NotNil(t, list.Item())
			require.False(t, list.Item().IsPrefix)
			delete(expectedObjects, list.Item().Key)

			require.Empty(t, expectedObjects)
			require.False(t, list.Next())
		})
	})
}

func TestListUploads(t *testing.T) {
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

		require.NoError(t, u.TestingCreateBucket(ctx, s, "testbucket"))

		for i := 0; i < 10; i++ {
			_, err := project.BeginUpload(ctx, "testbucket", "object"+strconv.Itoa(i), nil)
			require.NoError(t, err)
		}

		limitCtx := testuplink.WithListLimit(ctx, 3)
		list := project.ListUploads(limitCtx, "testbucket", nil)
		items := 0
		for list.Next() {
			items++
		}
		require.NoError(t, list.Err())
		require.Equal(t, 10, items)
	})
}

func TestNodeTagPlacement(t *testing.T) {
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
						signed, err := nodetag.Sign(t.Context(), tags, satelliteIdentity)
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
			for _, sn := range planet.StorageNodes {
				sn.Contact.Chore.TriggerWait(ctx)
			}

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
			testPlacement := func(t *testing.T, bucketName string, placement int, allowedNodes func(int) bool) {
				createGeofencedBucket(t, ctx, buckets, projectID, bucketName, storj.PlacementConstraint(placement))

				const objectNo = 5
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
				testPlacement(t, "constrained", 16, func(i int) bool {
					return i%2 == 0
				})
			})
			t.Run("upload to generic excluding constrained", func(t *testing.T) {
				testPlacement(t, "generic", 0, func(i int) bool {
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

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		bucketName := "versioned-bucket"
		objectKey := "versioned-object"

		project, err := planet.Uplinks[0].OpenProject(ctx, satelliteSys)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
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
			_, err = satelliteSys.API.Metainfo.Endpoint.DownloadObject(peerctx, &pb.DownloadObjectRequest{
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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))
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

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "unversioned"))
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "versioned"))

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

func TestEndpoint_UploadObjectWithRetentionLegalHold(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		ttl := time.Hour
		ttlApiKey, err := apiKey.Restrict(macaroon.Caveat{MaxObjectTtl: &ttl})
		require.NoError(t, err)

		_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowPutRetention: true})
		require.NoError(t, err)

		restrictedLegalHoldApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowPutLegalHold: true})
		require.NoError(t, err)

		createBucket := func(t *testing.T, name string, versioned, lockEnabled bool) {
			versioning := buckets.VersioningEnabled
			if !versioned {
				versioning, lockEnabled = buckets.Unversioned, false
			}
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  project.ID,
				Versioning: versioning,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)
		}

		newBeginReq := func(apiKey *macaroon.APIKey, bucketName, key string) *pb.BeginObjectRequest {
			return &pb.BeginObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(key),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
					BlockSize:   256,
				},
			}
		}

		newBeginReqWithRetention := func(apiKey *macaroon.APIKey, bucketName, key string, mode pb.Retention_Mode) *pb.BeginObjectRequest {
			req := newBeginReq(apiKey, bucketName, key)
			req.Retention = &pb.Retention{
				Mode:        mode,
				RetainUntil: time.Now().Add(time.Hour),
			}
			return req
		}

		newBeginReqWithLegalHold := func(apiKey *macaroon.APIKey, bucketName, key string) *pb.BeginObjectRequest {
			req := newBeginReq(apiKey, bucketName, key)
			req.LegalHold = true
			return req
		}

		type uploadRequests struct {
			beginObject       *pb.BeginObjectRequest
			makeInlineSegment *pb.MakeInlineSegmentRequest
			commitObject      *pb.CommitObjectRequest
		}

		newUploadReqs := func(apiKey *macaroon.APIKey, bucketName, key string) uploadRequests {
			begin := newBeginReq(apiKey, bucketName, key)
			return uploadRequests{
				beginObject: begin,
				makeInlineSegment: &pb.MakeInlineSegmentRequest{
					Header:              begin.Header,
					Position:            &pb.SegmentPosition{},
					EncryptedKey:        begin.EncryptedObjectKey,
					EncryptedKeyNonce:   testrand.Nonce(),
					PlainSize:           512,
					EncryptedInlineData: testrand.Bytes(32),
				},
				commitObject: &pb.CommitObjectRequest{
					Header: begin.Header,
				},
			}
		}

		newUploadReqsWithRetention := func(apiKey *macaroon.APIKey, bucketName, key string, mode pb.Retention_Mode) uploadRequests {
			reqs := newUploadReqs(apiKey, bucketName, key)
			reqs.beginObject.Retention = &pb.Retention{
				Mode:        mode,
				RetainUntil: time.Now().Add(time.Hour),
			}
			return reqs
		}

		newUploadReqsWithLegalHold := func(apiKey *macaroon.APIKey, bucketName, key string) uploadRequests {
			reqs := newUploadReqs(apiKey, bucketName, key)
			reqs.beginObject.LegalHold = true
			return reqs
		}

		getObject := func(bucketName, key string) metabase.Object {
			objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objects {
				if o.Location() == (metabase.ObjectLocation{
					ProjectID:  project.ID,
					BucketName: metabase.BucketName(bucketName),
					ObjectKey:  metabase.ObjectKey(key),
				}) {
					return o
				}
			}
			return metabase.Object{}
		}

		requireObject := func(t *testing.T, bucketName, key string) metabase.Object {
			obj := getObject(bucketName, key)
			require.NotZero(t, obj)
			return obj
		}

		requireNoObject := func(t *testing.T, bucketName, key string) {
			obj := getObject(bucketName, key)
			require.Zero(t, obj)
		}

		runLockCases := func(t *testing.T, apiKey *macaroon.APIKey, bucketName string, fn func(t *testing.T, reqs uploadRequests)) {
			for _, tt := range []struct {
				name string
				mode pb.Retention_Mode
			}{
				{name: "Compliance mode", mode: pb.Retention_COMPLIANCE},
				{name: "Governance mode", mode: pb.Retention_GOVERNANCE},
			} {
				t.Run(tt.name, func(t *testing.T) {
					fn(t, newUploadReqsWithRetention(apiKey, bucketName, testrand.Path(), tt.mode))
				})
			}

			t.Run("Legal hold", func(t *testing.T) {
				fn(t, newUploadReqsWithLegalHold(apiKey, bucketName, testrand.Path()))
			})
		}

		bucketName := testrand.BucketName()
		createBucket(t, bucketName, true, true)

		t.Run("BeginObject and CommitObject", func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				key := testrand.Path()

				beginReq := newBeginReqWithRetention(apiKey, bucketName, key, pb.Retention_COMPLIANCE)
				beginResp, err := endpoint.BeginObject(ctx, beginReq)
				require.NoError(t, err)

				commitResp, err := endpoint.CommitObject(ctx, &pb.CommitObjectRequest{
					Header:   &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					StreamId: beginResp.StreamId,
				})
				require.NoError(t, err)

				require.NotNil(t, commitResp.Object.Retention)
				require.EqualValues(t, storj.ComplianceMode, commitResp.Object.Retention.Mode)
				require.WithinDuration(t, beginReq.Retention.RetainUntil, commitResp.Object.Retention.RetainUntil, time.Microsecond)

				obj := requireObject(t, bucketName, key)
				require.Equal(t, storj.ComplianceMode, obj.Retention.Mode)
				require.WithinDuration(t, beginReq.Retention.RetainUntil, obj.Retention.RetainUntil, time.Microsecond)
			})

			t.Run("Success - Legal Hold", func(t *testing.T) {
				key := testrand.Path()

				beginReq := newBeginReqWithLegalHold(apiKey, bucketName, key)

				beginResp, err := endpoint.BeginObject(ctx, beginReq)
				require.NoError(t, err)

				commitResp, err := endpoint.CommitObject(ctx, &pb.CommitObjectRequest{
					Header:   &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					StreamId: beginResp.StreamId,
				})
				require.NoError(t, err)

				require.NotNil(t, commitResp.Object.LegalHold)
				require.True(t, commitResp.Object.LegalHold.Value)

				obj := requireObject(t, bucketName, key)
				require.True(t, obj.LegalHold)
			})

			t.Run("Success - No retention period", func(t *testing.T) {
				key := testrand.Path()
				beginReq := newBeginReq(apiKey, bucketName, key)
				beginResp, err := endpoint.BeginObject(ctx, beginReq)
				require.NoError(t, err)

				commitResp, err := endpoint.CommitObject(ctx, &pb.CommitObjectRequest{
					Header:   &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					StreamId: beginResp.StreamId,
				})
				require.NoError(t, err)

				require.Nil(t, commitResp.Object.Retention)

				obj := requireObject(t, bucketName, key)
				require.Zero(t, obj.Retention)
			})

			t.Run("Object Lock not globally supported", func(t *testing.T) {
				endpoint.TestSetObjectLockEnabled(false)
				defer endpoint.TestSetObjectLockEnabled(true)

				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, err := endpoint.BeginObject(ctx, reqs.beginObject)
					rpctest.RequireCode(t, err, rpcstatus.ObjectLockEndpointsDisabled)
				})
			})

			t.Run("unversioned bucket", func(t *testing.T) {
				bucketName := testrand.BucketName()
				createBucket(t, bucketName, false, false)

				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, err := endpoint.BeginObject(ctx, reqs.beginObject)
					rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})

			t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
				bucketName := testrand.BucketName()
				createBucket(t, bucketName, true, false)

				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, err := endpoint.BeginObject(ctx, reqs.beginObject)
					rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})

			t.Run("Invalid retention mode", func(t *testing.T) {
				key := testrand.Path()
				beginReq := newBeginReqWithRetention(apiKey, bucketName, key, pb.Retention_INVALID)

				_, err = endpoint.BeginObject(ctx, beginReq)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				beginReq.Retention.Mode = pb.Retention_GOVERNANCE + 1
				_, err = endpoint.BeginObject(ctx, beginReq)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				requireNoObject(t, bucketName, key)
			})

			t.Run("Invalid retention period expiration", func(t *testing.T) {
				key := testrand.Path()
				beginReq := newBeginReqWithRetention(apiKey, bucketName, key, pb.Retention_COMPLIANCE)

				beginReq.Retention.RetainUntil = time.Time{}
				_, err = endpoint.BeginObject(ctx, beginReq)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				beginReq.Retention.RetainUntil = time.Now().Add(-time.Minute)
				_, err = endpoint.BeginObject(ctx, beginReq)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				requireNoObject(t, bucketName, key)
			})

			t.Run("Object Lock with TTL is disallowed", func(t *testing.T) {
				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					reqs.beginObject.ExpiresAt = time.Now().Add(time.Hour)
					_, err = endpoint.BeginObject(ctx, reqs.beginObject)
					rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

					reqs.beginObject.Header.ApiKey = ttlApiKey.SerializeRaw()
					_, err = endpoint.BeginObject(ctx, reqs.beginObject)
					rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})

			t.Run("Unauthorized API key", func(t *testing.T) {
				runLockCases(t, oldApiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, err = endpoint.BeginObject(ctx, reqs.beginObject)
					rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

					if reqs.beginObject.LegalHold {
						reqs.beginObject.Header.ApiKey = restrictedLegalHoldApiKey.SerializeRaw()
					} else {
						reqs.beginObject.Header.ApiKey = restrictedApiKey.SerializeRaw()
					}

					_, err = endpoint.BeginObject(ctx, reqs.beginObject)
					rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})
		})

		t.Run("CommitInlineObject", func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				key := testrand.Path()

				reqs := newUploadReqsWithRetention(apiKey, bucketName, key, pb.Retention_COMPLIANCE)
				_, _, commitResp, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
				require.NoError(t, err)

				require.NotNil(t, commitResp.Object.Retention)
				require.EqualValues(t, storj.ComplianceMode, commitResp.Object.Retention.Mode)
				require.WithinDuration(t, reqs.beginObject.Retention.RetainUntil, commitResp.Object.Retention.RetainUntil, time.Microsecond)

				obj := requireObject(t, bucketName, key)
				require.Equal(t, storj.ComplianceMode, obj.Retention.Mode)
				require.WithinDuration(t, reqs.beginObject.Retention.RetainUntil, obj.Retention.RetainUntil, time.Microsecond)
			})

			t.Run("Success - Legal Hold", func(t *testing.T) {
				key := testrand.Path()

				reqs := newUploadReqsWithLegalHold(apiKey, bucketName, key)

				_, _, commitResp, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
				require.NoError(t, err)
				require.NotNil(t, commitResp.Object.LegalHold)
				require.True(t, commitResp.Object.LegalHold.Value)

				obj := requireObject(t, bucketName, key)
				require.True(t, obj.LegalHold)
			})

			t.Run("Success - No retention period", func(t *testing.T) {
				key := testrand.Path()

				reqs := newUploadReqs(apiKey, bucketName, key)

				_, _, commitResp, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
				require.NoError(t, err)
				require.Nil(t, commitResp.Object.Retention)

				obj := requireObject(t, bucketName, key)
				require.Zero(t, obj.Retention)
			})

			t.Run("Object Lock not globally supported", func(t *testing.T) {
				endpoint.TestSetObjectLockEnabled(false)
				defer endpoint.TestSetObjectLockEnabled(true)

				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, _, _, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
					rpctest.RequireCode(t, err, rpcstatus.ObjectLockEndpointsDisabled)
					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})

			t.Run("unversioned bucket", func(t *testing.T) {
				bucketName := testrand.BucketName()
				createBucket(t, bucketName, false, false)

				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, _, _, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
					rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})

			t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
				bucketName := testrand.BucketName()
				createBucket(t, bucketName, true, false)

				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, _, _, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
					rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})

			t.Run("Invalid retention mode", func(t *testing.T) {
				key := testrand.Path()

				reqs := newUploadReqsWithRetention(apiKey, bucketName, key, pb.Retention_COMPLIANCE-1)
				_, _, _, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				reqs.beginObject.Retention.Mode = pb.Retention_GOVERNANCE + 1
				_, _, _, err = endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				requireNoObject(t, bucketName, key)
			})

			t.Run("Invalid retention period expiration", func(t *testing.T) {
				key := testrand.Path()
				reqs := newUploadReqsWithRetention(apiKey, bucketName, key, pb.Retention_COMPLIANCE)

				reqs.beginObject.Retention.RetainUntil = time.Time{}
				_, _, _, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				reqs.beginObject.Retention.RetainUntil = time.Now().Add(-time.Minute)
				_, _, _, err = endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
				rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

				requireNoObject(t, bucketName, key)
			})

			t.Run("Object Lock with TTL is disallowed", func(t *testing.T) {
				runLockCases(t, apiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					reqs.beginObject.ExpiresAt = time.Now().Add(time.Hour)
					_, _, _, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
					rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

					reqs.beginObject.Header.ApiKey = ttlApiKey.SerializeRaw()
					_, _, _, err = endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
					rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)

					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})

			t.Run("Unauthorized API key", func(t *testing.T) {
				runLockCases(t, oldApiKey, bucketName, func(t *testing.T, reqs uploadRequests) {
					_, _, _, err := endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
					rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

					if reqs.beginObject.LegalHold {
						reqs.beginObject.Header.ApiKey = restrictedLegalHoldApiKey.SerializeRaw()
					} else {
						reqs.beginObject.Header.ApiKey = restrictedApiKey.SerializeRaw()
					}

					_, _, _, err = endpoint.CommitInlineObject(ctx, reqs.beginObject, reqs.makeInlineSegment, reqs.commitObject)
					rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

					requireNoObject(t, bucketName, string(reqs.beginObject.EncryptedObjectKey))
				})
			})
		})
	})
}

func TestEndpoint_UploadObjectWithDefaultRetention(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint
		db := sat.Metabase.DB
		bucketsDB := sat.API.Buckets.Service

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		inlineSegmentReq := func(req *pb.BeginObjectRequest) *pb.MakeInlineSegmentRequest {
			return &pb.MakeInlineSegmentRequest{
				Header:              req.Header,
				Position:            &pb.SegmentPosition{},
				EncryptedKey:        req.EncryptedObjectKey,
				EncryptedKeyNonce:   testrand.Nonce(),
				PlainSize:           512,
				EncryptedInlineData: testrand.Bytes(32),
			}
		}

		type testOpts struct {
			defaultRetentionMode  storj.RetentionMode
			defaultRetentionDays  int
			defaultRetentionYears int
			overrideRetention     *pb.Retention
			expectedRetention     metabase.Retention
			commitInline          bool
		}

		test := func(t *testing.T, opts testOpts) {
			bucketName := testrand.BucketName()
			_, err = bucketsDB.CreateBucket(ctx, buckets.Bucket{
				Name:       bucketName,
				ProjectID:  project.ID,
				Versioning: buckets.VersioningEnabled,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled:               true,
					DefaultRetentionMode:  opts.defaultRetentionMode,
					DefaultRetentionDays:  opts.defaultRetentionDays,
					DefaultRetentionYears: opts.defaultRetentionYears,
				},
			})
			require.NoError(t, err)

			objectKey := testrand.Path()

			req := &pb.BeginObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(objectKey),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_AESGCM,
					BlockSize:   256,
				},
				Retention: opts.overrideRetention,
			}

			if opts.commitInline {
				_, _, _, err := endpoint.CommitInlineObject(ctx, req, inlineSegmentReq(req), &pb.CommitObjectRequest{
					Header: req.Header,
				})
				require.NoError(t, err)
			} else {
				beginResp, err := endpoint.BeginObject(ctx, req)
				require.NoError(t, err)

				_, err = endpoint.CommitObject(ctx, &pb.CommitObjectRequest{
					Header:   &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					StreamId: beginResp.StreamId,
				})
				require.NoError(t, err)
			}

			retention, err := db.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
				ObjectLocation: metabase.ObjectLocation{
					ProjectID:  project.ID,
					BucketName: metabase.BucketName(bucketName),
					ObjectKey:  metabase.ObjectKey(objectKey),
				},
			})
			require.NoError(t, err)

			require.Equal(t, opts.expectedRetention.Mode, retention.Mode)
			require.WithinDuration(t, opts.expectedRetention.RetainUntil, retention.RetainUntil, time.Minute)
		}

		t.Run("Use default retention", func(t *testing.T) {
			opts := testOpts{
				defaultRetentionMode: storj.ComplianceMode,
				defaultRetentionDays: 3,
				expectedRetention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Now().AddDate(0, 0, 3),
				},
			}

			t.Run("Days, Compliance mode, CommitObject", func(t *testing.T) {
				test(t, opts)
			})

			t.Run("Days, Compliance mode, CommitInlineObject", func(t *testing.T) {
				opts.commitInline = true
				test(t, opts)
			})

			opts = testOpts{
				defaultRetentionMode:  storj.GovernanceMode,
				defaultRetentionYears: 5,
				expectedRetention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: time.Now().AddDate(5, 0, 0),
				},
			}

			t.Run("Years, Governance mode, CommitObject", func(t *testing.T) {
				test(t, opts)
			})

			t.Run("Years, Governance mode, CommitInlineObject", func(t *testing.T) {
				opts.commitInline = true
				test(t, opts)
			})

			t.Run("Leap year", func(t *testing.T) {
				// Find the nearest date N years after the current date that lies after a leap day.
				now := time.Now()
				leapYear := now.Year()
				var leapDay time.Time
				for {
					if (leapYear%4 == 0 && leapYear%100 != 0) || (leapYear%400 == 0) {
						leapDay = time.Date(leapYear, time.February, 29, 0, 0, 0, 0, time.UTC)
						if leapDay.After(now) {
							break
						}
					}
					leapYear++
				}
				years := leapYear - now.Year()
				if now.AddDate(years, 0, 0).Before(leapDay) {
					years++
				}

				// Expect 1 day to always be considered a 24-hour period, with no adjustments
				// made to accommodate the leap day.
				opts := testOpts{
					defaultRetentionMode: storj.ComplianceMode,
					defaultRetentionDays: 365 * years,
					expectedRetention: metabase.Retention{
						Mode:        storj.ComplianceMode,
						RetainUntil: time.Now().AddDate(0, 0, 365*years),
					},
				}

				t.Run("Days, CommitObject", func(t *testing.T) {
					test(t, opts)
				})

				t.Run("Days, CommitInlineObject", func(t *testing.T) {
					opts.commitInline = true
					test(t, opts)
				})

				// Expect the retention period duration to take the leap day into account.
				opts = testOpts{
					defaultRetentionMode:  storj.ComplianceMode,
					defaultRetentionYears: years,
					expectedRetention: metabase.Retention{
						Mode:        storj.ComplianceMode,
						RetainUntil: time.Now().AddDate(0, 0, 365*years+1),
					},
				}

				t.Run("Years, CommitObject", func(t *testing.T) {
					test(t, opts)
				})

				t.Run("Years, CommitInlineObject", func(t *testing.T) {
					opts.commitInline = true
					test(t, opts)
				})
			})
		})

		t.Run("Override default retention", func(t *testing.T) {
			opts := testOpts{
				defaultRetentionMode:  storj.ComplianceMode,
				defaultRetentionYears: 3,
				overrideRetention: &pb.Retention{
					Mode:        pb.Retention_GOVERNANCE,
					RetainUntil: time.Now().AddDate(0, 0, 5),
				},
				expectedRetention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: time.Now().AddDate(0, 0, 5),
				},
				commitInline: false,
			}

			t.Run("CommitObject", func(t *testing.T) {
				test(t, opts)
			})

			t.Run("CommitInlineObject", func(t *testing.T) {
				opts.commitInline = true
				test(t, opts)
			})
		})

		t.Run("TTL is disallowed", func(t *testing.T) {
			test := func(t *testing.T, commitInline bool) {
				bucketName := testrand.BucketName()
				_, err = bucketsDB.CreateBucket(ctx, buckets.Bucket{
					Name:       bucketName,
					ProjectID:  project.ID,
					Versioning: buckets.VersioningEnabled,
					ObjectLock: buckets.ObjectLockSettings{
						Enabled:              true,
						DefaultRetentionMode: storj.ComplianceMode,
						DefaultRetentionDays: 7,
					},
				})
				require.NoError(t, err)

				objectKey := testrand.Path()
				ttl := time.Hour
				req := &pb.BeginObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Bucket:             []byte(bucketName),
					EncryptedObjectKey: []byte(objectKey),
					EncryptionParameters: &pb.EncryptionParameters{
						CipherSuite: pb.CipherSuite_ENC_AESGCM,
						BlockSize:   256,
					},
					ExpiresAt: time.Now().Add(ttl),
				}

				if commitInline {
					_, _, _, err = endpoint.CommitInlineObject(ctx, req, inlineSegmentReq(req), &pb.CommitObjectRequest{
						Header: req.Header,
					})
				} else {
					_, err = endpoint.BeginObject(ctx, req)
				}
				rpctest.RequireCode(t, err, rpcstatus.ObjectLockUploadWithTTLAndDefaultRetention)

				ttlApiKey, err := apiKey.Restrict(macaroon.Caveat{MaxObjectTtl: &ttl})
				require.NoError(t, err)
				req.Header.ApiKey = ttlApiKey.SerializeRaw()
				req.ExpiresAt = time.Time{}

				if commitInline {
					_, _, _, err = endpoint.CommitInlineObject(ctx, req, inlineSegmentReq(req), &pb.CommitObjectRequest{
						Header: req.Header,
					})
				} else {
					_, err = endpoint.BeginObject(ctx, req)
				}
				rpctest.RequireCode(t, err, rpcstatus.ObjectLockUploadWithTTLAPIKeyAndDefaultRetention)
			}

			t.Run("BeginObject", func(t *testing.T) {
				test(t, false)
			})

			t.Run("CommitInlineObject", func(t *testing.T) {
				test(t, true)
			})
		})
	})
}

func TestEndpoint_GetObjectLegalHold(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint
		db := sat.Metabase.DB

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		createObject := func(t *testing.T, objStream metabase.ObjectStream, legalHold bool) metabase.Object {
			object, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    legalHold,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)
			return object
		}

		createBucket := func(t *testing.T, versioned, lockEnabled bool) string {
			versioning := buckets.VersioningEnabled
			if !versioned {
				versioning, lockEnabled = buckets.Unversioned, false
			}
			name := testrand.BucketName()
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  project.ID,
				Versioning: versioning,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)
			return name
		}

		lockBucketName := createBucket(t, true, true)

		t.Run("Success", func(t *testing.T) {
			objStream1 := randObjectStream(project.ID, lockBucketName)
			object1 := createObject(t, objStream1, true)

			objStream2 := objStream1
			objStream2.Version++
			createObject(t, objStream2, false)

			req := &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream1.BucketName),
				EncryptedObjectKey: []byte(objStream1.ObjectKey),
				ObjectVersion:      object1.StreamVersionID().Bytes(),
			}

			// exact version
			resp, err := endpoint.GetObjectLegalHold(ctx, req)
			require.NoError(t, err)
			require.True(t, resp.Enabled)

			// last committed version
			req.ObjectVersion = nil
			resp, err = endpoint.GetObjectLegalHold(ctx, req)
			require.NoError(t, err)
			require.False(t, resp.Enabled)
		})

		t.Run("Missing bucket", func(t *testing.T) {
			bucketName := testrand.BucketName()
			resp, err := endpoint.GetObjectLegalHold(ctx, &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(metabasetest.RandObjectKey()),
			})
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.NotFound, "bucket not found: "+bucketName)
		})

		t.Run("Missing object", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)

			req := &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
			}

			// last committed version
			resp, err := endpoint.GetObjectLegalHold(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireStatusContains(t, err, rpcstatus.NotFound, "object not found")

			// exact version
			createObject(t, objStream, true)
			req.ObjectVersion = metabase.NewStreamVersionID(randVersion(), testrand.UUID()).Bytes()
			resp, err = endpoint.GetObjectLegalHold(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireStatusContains(t, err, rpcstatus.NotFound, "object not found")
		})

		t.Run("Delete marker", func(t *testing.T) {
			objStream1 := randObjectStream(project.ID, lockBucketName)
			createObject(t, objStream1, true)

			deleteOpts := metainfo.DeleteCommittedObject{
				ObjectLocation: metabase.ObjectLocation{
					ObjectKey:  objStream1.ObjectKey,
					ProjectID:  project.ID,
					BucketName: objStream1.BucketName,
				},
				Version: []byte{},
			}
			result, err := endpoint.DeleteCommittedObject(ctx, deleteOpts)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result[0])

			req := &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream1.BucketName),
				EncryptedObjectKey: []byte(objStream1.ObjectKey),
				ObjectVersion:      result[0].ObjectVersion,
			}

			// exact version
			resp, err := endpoint.GetObjectLegalHold(ctx, req)
			require.Error(t, err)
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			objStream2 := randObjectStream(project.ID, lockBucketName)
			createObject(t, objStream2, false)
			objStream2.Version++
			createObject(t, objStream2, true)

			deleteOpts.ObjectKey = objStream2.ObjectKey
			deleteOpts.BucketName = objStream2.BucketName

			_, err = endpoint.DeleteCommittedObject(ctx, deleteOpts)
			require.NoError(t, err)

			req.Bucket = []byte(objStream2.BucketName)
			req.EncryptedObjectKey = []byte(objStream2.ObjectKey)
			req.ObjectVersion = nil

			// last committed version
			resp, err = endpoint.GetObjectLegalHold(ctx, req)
			require.Error(t, err)
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		})

		t.Run("Pending object", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)
			pending, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: objStream,
				Encryption:   metabasetest.DefaultEncryption,
				LegalHold:    true,
			})
			require.NoError(t, err)

			req := &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      pending.StreamVersionID().Bytes(),
			}

			// exact version
			_, err = endpoint.GetObjectLegalHold(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			// last committed version
			req.ObjectVersion = nil
			_, err = endpoint.GetObjectLegalHold(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			_, err = db.CommitObject(ctx, metabase.CommitObject{ObjectStream: objStream})
			require.NoError(t, err)

			pendingObjStream := objStream
			pendingObjStream.Version++
			_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: pendingObjStream,
				Encryption:   metabasetest.DefaultEncryption,
				LegalHold:    false,
			})
			require.NoError(t, err)

			// Ensure that the pending object is skipped despite being newer
			// than the committed one.
			resp, err := endpoint.GetObjectLegalHold(ctx, req)
			require.NoError(t, err)
			require.True(t, resp.Enabled)
		})

		t.Run("unversioned bucket", func(t *testing.T) {
			bucketName := createBucket(t, false, false)
			resp, err := endpoint.GetObjectLegalHold(ctx, &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(metabasetest.RandObjectKey()),
			})
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
		})

		t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
			bucketName := createBucket(t, true, false)
			resp, err := endpoint.GetObjectLegalHold(ctx, &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(metabasetest.RandObjectKey()),
			})
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			objStream := randObjectStream(project.ID, lockBucketName)
			object := createObject(t, objStream, true)
			req := &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(object.BucketName),
				EncryptedObjectKey: []byte(object.ObjectKey),
				ObjectVersion:      object.StreamVersionID().Bytes(),
			}
			resp, err := endpoint.GetObjectLegalHold(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockEndpointsDisabled, "Object Lock feature is not enabled")
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			objStream := randObjectStream(project.ID, lockBucketName)
			object := createObject(t, objStream, true)
			req := &pb.GetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: oldApiKey.SerializeRaw()},
				Bucket:             []byte(object.BucketName),
				EncryptedObjectKey: []byte(object.ObjectKey),
				ObjectVersion:      object.StreamVersionID().Bytes(),
			}
			resp, err := endpoint.GetObjectLegalHold(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

			restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{
				DisallowGetLegalHold: true,
			})
			require.NoError(t, err)

			req.Header.ApiKey = restrictedApiKey.SerializeRaw()
			resp, err = endpoint.GetObjectLegalHold(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
		})
	})
}

func TestEndpoint_SetObjectLegalHold(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint
		db := sat.Metabase.DB

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		requireLegalHold := func(
			t *testing.T, loc metabase.ObjectLocation, version metabase.Version, expectedLegalHold bool,
		) {
			objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objects {
				if o.Location() == loc && o.Version == version {
					require.Equal(t, expectedLegalHold, o.LegalHold)
					return
				}
			}
			t.Fatal("object not found")
		}

		createObject := func(t *testing.T, objStream metabase.ObjectStream, legalHold bool) metabase.Object {
			object, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    legalHold,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)
			return object
		}

		createBucket := func(t *testing.T, versioned, lockEnabled bool) string {
			versioning := buckets.VersioningEnabled
			if !versioned {
				versioning, lockEnabled = buckets.Unversioned, false
			}
			name := testrand.BucketName()
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  project.ID,
				Versioning: versioning,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)
			return name
		}

		lockBucketName := createBucket(t, true, true)

		t.Run("Set legal hold", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)
			obj1 := createObject(t, objStream, false)
			objStream.Version++
			obj2 := createObject(t, objStream, false)

			// exact version
			_, err := endpoint.SetObjectLegalHold(ctx, &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      obj1.StreamVersionID().Bytes(),
				Enabled:            true,
			})
			require.NoError(t, err)

			requireLegalHold(t, objStream.Location(), obj1.Version, true)
			requireLegalHold(t, objStream.Location(), obj2.Version, false)

			// last committed version
			_, err = endpoint.SetObjectLegalHold(ctx, &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				Enabled:            true,
			})
			require.NoError(t, err)

			requireLegalHold(t, objStream.Location(), obj1.Version, true)
			requireLegalHold(t, objStream.Location(), obj2.Version, true)
		})

		t.Run("Remove legal hold", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)
			obj := createObject(t, objStream, true)

			_, err := endpoint.SetObjectLegalHold(ctx, &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Enabled:            false,
			})
			require.NoError(t, err)
			requireLegalHold(t, objStream.Location(), objStream.Version, false)
		})

		t.Run("Missing bucket", func(t *testing.T) {
			objStream := randObjectStream(project.ID, testrand.BucketName())
			obj := createObject(t, objStream, false)

			_, err := endpoint.SetObjectLegalHold(ctx, &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Enabled:            true,
			})
			rpctest.RequireStatus(t, err, rpcstatus.NotFound, "bucket not found: "+string(objStream.BucketName))
			requireLegalHold(t, objStream.Location(), objStream.Version, false)
		})

		t.Run("Missing object", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)

			req := &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				Enabled:            true,
			}

			// last committed version
			_, err := endpoint.SetObjectLegalHold(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			createObject(t, objStream, false)

			// exact version
			req.ObjectVersion = metabase.NewStreamVersionID(randVersion(), testrand.UUID()).Bytes()
			_, err = endpoint.SetObjectLegalHold(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			requireLegalHold(t, objStream.Location(), objStream.Version, false)
		})

		t.Run("Pending object", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)
			pending, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: objStream,
				Encryption:   metabasetest.DefaultEncryption,
			})
			require.NoError(t, err)

			req := &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      pending.StreamVersionID().Bytes(),
				Enabled:            true,
			}

			// exact version
			_, err = endpoint.SetObjectLegalHold(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			// last committed version
			req.ObjectVersion = nil
			_, err = endpoint.SetObjectLegalHold(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			committed, err := db.CommitObject(ctx, metabase.CommitObject{ObjectStream: objStream})
			require.NoError(t, err)

			pendingObjStream := objStream
			pendingObjStream.Version++
			pending, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: pendingObjStream,
				Encryption:   metabasetest.DefaultEncryption,
			})
			require.NoError(t, err)

			_, err = endpoint.SetObjectLegalHold(ctx, req)
			require.NoError(t, err)

			// Ensure that the pending object is skipped despite being newer
			// than the committed one.
			requireLegalHold(t, objStream.Location(), pending.Version, false)
			requireLegalHold(t, objStream.Location(), committed.Version, true)
		})

		t.Run("Delete marker", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)

			metabasetest.CreateObject(ctx, t, db, objStream, 0)
			deleteResult, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: objStream.Location(),
				Versioned:      true,
			})
			require.NoError(t, err)

			req := &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      deleteResult.Markers[0].StreamVersionID().Bytes(),
				Enabled:            true,
			}

			// exact version
			_, err = endpoint.SetObjectLegalHold(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			// last committed version
			req.ObjectVersion = nil
			_, err = endpoint.SetObjectLegalHold(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		})

		t.Run("unversioned bucket", func(t *testing.T) {
			bucketName := createBucket(t, false, false)
			obj := createObject(t, randObjectStream(project.ID, bucketName), false)

			_, err := endpoint.SetObjectLegalHold(ctx, &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Enabled:            true,
			})
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
			requireLegalHold(t, obj.Location(), obj.Version, false)
		})

		t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
			bucketName := createBucket(t, true, false)
			obj := createObject(t, randObjectStream(project.ID, bucketName), false)

			_, err := endpoint.SetObjectLegalHold(ctx, &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Enabled:            true,
			})
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
			requireLegalHold(t, obj.Location(), obj.Version, false)
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			objStream := randObjectStream(project.ID, lockBucketName)
			obj := createObject(t, objStream, false)

			req := &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Enabled:            true,
			}

			_, err := endpoint.SetObjectLegalHold(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockEndpointsDisabled, "Object Lock feature is not enabled")
			requireLegalHold(t, obj.Location(), obj.Version, false)
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			objStream := randObjectStream(project.ID, lockBucketName)
			obj := createObject(t, objStream, false)

			req := &pb.SetObjectLegalHoldRequest{
				Header:             &pb.RequestHeader{ApiKey: oldApiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Enabled:            true,
			}

			_, err = endpoint.SetObjectLegalHold(ctx, req)
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

			restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowPutLegalHold: true})
			require.NoError(t, err)

			req.Header.ApiKey = restrictedApiKey.SerializeRaw()
			_, err = endpoint.SetObjectLegalHold(ctx, req)
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
		})
	})
}

func TestEndpoint_GetObjectRetention(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint
		db := sat.Metabase.DB

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		requireEqualRetention := func(t *testing.T, expected metabase.Retention, actual *pb.Retention) {
			require.NotNil(t, actual)
			require.EqualValues(t, expected.Mode, actual.Mode)
			require.WithinDuration(t, expected.RetainUntil, actual.RetainUntil, time.Microsecond)
		}

		createObject := func(t *testing.T, objStream metabase.ObjectStream, retention metabase.Retention) metabase.Object {
			object, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)
			return object
		}

		createBucket := func(t *testing.T, versioned, lockEnabled bool) string {
			versioning := buckets.VersioningEnabled
			if !versioned {
				versioning, lockEnabled = buckets.Unversioned, false
			}
			name := testrand.BucketName()
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  project.ID,
				Versioning: versioning,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)
			return name
		}

		lockBucketName := createBucket(t, true, true)

		t.Run("Success", func(t *testing.T) {
			objStream1 := randObjectStream(project.ID, lockBucketName)
			retention1 := metabase.Retention{
				Mode:        storj.GovernanceMode,
				RetainUntil: time.Now().Add(time.Hour),
			}
			object1 := createObject(t, objStream1, retention1)

			objStream2 := objStream1
			objStream2.Version++
			retention2 := randRetention(storj.ComplianceMode)
			createObject(t, objStream2, retention2)

			req := &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream1.BucketName),
				EncryptedObjectKey: []byte(objStream1.ObjectKey),
				ObjectVersion:      object1.StreamVersionID().Bytes(),
			}

			// exact version
			resp, err := endpoint.GetObjectRetention(ctx, req)
			require.NoError(t, err)
			requireEqualRetention(t, retention1, resp.Retention)

			// last committed version
			req.ObjectVersion = nil
			resp, err = endpoint.GetObjectRetention(ctx, req)
			require.NoError(t, err)
			requireEqualRetention(t, retention2, resp.Retention)
		})

		t.Run("Missing bucket", func(t *testing.T) {
			bucketName := testrand.BucketName()
			resp, err := endpoint.GetObjectRetention(ctx, &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(metabasetest.RandObjectKey()),
			})
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.NotFound, "bucket not found: "+bucketName)
		})

		t.Run("Missing object", func(t *testing.T) {
			objStream, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.ComplianceMode)

			req := &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
			}

			// last committed version
			resp, err := endpoint.GetObjectRetention(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireStatusContains(t, err, rpcstatus.NotFound, "object not found")

			// exact version
			createObject(t, objStream, retention)
			req.ObjectVersion = metabase.NewStreamVersionID(randVersion(), testrand.UUID()).Bytes()
			resp, err = endpoint.GetObjectRetention(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireStatusContains(t, err, rpcstatus.NotFound, "object not found")
		})

		t.Run("Delete marker", func(t *testing.T) {
			objStream1, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.ComplianceMode)
			createObject(t, objStream1, retention)

			deleteOpts := metainfo.DeleteCommittedObject{
				ObjectLocation: metabase.ObjectLocation{
					ObjectKey:  objStream1.ObjectKey,
					ProjectID:  project.ID,
					BucketName: objStream1.BucketName,
				},
				Version: []byte{},
			}
			result, err := endpoint.DeleteCommittedObject(ctx, deleteOpts)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result[0])

			req := &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream1.BucketName),
				EncryptedObjectKey: []byte(objStream1.ObjectKey),
				ObjectVersion:      result[0].ObjectVersion,
			}

			// exact version
			resp, err := endpoint.GetObjectRetention(ctx, req)
			require.Error(t, err)
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			objStream2 := randObjectStream(project.ID, lockBucketName)
			createObject(t, objStream2, retention)
			objStream2.Version++
			createObject(t, objStream2, retention)

			deleteOpts.BucketName = objStream2.BucketName
			deleteOpts.ObjectKey = objStream2.ObjectKey

			_, err = endpoint.DeleteCommittedObject(ctx, deleteOpts)
			require.NoError(t, err)

			req.Bucket = []byte(objStream2.BucketName)
			req.EncryptedObjectKey = []byte(objStream2.ObjectKey)
			req.ObjectVersion = nil

			// last committed version
			resp, err = endpoint.GetObjectRetention(ctx, req)
			require.Error(t, err)
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		})

		t.Run("Pending object", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)
			retention := randRetention(storj.ComplianceMode)
			pending, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: objStream,
				Encryption:   metabasetest.DefaultEncryption,
				Retention:    retention,
			})
			require.NoError(t, err)

			req := &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      pending.StreamVersionID().Bytes(),
			}

			// exact version
			_, err = endpoint.GetObjectRetention(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			// last committed version
			req.ObjectVersion = nil
			_, err = endpoint.GetObjectRetention(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			_, err = db.CommitObject(ctx, metabase.CommitObject{ObjectStream: objStream})
			require.NoError(t, err)

			pendingObjStream := objStream
			pendingObjStream.Version++
			_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: pendingObjStream,
				Encryption:   metabasetest.DefaultEncryption,
				Retention:    randRetention(storj.ComplianceMode),
			})
			require.NoError(t, err)

			// Ensure that the pending object is skipped despite being newer
			// than the committed one.
			resp, err := endpoint.GetObjectRetention(ctx, req)
			require.NoError(t, err)
			requireEqualRetention(t, retention, resp.Retention)
		})

		t.Run("Delete marker", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)

			metabasetest.CreateObject(ctx, t, db, objStream, 0)
			deleteResult, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: objStream.Location(),
				Versioned:      true,
			})
			require.NoError(t, err)

			req := &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      deleteResult.Markers[0].StreamVersionID().Bytes(),
			}

			// exact version
			_, err = endpoint.GetObjectRetention(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			// last committed version
			req.ObjectVersion = nil
			_, err = endpoint.GetObjectRetention(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		})

		t.Run("Missing retention period", func(t *testing.T) {
			object := createObject(t, randObjectStream(project.ID, lockBucketName), metabase.Retention{})

			resp, err := endpoint.GetObjectRetention(ctx, &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(object.BucketName),
				EncryptedObjectKey: []byte(object.ObjectKey),
				ObjectVersion:      object.StreamVersionID().Bytes(),
			})
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectRetentionConfigurationMissing, "object does not have a retention configuration")
		})

		t.Run("unversioned bucket", func(t *testing.T) {
			bucketName := createBucket(t, false, false)
			resp, err := endpoint.GetObjectRetention(ctx, &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(metabasetest.RandObjectKey()),
			})
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
		})

		t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
			bucketName := createBucket(t, true, false)
			resp, err := endpoint.GetObjectRetention(ctx, &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(bucketName),
				EncryptedObjectKey: []byte(metabasetest.RandObjectKey()),
			})
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			objStream, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.ComplianceMode)
			object := createObject(t, objStream, retention)
			req := &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(object.BucketName),
				EncryptedObjectKey: []byte(object.ObjectKey),
				ObjectVersion:      object.StreamVersionID().Bytes(),
			}
			resp, err := endpoint.GetObjectRetention(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockEndpointsDisabled, "Object Lock feature is not enabled")
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			objStream, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.ComplianceMode)
			object := createObject(t, objStream, retention)
			req := &pb.GetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: oldApiKey.SerializeRaw()},
				Bucket:             []byte(object.BucketName),
				EncryptedObjectKey: []byte(object.ObjectKey),
				ObjectVersion:      object.StreamVersionID().Bytes(),
			}
			resp, err := endpoint.GetObjectRetention(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)

			restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{
				DisallowGetRetention: true,
				DisallowPutRetention: true, // GetRetention is implicitly allowed if PutRetention is allowed
			})
			require.NoError(t, err)

			req.Header.ApiKey = restrictedApiKey.SerializeRaw()
			resp, err = endpoint.GetObjectRetention(ctx, req)
			require.Nil(t, resp)
			rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
		})
	})
}

func TestEndpoint_SetObjectRetention(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint
		db := sat.Metabase.DB

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		requireRetention := func(
			t *testing.T, loc metabase.ObjectLocation, version metabase.Version, expectedRetention metabase.Retention,
		) {
			objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objects {
				if o.Location() == loc && o.Version == version {
					require.Equal(t, expectedRetention.Mode, o.Retention.Mode)
					require.WithinDuration(t, expectedRetention.RetainUntil, o.Retention.RetainUntil, time.Microsecond)
					return
				}
			}
			t.Fatal("object not found")
		}

		createObject := func(t *testing.T, objStream metabase.ObjectStream, retention metabase.Retention) metabase.Object {
			object, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)
			return object
		}

		createBucket := func(t *testing.T, versioned, lockEnabled bool) string {
			versioning := buckets.VersioningEnabled
			if !versioned {
				versioning, lockEnabled = buckets.Unversioned, false
			}
			name := testrand.BucketName()
			_, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  project.ID,
				Versioning: versioning,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)
			return name
		}

		testCases := []struct {
			name string
			mode storj.RetentionMode
		}{
			{name: "Compliance mode", mode: storj.ComplianceMode},
			{name: "Governance mode", mode: storj.GovernanceMode},
		}

		lockBucketName := createBucket(t, true, true)

		t.Run("Set retention", func(t *testing.T) {
			for _, tt := range testCases {
				t.Run(tt.name, func(t *testing.T) {
					objStream := randObjectStream(project.ID, lockBucketName)
					obj1 := createObject(t, objStream, metabase.Retention{})
					objStream.Version++
					obj2 := createObject(t, objStream, metabase.Retention{})

					// exact version
					expectedRetention1 := randRetention(tt.mode)
					_, err := endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
						Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:             []byte(objStream.BucketName),
						EncryptedObjectKey: []byte(objStream.ObjectKey),
						ObjectVersion:      obj1.StreamVersionID().Bytes(),
						Retention:          retentionToProto(expectedRetention1),
					})
					require.NoError(t, err)

					// Ensure that retention periods can be lengthened.
					expectedRetention1.RetainUntil = expectedRetention1.RetainUntil.Add(time.Hour)
					_, err = endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
						Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:             []byte(objStream.BucketName),
						EncryptedObjectKey: []byte(objStream.ObjectKey),
						ObjectVersion:      obj1.StreamVersionID().Bytes(),
						Retention:          retentionToProto(expectedRetention1),
					})
					require.NoError(t, err)

					// last committed version
					expectedRetention2 := randRetention(tt.mode)
					_, err = endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
						Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:             []byte(objStream.BucketName),
						EncryptedObjectKey: []byte(objStream.ObjectKey),
						Retention:          retentionToProto(expectedRetention2),
					})
					require.NoError(t, err)

					requireRetention(t, objStream.Location(), obj1.Version, expectedRetention1)
					requireRetention(t, objStream.Location(), obj2.Version, expectedRetention2)
				})
			}
		})

		t.Run("Remove retention", func(t *testing.T) {
			for _, tt := range testCases {
				t.Run(tt.name, func(t *testing.T) {
					noRetentionObj := createObject(t, randObjectStream(project.ID, lockBucketName), metabase.Retention{})

					_, err = endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
						Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:             []byte(noRetentionObj.BucketName),
						EncryptedObjectKey: []byte(noRetentionObj.ObjectKey),
						ObjectVersion:      noRetentionObj.StreamVersionID().Bytes(),
					})
					require.NoError(t, err)
					requireRetention(t, noRetentionObj.Location(), noRetentionObj.Version, noRetentionObj.Retention)

					expiredRetentionObj := createObject(t, randObjectStream(project.ID, lockBucketName), metabase.Retention{
						Mode:        tt.mode,
						RetainUntil: time.Now().Add(-time.Hour),
					})

					_, err = endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
						Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:             []byte(expiredRetentionObj.BucketName),
						EncryptedObjectKey: []byte(expiredRetentionObj.ObjectKey),
						ObjectVersion:      expiredRetentionObj.StreamVersionID().Bytes(),
					})
					require.NoError(t, err)
					requireRetention(t, expiredRetentionObj.Location(), expiredRetentionObj.Version, metabase.Retention{})

					activeRetentionObj := createObject(t, randObjectStream(project.ID, lockBucketName), randRetention(tt.mode))

					opts := &pb.SetObjectRetentionRequest{
						Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:             []byte(activeRetentionObj.BucketName),
						EncryptedObjectKey: []byte(activeRetentionObj.ObjectKey),
						ObjectVersion:      activeRetentionObj.StreamVersionID().Bytes(),
					}

					_, err := endpoint.SetObjectRetention(ctx, opts)
					rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
					requireRetention(t, activeRetentionObj.Location(), activeRetentionObj.Version, activeRetentionObj.Retention)

					opts.BypassGovernanceRetention = true

					_, err = endpoint.SetObjectRetention(ctx, opts)
					if tt.mode == storj.GovernanceMode {
						require.NoError(t, err)
						requireRetention(t, activeRetentionObj.Location(), activeRetentionObj.Version, metabase.Retention{})
					} else {
						rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
						requireRetention(t, activeRetentionObj.Location(), activeRetentionObj.Version, activeRetentionObj.Retention)
					}
				})
			}
		})

		t.Run("Shorten retention", func(t *testing.T) {
			for _, tt := range testCases {
				t.Run(tt.name, func(t *testing.T) {
					obj := createObject(t, randObjectStream(project.ID, lockBucketName), randRetention(tt.mode))

					newRetention := obj.Retention
					newRetention.RetainUntil = newRetention.RetainUntil.Add(-time.Minute)

					opts := &pb.SetObjectRetentionRequest{
						Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
						Bucket:             []byte(obj.BucketName),
						EncryptedObjectKey: []byte(obj.ObjectKey),
						ObjectVersion:      obj.StreamVersionID().Bytes(),
						Retention:          retentionToProto(newRetention),
					}

					_, err := endpoint.SetObjectRetention(ctx, opts)
					rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
					requireRetention(t, obj.Location(), obj.Version, obj.Retention)

					opts.BypassGovernanceRetention = true

					_, err = endpoint.SetObjectRetention(ctx, opts)
					if tt.mode == storj.GovernanceMode {
						require.NoError(t, err)
						requireRetention(t, obj.Location(), obj.Version, newRetention)
					} else {
						rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
						requireRetention(t, obj.Location(), obj.Version, obj.Retention)
					}
				})
			}
		})

		t.Run("Change retention mode", func(t *testing.T) {
			retention := randRetention(storj.GovernanceMode)
			obj := createObject(t, randObjectStream(project.ID, lockBucketName), retention)

			newRetention := retention
			newRetention.Mode = storj.ComplianceMode

			opts := &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Retention:          retentionToProto(newRetention),
			}

			// Governance mode shouldn't be able to be switched to compliance mode without BypassGovernanceRetention.
			_, err := endpoint.SetObjectRetention(ctx, opts)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
			requireRetention(t, obj.Location(), obj.Version, retention)

			opts.BypassGovernanceRetention = true

			_, err = endpoint.SetObjectRetention(ctx, opts)
			require.NoError(t, err)
			requireRetention(t, obj.Location(), obj.Version, newRetention)

			// Compliance mode shouldn't be able to be switched to governance mode regardless of BypassGovernanceRetention.
			opts.Retention.Mode = pb.Retention_GOVERNANCE
			opts.BypassGovernanceRetention = false

			_, err = endpoint.SetObjectRetention(ctx, opts)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
			requireRetention(t, obj.Location(), obj.Version, newRetention)

			opts.BypassGovernanceRetention = true

			_, err = endpoint.SetObjectRetention(ctx, opts)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
			requireRetention(t, obj.Location(), obj.Version, newRetention)
		})

		t.Run("Invalid retention", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)
			obj := createObject(t, objStream, metabase.Retention{})

			check := func(retention *pb.Retention, errText string) {
				_, err := endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Bucket:             []byte(objStream.BucketName),
					EncryptedObjectKey: []byte(objStream.ObjectKey),
					ObjectVersion:      obj.StreamVersionID().Bytes(),
					Retention:          retention,
				})
				rpctest.RequireStatus(t, err, rpcstatus.InvalidArgument, errText)
			}

			check(&pb.Retention{}, "invalid retention mode 0")

			check(&pb.Retention{
				Mode: pb.Retention_COMPLIANCE,
			}, "retention period expiration time must be set if retention is set")

			check(&pb.Retention{
				Mode:        pb.Retention_COMPLIANCE,
				RetainUntil: time.Now().Add(-time.Minute),
			}, "retention period expiration time must not be in the past")

			check(&pb.Retention{
				Mode:        pb.Retention_GOVERNANCE + 1,
				RetainUntil: time.Now().Add(time.Hour),
			}, "invalid retention mode 3")

			requireRetention(t, objStream.Location(), objStream.Version, metabase.Retention{})
		})

		t.Run("Missing bucket", func(t *testing.T) {
			objStream := randObjectStream(project.ID, testrand.BucketName())
			obj := createObject(t, objStream, metabase.Retention{})

			_, err := endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Retention:          retentionToProto(randRetention(storj.ComplianceMode)),
			})
			rpctest.RequireStatus(t, err, rpcstatus.NotFound, "bucket not found: "+string(objStream.BucketName))

			requireRetention(t, objStream.Location(), objStream.Version, metabase.Retention{})
		})

		t.Run("Missing object", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)

			req := &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				Retention:          retentionToProto(randRetention(storj.ComplianceMode)),
			}

			// last committed version
			_, err := endpoint.SetObjectRetention(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			createObject(t, objStream, metabase.Retention{})

			// exact version
			req.ObjectVersion = metabase.NewStreamVersionID(randVersion(), testrand.UUID()).Bytes()
			_, err = endpoint.SetObjectRetention(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			requireRetention(t, objStream.Location(), objStream.Version, metabase.Retention{})
		})

		t.Run("Pending object", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)
			pending, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: objStream,
				Encryption:   metabasetest.DefaultEncryption,
			})
			require.NoError(t, err)

			retention := randRetention(storj.ComplianceMode)
			req := &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      pending.StreamVersionID().Bytes(),
				Retention:          retentionToProto(retention),
			}

			// exact version
			_, err = endpoint.SetObjectRetention(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			// last committed version
			req.ObjectVersion = nil
			_, err = endpoint.SetObjectRetention(ctx, req)
			rpctest.AssertStatusContains(t, err, rpcstatus.NotFound, "object not found")

			committed, err := db.CommitObject(ctx, metabase.CommitObject{ObjectStream: objStream})
			require.NoError(t, err)

			pendingObjStream := objStream
			pendingObjStream.Version++
			pending, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
				ObjectStream: pendingObjStream,
				Encryption:   metabasetest.DefaultEncryption,
			})
			require.NoError(t, err)

			_, err = endpoint.SetObjectRetention(ctx, req)
			require.NoError(t, err)

			// Ensure that the pending object is skipped despite being newer
			// than the committed one.
			requireRetention(t, objStream.Location(), pending.Version, metabase.Retention{})
			requireRetention(t, objStream.Location(), committed.Version, retention)
		})

		t.Run("Delete marker", func(t *testing.T) {
			objStream := randObjectStream(project.ID, lockBucketName)

			metabasetest.CreateObject(ctx, t, db, objStream, 0)
			deleteResult, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: objStream.Location(),
				Versioned:      true,
			})
			require.NoError(t, err)

			req := &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(objStream.BucketName),
				EncryptedObjectKey: []byte(objStream.ObjectKey),
				ObjectVersion:      deleteResult.Markers[0].StreamVersionID().Bytes(),
				Retention:          retentionToProto(randRetention(storj.ComplianceMode)),
			}

			// exact version
			_, err = endpoint.SetObjectRetention(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)

			// last committed version
			req.ObjectVersion = nil
			_, err = endpoint.SetObjectRetention(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockInvalidObjectState, objectInvalidStateErrMsg)
		})

		t.Run("unversioned bucket", func(t *testing.T) {
			bucketName := createBucket(t, false, false)
			obj := createObject(t, randObjectStream(project.ID, bucketName), metabase.Retention{})
			_, err := endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
			})
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
			requireRetention(t, obj.Location(), obj.Version, metabase.Retention{})
		})

		t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
			bucketName := createBucket(t, true, false)
			obj := createObject(t, randObjectStream(project.ID, bucketName), metabase.Retention{})
			_, err := endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
			})
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "Object Lock is not enabled for this bucket")
			requireRetention(t, obj.Location(), obj.Version, metabase.Retention{})
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			objStream, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.ComplianceMode)
			obj := createObject(t, objStream, metabase.Retention{})

			req := &pb.SetObjectRetentionRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(obj.BucketName),
				EncryptedObjectKey: []byte(obj.ObjectKey),
				ObjectVersion:      obj.StreamVersionID().Bytes(),
				Retention:          retentionToProto(retention),
			}

			_, err := endpoint.SetObjectRetention(ctx, req)
			rpctest.RequireStatus(t, err, rpcstatus.ObjectLockEndpointsDisabled, "Object Lock feature is not enabled")
			requireRetention(t, obj.Location(), obj.Version, obj.Retention)
		})

		t.Run("Unauthorized API key", func(t *testing.T) {
			t.Run("Old API key", func(t *testing.T) {
				_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
				require.NoError(t, err)

				objStream, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.ComplianceMode)
				obj := createObject(t, objStream, metabase.Retention{})

				_, err = endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
					Header:             &pb.RequestHeader{ApiKey: oldApiKey.SerializeRaw()},
					Bucket:             []byte(obj.BucketName),
					EncryptedObjectKey: []byte(obj.ObjectKey),
					ObjectVersion:      obj.StreamVersionID().Bytes(),
					Retention:          retentionToProto(retention),
				})
				rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
			})

			t.Run("Missing PutRetention permission", func(t *testing.T) {
				restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowPutRetention: true})
				require.NoError(t, err)

				objStream, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.ComplianceMode)
				obj := createObject(t, objStream, metabase.Retention{})

				_, err = endpoint.SetObjectRetention(ctx, &pb.SetObjectRetentionRequest{
					Header:             &pb.RequestHeader{ApiKey: restrictedApiKey.SerializeRaw()},
					Bucket:             []byte(obj.BucketName),
					EncryptedObjectKey: []byte(obj.ObjectKey),
					ObjectVersion:      obj.StreamVersionID().Bytes(),
					Retention:          retentionToProto(retention),
				})
				rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
			})

			t.Run("Missing BypassGovernanceRetention permission", func(t *testing.T) {
				restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowBypassGovernanceRetention: true})
				require.NoError(t, err)

				objStream, retention := randObjectStream(project.ID, lockBucketName), randRetention(storj.GovernanceMode)
				obj := createObject(t, objStream, metabase.Retention{})

				// Ensure that retention shortening is forbidden.
				newRetention := retention
				newRetention.RetainUntil = newRetention.RetainUntil.Add(-time.Minute)

				opts := &pb.SetObjectRetentionRequest{
					Header:                    &pb.RequestHeader{ApiKey: restrictedApiKey.SerializeRaw()},
					Bucket:                    []byte(obj.BucketName),
					EncryptedObjectKey:        []byte(obj.ObjectKey),
					ObjectVersion:             obj.StreamVersionID().Bytes(),
					Retention:                 retentionToProto(newRetention),
					BypassGovernanceRetention: true,
				}

				_, err = endpoint.SetObjectRetention(ctx, opts)
				rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
				requireRetention(t, obj.Location(), obj.Version, obj.Retention)

				// Ensure that removal of an active retention period is forbidden.
				opts.Retention = nil

				_, err = endpoint.SetObjectRetention(ctx, opts)
				rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
				requireRetention(t, obj.Location(), obj.Version, obj.Retention)

				// Ensure that changing the retention mode is forbidden.
				newRetention = retention
				newRetention.Mode = storj.ComplianceMode
				opts.Retention = retentionToProto(newRetention)

				_, err = endpoint.SetObjectRetention(ctx, opts)
				rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
				requireRetention(t, obj.Location(), obj.Version, obj.Retention)
			})
		})
	})
}

func TestEndpoint_GetObjectWithLockConfiguration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		endpoint := sat.Metainfo.Endpoint

		userCtx, err := sat.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)
		_, oldApiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)
		restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{
			DisallowGetRetention: true,
			DisallowPutRetention: true, // GetRetention is implicitly allowed if PutRetention is allowed
			DisallowGetLegalHold: true,
			DisallowPutLegalHold: true,
		})
		require.NoError(t, err)

		requireEqualRetention := func(t *testing.T, expected metabase.Retention, actual *pb.Retention) {
			require.NotNil(t, actual)
			require.EqualValues(t, expected.Mode, actual.Mode)
			require.WithinDuration(t, expected.RetainUntil, actual.RetainUntil, time.Microsecond)
		}

		createObject := func(t *testing.T, objStream metabase.ObjectStream, retention metabase.Retention, legalHold bool) metabase.Object {
			obj, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
					LegalHold:    legalHold,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Run(ctx, t, sat.Metabase.DB, objStream, 0)
			return obj
		}

		lockedObj := createObject(t, randObjectStream(project.ID, testrand.BucketName()), randRetention(storj.ComplianceMode), true)
		plainObj := createObject(t, randObjectStream(project.ID, testrand.BucketName()), metabase.Retention{}, false)

		t.Run("GetObject", func(t *testing.T) {
			getLockedObj := &pb.GetObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(lockedObj.BucketName),
				EncryptedObjectKey: []byte(lockedObj.ObjectKey),
				ObjectVersion:      lockedObj.StreamVersionID().Bytes(),
			}

			t.Run("Success", func(t *testing.T) {
				resp, err := endpoint.GetObject(ctx, getLockedObj)
				require.NoError(t, err)
				requireEqualRetention(t, lockedObj.Retention, resp.Object.Retention)
				require.NotNil(t, resp.Object.LegalHold)
				require.True(t, resp.Object.LegalHold.Value)

				resp, err = endpoint.GetObject(ctx, &pb.GetObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Bucket:             []byte(plainObj.BucketName),
					EncryptedObjectKey: []byte(plainObj.ObjectKey),
					ObjectVersion:      plainObj.StreamVersionID().Bytes(),
				})
				require.NoError(t, err)
				require.Nil(t, resp.Object.Retention)
				require.NotNil(t, resp.Object.LegalHold)
				require.False(t, resp.Object.LegalHold.Value)
			})

			t.Run("Unauthorized API key", func(t *testing.T) {
				getLockedObj.Header.ApiKey = oldApiKey.SerializeRaw()
				resp, err := endpoint.GetObject(ctx, getLockedObj)
				require.NoError(t, err)
				require.Nil(t, resp.Object.Retention)

				getLockedObj.Header.ApiKey = restrictedApiKey.SerializeRaw()
				resp, err = endpoint.GetObject(ctx, getLockedObj)
				require.NoError(t, err)
				require.Nil(t, resp.Object.Retention)
				require.Nil(t, resp.Object.LegalHold)
			})
		})

		t.Run("DownloadObject", func(t *testing.T) {
			ctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
				State: tls.ConnectionState{
					PeerCertificates: planet.Uplinks[0].Identity.Chain(),
				},
			})

			dlLockedObj := &pb.DownloadObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
				Bucket:             []byte(lockedObj.BucketName),
				EncryptedObjectKey: []byte(lockedObj.ObjectKey),
				ObjectVersion:      lockedObj.StreamVersionID().Bytes(),
			}

			t.Run("Success", func(t *testing.T) {
				resp, err := endpoint.DownloadObject(ctx, dlLockedObj)
				require.NoError(t, err)
				requireEqualRetention(t, lockedObj.Retention, resp.Object.Retention)
				require.NotNil(t, resp.Object.LegalHold)
				require.True(t, resp.Object.LegalHold.Value)

				resp, err = endpoint.DownloadObject(ctx, &pb.DownloadObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Bucket:             []byte(plainObj.BucketName),
					EncryptedObjectKey: []byte(plainObj.ObjectKey),
					ObjectVersion:      plainObj.StreamVersionID().Bytes(),
				})
				require.NoError(t, err)
				require.Nil(t, resp.Object.Retention)
				require.NotNil(t, resp.Object.LegalHold)
				require.False(t, resp.Object.LegalHold.Value)
			})

			t.Run("Unauthorized API key", func(t *testing.T) {
				dlLockedObj.Header.ApiKey = oldApiKey.SerializeRaw()
				resp, err := endpoint.DownloadObject(ctx, dlLockedObj)
				require.NoError(t, err)
				require.Nil(t, resp.Object.Retention)

				dlLockedObj.Header.ApiKey = restrictedApiKey.SerializeRaw()
				resp, err = endpoint.DownloadObject(ctx, dlLockedObj)
				require.NoError(t, err)
				require.Nil(t, resp.Object.Retention)
				require.Nil(t, resp.Object.LegalHold)
			})
		})
	})
}

var objectLockTestCases = []struct {
	name              string
	expectedRetention func() *metabase.Retention
	legalHold         bool
}{
	{name: "no retention, no legal hold"},
	{
		name: "retention - compliance, no legal hold",
		expectedRetention: func() *metabase.Retention {
			return &metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Now().Add(time.Hour).Truncate(time.Minute),
			}
		},
	},
	{
		name: "retention - governance, no legal hold",
		expectedRetention: func() *metabase.Retention {
			return &metabase.Retention{
				Mode:        storj.GovernanceMode,
				RetainUntil: time.Now().Add(time.Hour).Truncate(time.Minute),
			}
		},
	},
	{name: "no retention, legal hold", legalHold: true},
	{
		name: "retention - compliance, legal hold",
		expectedRetention: func() *metabase.Retention {
			return &metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Now().Add(time.Hour).Truncate(time.Minute),
			}
		},
		legalHold: true,
	},
	{
		name: "retention - governance, legal hold",
		expectedRetention: func() *metabase.Retention {
			return &metabase.Retention{
				Mode:        storj.GovernanceMode,
				RetainUntil: time.Now().Add(time.Hour).Truncate(time.Minute),
			}
		},
		legalHold: true,
	},
}

func TestEndpoint_CopyObjectWithRetention(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		createBucket := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, lockEnabled bool) string {
			name := testrand.BucketName()

			_, err := satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  projectID,
				Versioning: buckets.VersioningEnabled,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)

			return name
		}

		getObject := func(satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string) metabase.Object {
			objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objects {
				if o.Location() == (metabase.ObjectLocation{
					ProjectID:  projectID,
					BucketName: metabase.BucketName(bucketName),
					ObjectKey:  metabase.ObjectKey(key),
				}) {
					return o
				}
			}
			return metabase.Object{}
		}

		putObject := func(t *testing.T, satellite *testplanet.Satellite, objStream metabase.ObjectStream, expiresAt *time.Time, retention metabase.Retention) metabase.Object {
			object, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					ExpiresAt:    expiresAt,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Run(ctx, t, satellite.Metabase.DB, objStream, 0)
			return object
		}

		newCopy := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, apiKey []byte, srcBucket string, srcExpiresAt *time.Time, dstBucket, dstKey string) *pb.ObjectBeginCopyResponse {
			o := putObject(t, satellite, randObjectStream(projectID, srcBucket), srcExpiresAt, metabase.Retention{})

			response, err := satellite.API.Metainfo.Endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey,
				},
				Bucket:                []byte(srcBucket),
				EncryptedObjectKey:    []byte(o.ObjectKey),
				ObjectVersion:         o.StreamVersionID().Bytes(),
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
			})
			require.NoError(t, err)

			return response
		}

		satellite, project := planet.Satellites[0], planet.Uplinks[0].Projects[0]

		userCtx, err := satellite.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := satellite.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		ttl := time.Hour
		ttlApiKey, err := apiKey.Restrict(macaroon.Caveat{MaxObjectTtl: &ttl})
		require.NoError(t, err)

		_, oldApiKey, err := satellite.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowPutRetention: true})
		require.NoError(t, err)

		requireObject := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string) metabase.Object {
			obj := getObject(satellite, projectID, bucketName, key)
			require.NotZero(t, obj)
			return obj
		}

		requireNoObject := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string) {
			obj := getObject(satellite, projectID, bucketName, key)
			require.Zero(t, obj)
		}

		requireRetention := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string, r *metabase.Retention) {
			if r == nil {
				return
			}
			o := requireObject(t, satellite, projectID, bucketName, key)
			require.Nil(t, o.ExpiresAt)
			// We use cmp.Diff to ignore the timezone differences due to how Spanner maps timestamps in
			// regards to the pgx driver map.
			require.Zero(t, cmp.Diff(r, &o.Retention, cmpopts.EquateApproxTime(0)))
		}

		requireRetentionWithinDuration := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string, r *metabase.Retention, duration time.Duration) {
			if r == nil {
				return
			}
			o := requireObject(t, satellite, projectID, bucketName, key)
			require.Nil(t, o.ExpiresAt)
			require.Equal(t, r.Mode, o.Retention.Mode)
			require.WithinDuration(t, r.RetainUntil, o.Retention.RetainUntil, duration)
		}

		requireLegalHold := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string, lh bool) {
			o := requireObject(t, satellite, projectID, bucketName, key)
			require.Equal(t, lh, o.LegalHold)
		}

		requireEqualRetention := func(t *testing.T, expected *metabase.Retention, actual *pb.Retention) {
			var expectedRetention *pb.Retention
			if expected != nil {
				expectedRetention = retentionToProto(*expected)
			}
			require.Equal(t, expectedRetention, actual)
		}

		srcBucket := createBucket(t, satellite, project.ID, false)

		t.Run("success", func(t *testing.T) {
			dstBucket := createBucket(t, satellite, project.ID, true)

			for _, testCase := range objectLockTestCases {
				t.Run(testCase.name, func(t *testing.T) {
					dstKey := testrand.Path()
					beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

					var expectedRetention *metabase.Retention
					var expectedRetentionProto *pb.Retention
					if testCase.expectedRetention != nil {
						expectedRetention = testCase.expectedRetention()
						expectedRetentionProto = retentionToProto(*expectedRetention)
					}

					response, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
						Header: &pb.RequestHeader{
							ApiKey: apiKey.SerializeRaw(),
						},
						StreamId:              beginResponse.StreamId,
						NewBucket:             []byte(dstBucket),
						NewEncryptedObjectKey: []byte(dstKey),
						Retention:             expectedRetentionProto,
						LegalHold:             testCase.legalHold,
					})
					require.NoError(t, err)

					requireEqualRetention(t, expectedRetention, response.Object.Retention)
					requireRetention(t, satellite, project.ID, dstBucket, dstKey, expectedRetention)
					requireLegalHold(t, satellite, project.ID, dstBucket, dstKey, testCase.legalHold)
				})
			}
		})

		t.Run("unspecified retention mode or period", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

			_, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention: &pb.Retention{
					Mode: pb.Retention_COMPLIANCE,
				},
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)

			_, err = satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention: &pb.Retention{
					RetainUntil: time.Now().Add(time.Hour),
				},
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("Object Lock disabled", func(t *testing.T) {
			endpoint := satellite.Metainfo.Endpoint

			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)

			_, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockEndpointsDisabled)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("unversioned bucket", func(t *testing.T) {
			dstBucket, dstKey := testrand.BucketName(), testrand.Path()

			_, err := satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:      dstBucket,
				ProjectID: project.ID,
			})
			require.NoError(t, err)

			beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

			_, err = satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(randRetention(storj.ComplianceMode)),
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, false), testrand.Path()

			beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

			_, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(randRetention(storj.ComplianceMode)),
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("invalid retention mode", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

			expectedRetention := randRetention(storj.NoRetention)

			_, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("retention period is in the past", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)
			expectedRetention.RetainUntil = time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC)

			_, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("copying expiring objects is disallowed", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			ttl := time.Now().Add(time.Hour)

			beginResponse := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, &ttl, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)
			expectedRetention.RetainUntil = expectedRetention.RetainUntil.Add(time.Hour)

			_, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("TTLd API key has no effect", func(t *testing.T) {
			// NOTE(artur): however weird this test might be, this is
			// the current behavior that this test ensures we keep.
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newCopy(t, satellite, project.ID, ttlApiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)
			expectedRetention.RetainUntil = expectedRetention.RetainUntil.Add(time.Hour)

			response, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: ttlApiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			require.NoError(t, err)

			require.Zero(t, response.Object.ExpiresAt)
			requireEqualRetention(t, &expectedRetention, response.Object.Retention)
			requireRetention(t, satellite, project.ID, dstBucket, dstKey, &expectedRetention)
		})

		t.Run("unauthorized API keys", func(t *testing.T) {
			for _, k := range []*macaroon.APIKey{oldApiKey, restrictedApiKey} {
				dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

				beginResponse := newCopy(t, satellite, project.ID, k.SerializeRaw(), srcBucket, nil, dstBucket, dstKey)

				_, err := satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
					Header: &pb.RequestHeader{
						ApiKey: k.SerializeRaw(),
					},
					StreamId:              beginResponse.StreamId,
					NewBucket:             []byte(dstBucket),
					NewEncryptedObjectKey: []byte(dstKey),
					Retention:             retentionToProto(randRetention(storj.ComplianceMode)),
				})
				rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
				requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
			}
		})

		t.Run("use default retention", func(t *testing.T) {
			dstBucket, dstKey1, dstKey2 := testrand.BucketName(), testrand.Path(), testrand.Path()

			_, err := satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       dstBucket,
				ProjectID:  project.ID,
				Versioning: buckets.VersioningEnabled,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled:              true,
					DefaultRetentionMode: storj.GovernanceMode,
					DefaultRetentionDays: 1,
				},
			})
			require.NoError(t, err)

			beginResponse1 := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey1)
			beginResponse2 := newCopy(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, dstBucket, dstKey2)

			finishCopy := func(streamID storj.StreamID, dstBucket, dstKey string, retention *pb.Retention) {
				_, err = satellite.API.Metainfo.Endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
					Header: &pb.RequestHeader{
						ApiKey: apiKey.SerializeRaw(),
					},
					StreamId:              streamID,
					NewBucket:             []byte(dstBucket),
					NewEncryptedObjectKey: []byte(dstKey),
					Retention:             retention,
				})
				require.NoError(t, err)
			}

			finishCopy(beginResponse1.StreamId, dstBucket, dstKey1, nil)

			retention := randRetention(storj.ComplianceMode)

			finishCopy(beginResponse2.StreamId, dstBucket, dstKey2, retentionToProto(retention))

			requireRetentionWithinDuration(t, satellite, project.ID, dstBucket, dstKey1, &metabase.Retention{
				Mode:        storj.GovernanceMode,
				RetainUntil: time.Now().AddDate(0, 0, 1),
			}, time.Minute)

			requireRetention(t, satellite, project.ID, dstBucket, dstKey2, &retention)
		})
	})
}

func TestEndpoint_MoveObjectWithRetention(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ObjectLockEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		createBucket := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, lockEnabled bool) string {
			name := testrand.BucketName()

			_, err := satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       name,
				ProjectID:  projectID,
				Versioning: buckets.VersioningEnabled,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: lockEnabled,
				},
			})
			require.NoError(t, err)

			return name
		}

		getObject := func(satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string) metabase.Object {
			objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objects {
				if o.Location() == (metabase.ObjectLocation{
					ProjectID:  projectID,
					BucketName: metabase.BucketName(bucketName),
					ObjectKey:  metabase.ObjectKey(key),
				}) {
					return o
				}
			}
			return metabase.Object{}
		}

		putObject := func(t *testing.T, satellite *testplanet.Satellite, objStream metabase.ObjectStream, expiresAt *time.Time, retention metabase.Retention, legalHold bool) metabase.Object {
			object, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					ExpiresAt:    expiresAt,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
					LegalHold:    legalHold,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Run(ctx, t, satellite.Metabase.DB, objStream, 0)
			return object
		}

		newMove := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, apiKey []byte, srcBucket string, srcExpiresAt *time.Time, srcRetention metabase.Retention, srcLegalHold bool, dstBucket, dstKey string) *pb.ObjectBeginMoveResponse {
			o := putObject(t, satellite, randObjectStream(projectID, srcBucket), srcExpiresAt, srcRetention, srcLegalHold)

			response, err := satellite.API.Metainfo.Endpoint.BeginMoveObject(ctx, &pb.ObjectBeginMoveRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey,
				},
				Bucket:                []byte(srcBucket),
				EncryptedObjectKey:    []byte(o.ObjectKey),
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
			})
			require.NoError(t, err)

			return response
		}

		satellite, project := planet.Satellites[0], planet.Uplinks[0].Projects[0]

		userCtx, err := satellite.UserContext(ctx, project.Owner.ID)
		require.NoError(t, err)

		_, apiKey, err := satellite.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
		require.NoError(t, err)

		ttl := time.Hour
		ttlApiKey, err := apiKey.Restrict(macaroon.Caveat{MaxObjectTtl: &ttl})
		require.NoError(t, err)

		_, oldApiKey, err := satellite.API.Console.Service.CreateAPIKey(userCtx, project.ID, "old key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		restrictedApiKey, err := apiKey.Restrict(macaroon.Caveat{DisallowPutRetention: true})
		require.NoError(t, err)

		requireObject := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string) metabase.Object {
			obj := getObject(satellite, projectID, bucketName, key)
			require.NotZero(t, obj)
			return obj
		}

		requireNoObject := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string) {
			obj := getObject(satellite, projectID, bucketName, key)
			require.Zero(t, obj)
		}

		requireRetention := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string, r *metabase.Retention) {
			if r == nil {
				return
			}
			o := requireObject(t, satellite, projectID, bucketName, key)
			require.Nil(t, o.ExpiresAt)
			// We use cmp.Diff to ignore the timezone differences due to how Spanner maps timestamps in
			// regards to the pgx driver map.
			require.Zero(t, cmp.Diff(r, &o.Retention, cmpopts.EquateApproxTime(0)))
		}

		requireRetentionWithinDuration := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string, r *metabase.Retention, duration time.Duration) {
			if r == nil {
				return
			}
			o := requireObject(t, satellite, projectID, bucketName, key)
			require.Nil(t, o.ExpiresAt)
			require.Equal(t, r.Mode, o.Retention.Mode)
			require.WithinDuration(t, r.RetainUntil, o.Retention.RetainUntil, duration)
		}

		requireLegalHold := func(t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName, key string, lh bool) {
			o := requireObject(t, satellite, projectID, bucketName, key)
			require.Equal(t, lh, o.LegalHold)
		}

		srcBucket := createBucket(t, satellite, project.ID, false)

		t.Run("success", func(t *testing.T) {
			dstBucket := createBucket(t, satellite, project.ID, true)

			for _, testCase := range objectLockTestCases {
				t.Run(testCase.name, func(t *testing.T) {
					dstKey := testrand.Path()

					beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

					var expectedRetention *metabase.Retention
					var expectedRetentionProto *pb.Retention
					if testCase.expectedRetention != nil {
						expectedRetention = testCase.expectedRetention()
						expectedRetentionProto = retentionToProto(*expectedRetention)
					}

					_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
						Header: &pb.RequestHeader{
							ApiKey: apiKey.SerializeRaw(),
						},
						StreamId:              beginResponse.StreamId,
						NewBucket:             []byte(dstBucket),
						NewEncryptedObjectKey: []byte(dstKey),
						Retention:             expectedRetentionProto,
						LegalHold:             testCase.legalHold,
					})
					require.NoError(t, err)

					requireRetention(t, satellite, project.ID, dstBucket, dstKey, expectedRetention)
					requireLegalHold(t, satellite, project.ID, dstBucket, dstKey, testCase.legalHold)
				})
			}
		})

		t.Run("unspecified retention mode or period", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention: &pb.Retention{
					Mode: pb.Retention_COMPLIANCE,
				},
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)

			_, err = satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention: &pb.Retention{
					RetainUntil: time.Now().Add(time.Hour),
				},
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("Object Lock not globally supported", func(t *testing.T) {
			endpoint := satellite.Metainfo.Endpoint

			endpoint.TestSetObjectLockEnabled(false)
			defer endpoint.TestSetObjectLockEnabled(true)

			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockEndpointsDisabled)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("unversioned bucket", func(t *testing.T) {
			dstBucket, dstKey := testrand.BucketName(), testrand.Path()

			_, err := satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:      dstBucket,
				ProjectID: project.ID,
			})
			require.NoError(t, err)

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

			_, err = satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(randRetention(storj.ComplianceMode)),
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("Object Lock not enabled for bucket", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, false), testrand.Path()

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(randRetention(storj.ComplianceMode)),
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockBucketRetentionConfigurationMissing)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("invalid retention mode", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

			expectedRetention := randRetention(storj.NoRetention)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("retention period is in the past", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)
			expectedRetention.RetainUntil = time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("moving expiring objects is disallowed", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			ttl := time.Now().Add(time.Hour)

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, &ttl, metabase.Retention{}, false, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)
			expectedRetention.RetainUntil = expectedRetention.RetainUntil.Add(time.Hour)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			rpctest.RequireCode(t, err, rpcstatus.InvalidArgument)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("TTLd API key has no effect", func(t *testing.T) {
			// NOTE(artur): however weird this test might be, this is
			// the current behavior that this test ensures we keep.
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newMove(t, satellite, project.ID, ttlApiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

			expectedRetention := randRetention(storj.ComplianceMode)
			expectedRetention.RetainUntil = expectedRetention.RetainUntil.Add(time.Hour)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: ttlApiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(expectedRetention),
			})
			require.NoError(t, err)

			requireRetention(t, satellite, project.ID, dstBucket, dstKey, &expectedRetention)
		})

		t.Run("unauthorized API keys", func(t *testing.T) {
			for _, k := range []*macaroon.APIKey{oldApiKey, restrictedApiKey} {
				dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

				beginResponse := newMove(t, satellite, project.ID, k.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey)

				_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
					Header: &pb.RequestHeader{
						ApiKey: k.SerializeRaw(),
					},
					StreamId:              beginResponse.StreamId,
					NewBucket:             []byte(dstBucket),
					NewEncryptedObjectKey: []byte(dstKey),
					Retention:             retentionToProto(randRetention(storj.ComplianceMode)),
				})
				rpctest.RequireCode(t, err, rpcstatus.PermissionDenied)
				requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
			}
		})

		t.Run("moving an object from a locked location is impossible", func(t *testing.T) {
			dstBucket, dstKey := createBucket(t, satellite, project.ID, true), testrand.Path()

			beginResponse := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, randRetention(storj.ComplianceMode), false, dstBucket, dstKey)

			_, err := satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResponse.StreamId,
				NewBucket:             []byte(dstBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				Retention:             retentionToProto(randRetention(storj.ComplianceMode)),
			})
			rpctest.RequireCode(t, err, rpcstatus.ObjectLockObjectProtected)
			requireNoObject(t, satellite, project.ID, dstBucket, dstKey)
		})

		t.Run("default retention", func(t *testing.T) {
			dstBucket, dstKey1, dstKey2 := testrand.BucketName(), testrand.Path(), testrand.Path()

			_, err := satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				Name:       dstBucket,
				ProjectID:  project.ID,
				Versioning: buckets.VersioningEnabled,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled:              true,
					DefaultRetentionMode: storj.GovernanceMode,
					DefaultRetentionDays: 1,
				},
			})
			require.NoError(t, err)

			beginResponse1 := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey1)
			beginResponse2 := newMove(t, satellite, project.ID, apiKey.SerializeRaw(), srcBucket, nil, metabase.Retention{}, false, dstBucket, dstKey2)

			finishMove := func(streamID storj.StreamID, dstBucket, dstKey string, retention *pb.Retention) {
				_, err = satellite.API.Metainfo.Endpoint.FinishMoveObject(ctx, &pb.FinishMoveObjectRequest{
					Header: &pb.RequestHeader{
						ApiKey: apiKey.SerializeRaw(),
					},
					StreamId:              streamID,
					NewBucket:             []byte(dstBucket),
					NewEncryptedObjectKey: []byte(dstKey),
					Retention:             retention,
				})
				require.NoError(t, err)
			}

			finishMove(beginResponse1.StreamId, dstBucket, dstKey1, nil)

			retention := randRetention(storj.ComplianceMode)

			finishMove(beginResponse2.StreamId, dstBucket, dstKey2, retentionToProto(retention))

			requireRetentionWithinDuration(t, satellite, project.ID, dstBucket, dstKey1, &metabase.Retention{
				Mode:        storj.GovernanceMode,
				RetainUntil: time.Now().AddDate(0, 0, 1),
			}, time.Minute)

			requireRetention(t, satellite, project.ID, dstBucket, dstKey2, &retention)
		})
	})
}

func TestConditionalWrites(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		UplinkCount:    1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		endpoint := satellite.Metainfo.Endpoint

		unversionedBucket := testrand.BucketName()
		_, err := satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
			Name:      unversionedBucket,
			ProjectID: project.ID,
		})
		require.NoError(t, err)

		versionedBucket := testrand.BucketName()
		_, err = satellite.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
			Name:       versionedBucket,
			ProjectID:  project.ID,
			Versioning: buckets.VersioningEnabled,
		})
		require.NoError(t, err)

		beginObjReq := func(bucket, key string) *pb.ObjectBeginRequest {
			return &pb.ObjectBeginRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:             []byte(bucket),
				EncryptedObjectKey: []byte(key),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: pb.CipherSuite_ENC_NULL,
					BlockSize:   256,
				},
			}
		}

		createObject := func(bucket, key string, ifNoneMatch []string) (*pb.CommitObjectResponse, error) {
			beginObjReq := beginObjReq(bucket, key)

			beginResp, err := endpoint.BeginObject(ctx, beginObjReq)
			require.NoError(t, err)

			return endpoint.CommitObject(ctx, &pb.CommitObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:                      beginResp.StreamId,
				EncryptedMetadataNonce:        testrand.Nonce(),
				EncryptedMetadataEncryptedKey: randomEncryptedKey,
				IfNoneMatch:                   ifNoneMatch,
			})
		}

		t.Run("CommitObject not implemented", func(t *testing.T) {
			_, err := createObject(unversionedBucket, testrand.Path(), []string{"somethingelse"})
			rpctest.RequireCode(t, err, rpcstatus.Unimplemented)
		})

		t.Run("CommitInlineObject not implemented", func(t *testing.T) {
			beginObjReq := beginObjReq(unversionedBucket, testrand.Path())

			makeInlineSegment := &pb.MakeInlineSegmentRequest{
				Header:              beginObjReq.Header,
				Position:            &pb.SegmentPosition{},
				EncryptedKey:        beginObjReq.EncryptedObjectKey,
				EncryptedKeyNonce:   testrand.Nonce(),
				PlainSize:           512,
				EncryptedInlineData: testrand.Bytes(32),
			}
			commitObject := &pb.CommitObjectRequest{
				Header:      beginObjReq.Header,
				IfNoneMatch: []string{"somethingelse"},
			}

			_, _, _, err = endpoint.CommitInlineObject(ctx, beginObjReq, makeInlineSegment, commitObject)
			rpctest.RequireCode(t, err, rpcstatus.Unimplemented)
		})

		t.Run("CommitObject", func(t *testing.T) {
			key := testrand.Path()

			_, err := createObject(unversionedBucket, key, []string{"*"})
			require.NoError(t, err)

			_, err = createObject(unversionedBucket, key, []string{"*"})
			rpctest.RequireCode(t, err, rpcstatus.FailedPrecondition)
		})

		t.Run("CommitObject versioned", func(t *testing.T) {
			key := testrand.Path()

			_, err := createObject(versionedBucket, key, []string{"*"})
			require.NoError(t, err)

			_, err = createObject(versionedBucket, key, []string{"*"})
			rpctest.RequireCode(t, err, rpcstatus.FailedPrecondition)
		})

		t.Run("CommitObject delete marker", func(t *testing.T) {
			key := testrand.Path()

			_, err := createObject(versionedBucket, key, []string{"*"})
			require.NoError(t, err)

			_, err = endpoint.DeleteCommittedObject(ctx, metainfo.DeleteCommittedObject{
				ObjectLocation: metabase.ObjectLocation{
					ObjectKey:  metabase.ObjectKey(key),
					ProjectID:  project.ID,
					BucketName: metabase.BucketName(versionedBucket),
				},
				Version: []byte{},
			})
			require.NoError(t, err)

			_, err = createObject(versionedBucket, key, []string{"*"})
			require.NoError(t, err)
		})

		t.Run("CommitInlineObject", func(t *testing.T) {
			key := testrand.Path()

			_, err = createObject(unversionedBucket, key, []string{"*"})

			beginObjReq := beginObjReq(unversionedBucket, key)

			makeInlineSegment := &pb.MakeInlineSegmentRequest{
				Header:              beginObjReq.Header,
				Position:            &pb.SegmentPosition{},
				EncryptedKey:        beginObjReq.EncryptedObjectKey,
				EncryptedKeyNonce:   testrand.Nonce(),
				PlainSize:           512,
				EncryptedInlineData: testrand.Bytes(32),
			}
			commitObject := &pb.CommitObjectRequest{
				Header:      beginObjReq.Header,
				IfNoneMatch: []string{"*"},
			}

			_, _, _, err = endpoint.CommitInlineObject(ctx, beginObjReq, makeInlineSegment, commitObject)
			rpctest.RequireCode(t, err, rpcstatus.FailedPrecondition)
		})

		t.Run("CopyObject", func(t *testing.T) {
			srcKey, dstKey := testrand.Path(), testrand.Path()

			_, err := createObject(unversionedBucket, srcKey, nil)
			require.NoError(t, err)

			_, err = createObject(unversionedBucket, dstKey, nil)
			require.NoError(t, err)

			beginResp, err := endpoint.BeginCopyObject(ctx, &pb.ObjectBeginCopyRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Bucket:                []byte(unversionedBucket),
				EncryptedObjectKey:    []byte(srcKey),
				NewBucket:             []byte(unversionedBucket),
				NewEncryptedObjectKey: []byte(dstKey),
			})
			require.NoError(t, err)

			_, err = endpoint.FinishCopyObject(ctx, &pb.FinishCopyObjectRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				StreamId:              beginResp.StreamId,
				NewBucket:             []byte(unversionedBucket),
				NewEncryptedObjectKey: []byte(dstKey),
				NewSegmentKeys:        beginResp.SegmentKeys,
				IfNoneMatch:           []string{"*"},
			})
			rpctest.RequireCode(t, err, rpcstatus.FailedPrecondition)
		})
	})
}

func TestListObjects_ArbitraryPrefix(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Uplink: func(log *zap.Logger, index int, config *testplanet.UplinkConfig) {
				config.DefaultPathCipher = storj.EncNull
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].SerializeRaw()
		bucketName := "test-bucket"

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], bucketName))

		err := planet.Uplinks[0].Upload(ctx, satellite, bucketName, "photos/2023/image1.jpg", testrand.Bytes(100))
		require.NoError(t, err)
		err = planet.Uplinks[0].Upload(ctx, satellite, bucketName, "photos/2023/image2.jpg", testrand.Bytes(100))
		require.NoError(t, err)
		err = planet.Uplinks[0].Upload(ctx, satellite, bucketName, "photos/2024/image3.jpg", testrand.Bytes(100))
		require.NoError(t, err)
		err = planet.Uplinks[0].Upload(ctx, satellite, bucketName, "documents/file1.txt", testrand.Bytes(100))
		require.NoError(t, err)

		t.Run("ArbitraryPrefix=false", func(t *testing.T) {
			// Should add slash and return items without them
			resp, err := satellite.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:          &pb.RequestHeader{ApiKey: apiKey},
				Bucket:          []byte(bucketName),
				EncryptedPrefix: []byte("photos/2023"),
				ArbitraryPrefix: false,
				Recursive:       true,
			})
			require.NoError(t, err)
			require.Len(t, resp.Items, 2)
			require.Equal(t, "image1.jpg", string(resp.Items[0].EncryptedObjectKey))
			require.Equal(t, "image2.jpg", string(resp.Items[1].EncryptedObjectKey))
		})

		t.Run("ArbitraryPrefix=true", func(t *testing.T) {
			resp, err := satellite.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:          &pb.RequestHeader{ApiKey: apiKey},
				Bucket:          []byte(bucketName),
				EncryptedPrefix: []byte("photos/2025"),
				ArbitraryPrefix: true,
			})
			require.NoError(t, err)
			require.Len(t, resp.Items, 0)

			resp, err = satellite.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:          &pb.RequestHeader{ApiKey: apiKey},
				Bucket:          []byte(bucketName),
				EncryptedPrefix: []byte("photos/2023/image1.jpg"),
				ArbitraryPrefix: true,
			})
			require.NoError(t, err)
			require.Len(t, resp.Items, 1)
			require.Equal(t, "", string(resp.Items[0].EncryptedObjectKey))

			resp, err = satellite.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:          &pb.RequestHeader{ApiKey: apiKey},
				Bucket:          []byte(bucketName),
				EncryptedPrefix: []byte("photos/2023"),
				ArbitraryPrefix: true,
				Recursive:       true,
			})
			require.NoError(t, err)
			require.Len(t, resp.Items, 2)
			require.Equal(t, "/image1.jpg", string(resp.Items[0].EncryptedObjectKey))
			require.Equal(t, "/image2.jpg", string(resp.Items[1].EncryptedObjectKey))

			resp, err = satellite.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:          &pb.RequestHeader{ApiKey: apiKey},
				Bucket:          []byte(bucketName),
				EncryptedPrefix: []byte("photos/202"),
				ArbitraryPrefix: true,
			})
			require.NoError(t, err)
			require.Len(t, resp.Items, 2)
			require.Equal(t, "3/", string(resp.Items[0].EncryptedObjectKey))
			require.Equal(t, "4/", string(resp.Items[1].EncryptedObjectKey))
			require.Equal(t, pb.Object_PREFIX, resp.Items[0].Status)
			require.Equal(t, pb.Object_PREFIX, resp.Items[1].Status)

			resp, err = satellite.API.Metainfo.Endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:          &pb.RequestHeader{ApiKey: apiKey},
				Bucket:          []byte(bucketName),
				EncryptedPrefix: []byte("photos/202"),
				ArbitraryPrefix: true,
				Recursive:       true,
			})
			require.NoError(t, err)
			require.Len(t, resp.Items, 3)
			require.Equal(t, "3/image1.jpg", string(resp.Items[0].EncryptedObjectKey))
			require.Equal(t, "3/image2.jpg", string(resp.Items[1].EncryptedObjectKey))
			require.Equal(t, "4/image3.jpg", string(resp.Items[2].EncryptedObjectKey))
		})
	})
}

func TestEndpoint_ListObjectsMetadata(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxObjectKeyLength(1024),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.Metabase.DB
		endpoint := sat.API.Metainfo.Endpoint
		apiKey := planet.Uplinks[0].APIKey[sat.ID()]

		const bucket = "bucket"

		err := planet.Uplinks[0].TestingCreateBucket(ctx, sat, bucket)
		require.NoError(t, err)

		// setup following objects:
		// 1:  metadata   etag
		// 2:  metadata   !etag
		// 3:  !metadata   etag
		// 4:  !metadata   !etag

		// need to use valid marshaled metadata, because metainfo relies on it
		metadata, err := pb.Marshal(&pb.StreamMeta{
			EncryptedStreamInfo: testrand.Bytes(5 * memory.KiB),
			NumberOfSegments:    1,
		})
		require.NoError(t, err)
		data1 := metabasetest.RandEncryptedUserData()
		data1.EncryptedMetadata = metadata
		data2 := metabasetest.RandEncryptedUserDataWithoutETag()
		data2.EncryptedMetadata = metadata
		data3 := metabasetest.RandEncryptedUserData()
		data3.EncryptedMetadata = nil

		data4 := metabase.EncryptedUserData{}

		var objects []metabase.RawObject
		for i, data := range []metabase.EncryptedUserData{data1, data2, data3, data4} {
			objects = append(objects, metabase.RawObject{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  planet.Uplinks[0].Projects[0].ID,
					BucketName: bucket,
					ObjectKey:  metabase.ObjectKey(fmt.Sprint(i)),
					Version:    1,
					StreamID:   uuid.UUID{byte(i + 1)},
				},
				EncryptedUserData: data,
				Status:            metabase.CommittedUnversioned,
			})
		}
		require.NoError(t, db.TestingBatchInsertObjects(ctx, objects))

		type expect struct {
			stream, custom, etag bool
		}

		check := func(inc pb.ObjectListItemIncludes, expected []expect) {
			name := ""
			if inc.Metadata {
				name += "meta,"
			}
			if inc.IncludeEtag {
				name += "etag,"
			}
			if inc.IncludeEtagOrCustomMetadata {
				name += "either,"
			}

			if name == "" {
				name = "none"
			} else {
				name = name[:len(name)-1]
			}

			t.Run(name, func(t *testing.T) {
				resp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
					Header: &pb.RequestHeader{
						ApiKey: apiKey.SerializeRaw(),
					},
					Bucket:            []byte(bucket),
					ObjectIncludes:    &inc,
					UseObjectIncludes: true,
					Limit:             1000,
				})
				require.NoError(t, err)

				require.Equal(t, len(expected), len(resp.Items))
				for i, exp := range expected {
					name := string(resp.Items[i].EncryptedObjectKey)

					shouldHaveKey := exp.etag || exp.stream // when the output contains etag or metadata
					assert.Equal(t, shouldHaveKey, !resp.Items[i].EncryptedMetadataNonce.IsZero(), name)
					assert.Equal(t, shouldHaveKey, len(resp.Items[i].EncryptedMetadataEncryptedKey) > 0, name)

					assert.Equal(t, exp.etag, len(resp.Items[i].EncryptedEtag) > 0, name)
					assert.Equal(t, exp.stream, len(resp.Items[i].EncryptedMetadata) > 0, name)
					if exp.stream {
						var streamMeta pb.StreamMeta
						assert.NoError(t, pb.Unmarshal(resp.Items[i].EncryptedMetadata, &streamMeta), name)
						assert.Equal(t, exp.custom, len(streamMeta.EncryptedStreamInfo) > 0, name)
					}
				}
			})
		}

		check(pb.ObjectListItemIncludes{
			Metadata:    true,
			IncludeEtag: true,
		}, []expect{
			{stream: true, custom: true, etag: true},
			{stream: true, custom: true, etag: false},
			{stream: true, custom: false, etag: true},
			{stream: false, custom: false, etag: false},
		})

		check(pb.ObjectListItemIncludes{
			Metadata:    true,
			IncludeEtag: false,
		}, []expect{
			{stream: true, custom: true, etag: false},
			{stream: true, custom: true, etag: false},
			{stream: true, custom: false, etag: false},
			{stream: false, custom: false, etag: false},
		})

		check(pb.ObjectListItemIncludes{
			Metadata:    false,
			IncludeEtag: true,
		}, []expect{
			{stream: true, custom: false, etag: true},
			{stream: false, custom: false, etag: false},
			{stream: true, custom: false, etag: true},
			{stream: false, custom: false, etag: false},
		})

		check(pb.ObjectListItemIncludes{
			Metadata:                    false,
			IncludeEtag:                 true,
			IncludeEtagOrCustomMetadata: true,
		}, []expect{
			{stream: true, custom: false, etag: true},
			{stream: true, custom: true, etag: false},
			{stream: true, custom: false, etag: true},
			{stream: false, custom: false, etag: false},
		})

		check(pb.ObjectListItemIncludes{
			Metadata:                    true,
			IncludeEtag:                 false,
			IncludeEtagOrCustomMetadata: true,
		}, []expect{
			{stream: true, custom: true, etag: true},
			{stream: true, custom: true, etag: false},
			{stream: true, custom: false, etag: true},
			{stream: false, custom: false, etag: false},
		})

		check(pb.ObjectListItemIncludes{
			Metadata:                    false,
			IncludeEtag:                 false,
			IncludeEtagOrCustomMetadata: true,
		}, []expect{
			{stream: true, custom: false, etag: true},
			{stream: true, custom: true, etag: false},
			{stream: true, custom: false, etag: true},
			{stream: false, custom: false, etag: false},
		})
	})
}

func TestDownloadObject_DownloadSegment_ServerSideCopy(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Uplink: func(log *zap.Logger, index int, config *testplanet.UplinkConfig) {
				config.DefaultPathCipher = storj.EncNull
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].SerializeRaw()
		endpoint := planet.Satellites[0].API.Metainfo.Endpoint

		now := time.Now()

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test"))
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test", "remote", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test", "inline", testrand.Bytes(500))
		require.NoError(t, err)

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		t.Run("untrusted uplink", func(t *testing.T) {
			for _, objectKey := range []string{"remote", "inline"} {
				_, err = endpoint.DownloadObject(peerctx, &pb.DownloadObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey},
					Bucket:             []byte("test"),
					EncryptedObjectKey: []byte(objectKey),

					ServerSideCopy: true,
				})
				rpctest.AssertCode(t, err, rpcstatus.InvalidArgument)

				resp, err := endpoint.GetObject(ctx, &pb.GetObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey},
					Bucket:             []byte("test"),
					EncryptedObjectKey: []byte(objectKey),
				})
				require.NoError(t, err)

				_, err = endpoint.DownloadSegment(peerctx, &pb.DownloadSegmentRequest{
					Header:         &pb.RequestHeader{ApiKey: apiKey},
					StreamId:       resp.Object.StreamId,
					CursorPosition: &pb.SegmentPosition{},
					ServerSideCopy: true,
				})
				rpctest.AssertCode(t, err, rpcstatus.InvalidArgument)
			}
		})
		t.Run("trusted uplink", func(t *testing.T) {
			endpoint.TestingAddTrustedUplink(planet.Uplinks[0].ID())

			for _, objectKey := range []string{"remote", "inline"} {
				resp, err := endpoint.DownloadObject(peerctx, &pb.DownloadObjectRequest{
					Header:             &pb.RequestHeader{ApiKey: apiKey},
					Bucket:             []byte("test"),
					EncryptedObjectKey: []byte(objectKey),

					ServerSideCopy: true,
				})
				require.NoError(t, err)

				dsResp, err := endpoint.DownloadSegment(peerctx, &pb.DownloadSegmentRequest{
					Header:         &pb.RequestHeader{ApiKey: apiKey},
					StreamId:       resp.Object.StreamId,
					CursorPosition: &pb.SegmentPosition{},
					ServerSideCopy: true,
				})
				require.NoError(t, err)

				// test egress skip while using DownloadObject and DownloadSegment endpoints
				for _, toDownload := range []*pb.DownloadSegmentResponse{resp.SegmentDownload[0], dsResp} {
					var data []byte
					if toDownload.EncryptedInlineData != nil {
						data = toDownload.EncryptedInlineData
						// encrypted data takes more space than the original data
						require.GreaterOrEqual(t, len(data), 500)
					} else {
						limit := toDownload.AddressedLimits[0]
						nodeURL := storj.NodeURL{
							ID:      limit.Limit.StorageNodeId,
							Address: limit.StorageNodeAddress.Address,
						}
						require.NoError(t, err)

						func() {
							client, err := piecestore.Dial(ctx, planet.Uplinks[0].Dialer, nodeURL, piecestore.DefaultConfig)
							require.NoError(t, err)
							defer ctx.Check(client.Close)

							download, err := client.Download(ctx, limit.Limit, toDownload.PrivateKey, 0, 400)
							require.NoError(t, err)
							defer ctx.Check(download.Close)

							data, err = io.ReadAll(download)
							require.NoError(t, err)
							require.Len(t, data, 400)
						}()
					}
				}
			}

			for _, sn := range planet.StorageNodes {
				sn.Storage2.Orders.SendOrders(ctx, now.Add(24*time.Hour))
			}
			planet.Satellites[0].Orders.Chore.Loop.TriggerWait()

			usage, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectTotal(ctx, planet.Uplinks[0].Projects[0].ID, now.Add(-time.Hour), now.Add(time.Hour))
			require.NoError(t, err)
			require.Zero(t, usage.Egress)
		})
	})
}

func TestListObjects_Delimiter(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Uplink: func(log *zap.Logger, index int, config *testplanet.UplinkConfig) {
				config.DefaultPathCipher = storj.EncNull
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			delimiter        = "###"
			defaultDelimiter = metabase.Delimiter
		)

		sat := planet.Satellites[0]
		endpoint := sat.Metainfo.Endpoint
		apiKey := planet.Uplinks[0].APIKey[sat.ID()].SerializeRaw()
		bucketName := testrand.BucketName()

		_, err := endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
			Header: &pb.RequestHeader{ApiKey: apiKey},
			Name:   []byte(bucketName),
		})
		require.NoError(t, err)

		objects := make(map[string]*pb.ObjectListItem)

		for _, objectKey := range []string{
			"abc" + delimiter,
			"abc" + delimiter + "def",
			"abc" + delimiter + "def" + delimiter + "ghi",
			"abc" + defaultDelimiter + "def",
			"xyz" + delimiter + "uvw",
		} {
			object := metabasetest.CreateObject(ctx, t, sat.Metabase.DB, metabase.ObjectStream{
				ProjectID:  planet.Uplinks[0].Projects[0].ID,
				BucketName: metabase.BucketName(bucketName),
				ObjectKey:  metabase.ObjectKey(objectKey),
				Version:    1,
				StreamID:   testrand.UUID(),
			}, 0)

			objects[objectKey] = &pb.ObjectListItem{
				EncryptedObjectKey: []byte(object.ObjectKey),
				Status:             pb.Object_COMMITTED_UNVERSIONED,
				ObjectVersion:      object.StreamVersionID().Bytes(),
				IsLatest:           true,
			}
		}

		prefixEntry := func(objectKey string) *pb.ObjectListItem {
			return &pb.ObjectListItem{
				EncryptedObjectKey: []byte(objectKey),
				Status:             pb.Object_PREFIX,
				ObjectVersion:      metabase.StreamVersionID{}.Bytes(),
			}
		}

		withoutPrefix := func(prefix string, item *pb.ObjectListItem) *pb.ObjectListItem {
			newItem := *item
			newItem.EncryptedObjectKey = item.EncryptedObjectKey[len(prefix):]
			return &newItem
		}

		t.Run("Default delimiter", func(t *testing.T) {
			resp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:            &pb.RequestHeader{ApiKey: apiKey},
				Bucket:            []byte(bucketName),
				UseObjectIncludes: true,
				ObjectIncludes:    &pb.ObjectListItemIncludes{ExcludeSystemMetadata: true},
			})
			require.NoError(t, err)

			require.Equal(t, []*pb.ObjectListItem{
				objects["abc"+delimiter],
				objects["abc"+delimiter+"def"],
				objects["abc"+delimiter+"def"+delimiter+"ghi"],
				prefixEntry("abc" + defaultDelimiter),
				objects["xyz"+delimiter+"uvw"],
			}, resp.Items)
		})

		t.Run("Root", func(t *testing.T) {
			resp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:            &pb.RequestHeader{ApiKey: apiKey},
				Bucket:            []byte(bucketName),
				Delimiter:         []byte(delimiter),
				UseObjectIncludes: true,
				ObjectIncludes:    &pb.ObjectListItemIncludes{ExcludeSystemMetadata: true},
			})
			require.NoError(t, err)

			require.Equal(t, []*pb.ObjectListItem{
				prefixEntry("abc" + delimiter),
				objects["abc"+defaultDelimiter+"def"],
				prefixEntry("xyz" + delimiter),
			}, resp.Items)
		})

		t.Run("1 level deep", func(t *testing.T) {
			resp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:            &pb.RequestHeader{ApiKey: apiKey},
				Bucket:            []byte(bucketName),
				Delimiter:         []byte(delimiter),
				EncryptedPrefix:   []byte("abc" + delimiter),
				UseObjectIncludes: true,
				ObjectIncludes:    &pb.ObjectListItemIncludes{ExcludeSystemMetadata: true},
			})
			require.NoError(t, err)

			require.Equal(t, []*pb.ObjectListItem{
				withoutPrefix("abc"+delimiter, objects["abc"+delimiter]),
				withoutPrefix("abc"+delimiter, objects["abc"+delimiter+"def"]),
				prefixEntry("def" + delimiter),
			}, resp.Items)
		})

		t.Run("2 levels deep", func(t *testing.T) {
			resp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:            &pb.RequestHeader{ApiKey: apiKey},
				Bucket:            []byte(bucketName),
				Delimiter:         []byte(delimiter),
				EncryptedPrefix:   []byte("abc" + delimiter + "def" + delimiter),
				UseObjectIncludes: true,
				ObjectIncludes:    &pb.ObjectListItemIncludes{ExcludeSystemMetadata: true},
			})
			require.NoError(t, err)

			require.Equal(t, []*pb.ObjectListItem{
				withoutPrefix(
					"abc"+delimiter+"def"+delimiter,
					objects["abc"+delimiter+"def"+delimiter+"ghi"],
				),
			}, resp.Items)
		})

		t.Run("Prefix suffixed with partial delimiter", func(t *testing.T) {
			partialDelimiter := delimiter[:len(delimiter)-1]
			remainingDelimiter := delimiter[len(delimiter)-1:]

			resp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:            &pb.RequestHeader{ApiKey: apiKey},
				Bucket:            []byte(bucketName),
				Delimiter:         []byte(delimiter),
				EncryptedPrefix:   []byte("abc" + partialDelimiter),
				ArbitraryPrefix:   true,
				UseObjectIncludes: true,
				ObjectIncludes:    &pb.ObjectListItemIncludes{ExcludeSystemMetadata: true},
			})
			require.NoError(t, err)

			require.Equal(t, []*pb.ObjectListItem{
				withoutPrefix("abc"+partialDelimiter, objects["abc"+delimiter]),
				withoutPrefix("abc"+partialDelimiter, objects["abc"+delimiter+"def"]),
				prefixEntry(remainingDelimiter + "def" + delimiter),
			}, resp.Items)
		})

		t.Run("Delimiter with recursive", func(t *testing.T) {
			// Ensure that the delimiter has no effect if recursive listing was requested.
			resp, err := endpoint.ListObjects(ctx, &pb.ListObjectsRequest{
				Header:            &pb.RequestHeader{ApiKey: apiKey},
				Bucket:            []byte(bucketName),
				Delimiter:         []byte(delimiter),
				Recursive:         true,
				UseObjectIncludes: true,
				ObjectIncludes:    &pb.ObjectListItemIncludes{ExcludeSystemMetadata: true},
			})
			require.NoError(t, err)

			require.Equal(t, []*pb.ObjectListItem{
				objects["abc"+delimiter],
				objects["abc"+delimiter+"def"],
				objects["abc"+delimiter+"def"+delimiter+"ghi"],
				objects["abc"+defaultDelimiter+"def"],
				objects["xyz"+delimiter+"uvw"],
			}, resp.Items)
		})
	})
}

func TestDownloadObject_DownloadSegment_DesiredNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
			Uplink: func(log *zap.Logger, index int, config *testplanet.UplinkConfig) {
				config.DefaultPathCipher = storj.EncNull
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].SerializeRaw()
		endpoint := planet.Satellites[0].API.Metainfo.Endpoint

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test"))
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test", "remote", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		peerctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: planet.Uplinks[0].Identity.Chain(),
			}})

		defaultNodesNumber := planet.Satellites[0].API.Orders.Service.DownloadNodes(storj.RedundancyScheme{
			Algorithm:      storj.ReedSolomon,
			RequiredShares: 2,
			TotalShares:    6,
		})

		for desiredNodes, result := range map[int32]int{
			// TODO right now DesiredNodes feature can return default number of nodes or more
			0:             int(defaultNodesNumber),
			3:             3,
			4:             4,
			10:            4,
			math.MaxInt32: 4,
		} {
			resp, err := endpoint.DownloadObject(peerctx, &pb.DownloadObjectRequest{
				Header:             &pb.RequestHeader{ApiKey: apiKey},
				Bucket:             []byte("test"),
				EncryptedObjectKey: []byte("remote"),

				DesiredNodes: desiredNodes,
			})
			require.NoError(t, err)

			orderLimits := 0
			for _, limit := range resp.SegmentDownload[0].AddressedLimits {
				if limit.Limit != nil {
					orderLimits++
				}
			}
			require.Equal(t, result, orderLimits)

			dsResp, err := endpoint.DownloadSegment(peerctx, &pb.DownloadSegmentRequest{
				Header:         &pb.RequestHeader{ApiKey: apiKey},
				StreamId:       resp.Object.StreamId,
				CursorPosition: &pb.SegmentPosition{},
				DesiredNodes:   desiredNodes,
			})
			require.NoError(t, err)

			orderLimits = 0
			for _, limit := range dsResp.AddressedLimits {
				if limit.Limit != nil {
					orderLimits++
				}
			}
			require.Equal(t, result, orderLimits)
		}
	})
}

func randRetention(mode storj.RetentionMode) metabase.Retention {
	randDur := time.Duration(rand.Int63n(1000 * int64(time.Hour)))
	return metabase.Retention{
		Mode:        mode,
		RetainUntil: time.Now().Add(time.Hour + randDur).Truncate(time.Minute),
	}
}

func retentionToProto(retention metabase.Retention) *pb.Retention {
	return &pb.Retention{
		Mode:        pb.Retention_Mode(retention.Mode),
		RetainUntil: retention.RetainUntil,
	}
}

func randVersion() metabase.Version {
	// math.MaxInt32-10 gives room for 10 subsequent versions to be
	// created before reaching the limit of pb.Object.Version.
	return metabase.Version(1 + rand.Int31n(math.MaxInt32-10))
}

func randObjectStream(projectID uuid.UUID, bucketName string) metabase.ObjectStream {
	return metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: metabase.BucketName(bucketName),
		ObjectKey:  metabasetest.RandObjectKey(),
		Version:    randVersion(),
		StreamID:   testrand.UUID(),
	}
}

func TestNegativeVersion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		UplinkCount:    1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.TestingAlternativeBeginObject = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		checkObjects := func(bucketName string, f func(object metabase.Object)) {
			objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, objects)
			for _, object := range objects {
				if string(object.BucketName) == bucketName {
					f(object)
				}
			}
		}

		t.Run("Move and Copy do not create negative versions", func(t *testing.T) {
			bucketName := "testbucket"
			require.NoError(t, planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName))
			require.NoError(t, planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "an/object/keyA", testrand.Bytes(memory.KiB)))
			require.NoError(t, planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "an/object/keyB", testrand.Bytes(memory.KiB)))

			checkObjects(bucketName, func(object metabase.Object) {
				require.Greater(t, object.Version, metabase.Version(0))
			})

			_, err = project.BeginUpload(ctx, bucketName, "an/object/move_key", nil)
			require.NoError(t, err)
			_, err = project.BeginUpload(ctx, bucketName, "an/object/copy_key", nil)
			require.NoError(t, err)

			require.NoError(t, project.MoveObject(ctx, bucketName, "an/object/keyA", bucketName, "an/object/move_key", nil))
			_, err = project.CopyObject(ctx, bucketName, "an/object/keyB", bucketName, "an/object/copy_key", nil)
			require.NoError(t, err)

			checkObjects(bucketName, func(object metabase.Object) {
				if object.Status > metabase.Pending {
					require.Greater(t, object.Version, metabase.Version(0))
				} else {
					require.Less(t, object.Version, metabase.Version(0))
				}
			})
		})

		t.Run("Delete marker do not create negative versions", func(t *testing.T) {
			bucketName := "testbucket-versioned"
			require.NoError(t, planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName))
			require.NoError(t, bucket.SetBucketVersioning(ctx, project, bucketName, true))
			require.NoError(t, planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "an/object/keyA", testrand.Bytes(memory.KiB)))

			checkObjects(bucketName, func(object metabase.Object) {
				require.Greater(t, object.Version, metabase.Version(0))
			})

			_, err = project.BeginUpload(ctx, bucketName, "an/object/keyA", nil)
			require.NoError(t, err)

			_, err = project.DeleteObject(ctx, bucketName, "an/object/keyA")
			require.NoError(t, err)

			checkObjects(bucketName, func(object metabase.Object) {
				if object.Status > metabase.Pending {
					require.Greater(t, object.Version, metabase.Version(0))
				} else {
					require.Less(t, object.Version, metabase.Version(0))
				}
			})
		})
	})
}

func TestUploadWithNoPendingObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.TestingNoPendingObjectUpload = true
				config.Metainfo.MaxSegmentSize = 13 * memory.KiB
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		objects := map[string]memory.Size{
			"1KiB":   memory.KiB,
			"10KiB":  10 * memory.KiB,
			"100KiB": 100 * memory.KiB,
		}

		for objectKey, size := range objects {
			t.Run(objectKey, func(t *testing.T) {
				for range 3 {
					expectedData := testrand.Bytes(size)
					require.NoError(t, planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket", objectKey, expectedData))

					data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test-bucket", objectKey)
					require.NoError(t, err)
					require.Equal(t, expectedData, data)
				}
			})
		}
	})
}
