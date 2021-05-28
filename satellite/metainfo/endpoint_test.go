// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
	"storj.io/uplink"
	"storj.io/uplink/private/metaclient"
)

func TestEndpoint_DeleteCommittedObject(t *testing.T) {
	bucketName := "a-bucket"
	createObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "object-filename", data)
		require.NoError(t, err)
	}
	deleteObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metainfo.Metabase.TestingAllCommittedObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 1)

		_, err = planet.Satellites[0].Metainfo.Endpoint2.DeleteCommittedObject(ctx, projectID, bucketName, items[0].ObjectKey)
		require.NoError(t, err)

		items, err = planet.Satellites[0].Metainfo.Metabase.TestingAllCommittedObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}
	testDeleteObject(t, createObject, deleteObject)
}

func TestEndpoint_DeletePendingObject(t *testing.T) {
	bucketName := "a-bucket"
	createObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		// TODO This should be replaced by a call to testplanet.Uplink.MultipartUpload when available.
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err, "failed to retrieve project")

		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err, "failed to create bucket")

		info, err := project.BeginUpload(ctx, bucketName, "object-filename", &uplink.UploadOptions{})
		require.NoError(t, err, "failed to start multipart upload")

		upload, err := project.UploadPart(ctx, bucketName, bucketName, info.UploadID, 1)
		require.NoError(t, err, "failed to put object part")
		_, err = upload.Write(data)
		require.NoError(t, err, "failed to put object part")
		require.NoError(t, upload.Commit(), "failed to put object part")
	}
	deleteObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metainfo.Metabase.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 1)

		deletedObjects, err := planet.Satellites[0].Metainfo.Endpoint2.DeletePendingObject(ctx,
			metabase.ObjectStream{
				ProjectID:  projectID,
				BucketName: bucketName,
				ObjectKey:  items[0].ObjectKey,
				Version:    1,
				StreamID:   items[0].StreamID,
			})
		require.NoError(t, err)
		require.Len(t, deletedObjects, 1)

		items, err = planet.Satellites[0].Metainfo.Metabase.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}
	testDeleteObject(t, createObject, deleteObject)
}

func TestEndpoint_DeleteObjectAnyStatus(t *testing.T) {
	bucketName := "a-bucket"
	createCommittedObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], bucketName, "object-filename", data)
		require.NoError(t, err)
	}
	deleteCommittedObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metainfo.Metabase.TestingAllCommittedObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 1)

		deletedObjects, err := planet.Satellites[0].Metainfo.Endpoint2.DeleteObjectAnyStatus(ctx, metabase.ObjectLocation{
			ProjectID:  projectID,
			BucketName: bucketName,
			ObjectKey:  items[0].ObjectKey,
		})
		require.NoError(t, err)
		require.Len(t, deletedObjects, 1)

		items, err = planet.Satellites[0].Metainfo.Metabase.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}
	testDeleteObject(t, createCommittedObject, deleteCommittedObject)

	createPendingObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet, data []byte) {
		// TODO This should be replaced by a call to testplanet.Uplink.MultipartUpload when available.
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err, "failed to retrieve project")

		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err, "failed to create bucket")

		info, err := project.BeginUpload(ctx, bucketName, "object-filename", &uplink.UploadOptions{})
		require.NoError(t, err, "failed to start multipart upload")

		upload, err := project.UploadPart(ctx, bucketName, bucketName, info.UploadID, 1)
		require.NoError(t, err, "failed to put object part")
		_, err = upload.Write(data)
		require.NoError(t, err, "failed to start multipart upload")
		require.NoError(t, upload.Commit(), "failed to start multipart upload")
	}

	deletePendingObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metainfo.Metabase.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 1)

		deletedObjects, err := planet.Satellites[0].Metainfo.Endpoint2.DeleteObjectAnyStatus(ctx, metabase.ObjectLocation{
			ProjectID:  projectID,
			BucketName: bucketName,
			ObjectKey:  items[0].ObjectKey,
		})
		require.NoError(t, err)
		require.Len(t, deletedObjects, 1)

		items, err = planet.Satellites[0].Metainfo.Metabase.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 0)
	}

	testDeleteObject(t, createPendingObject, deletePendingObject)
}

func testDeleteObject(t *testing.T, createObject func(ctx context.Context, t *testing.T, planet *testplanet.Planet,
	data []byte), deleteObject func(ctx context.Context, t *testing.T, planet *testplanet.Planet)) {
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

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.caseDescription, func(t *testing.T) {
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

					createObject(ctx, t, planet, tc.objData)

					// calculate the SNs total used space after data upload
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					deleteObject(ctx, t, planet)

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

			})
		}
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

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.caseDescription, func(t *testing.T) {
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

					createObject(ctx, t, planet, tc.objData)

					// Shutdown the first numToShutdown storage nodes before we delete the pieces
					require.NoError(t, planet.StopPeer(planet.StorageNodes[0]))
					require.NoError(t, planet.StopPeer(planet.StorageNodes[1]))

					deleteObject(ctx, t, planet)

					planet.WaitForStorageNodeDeleters(ctx)

					// Check that storage nodes that were offline when deleting the pieces
					// they are still holding data
					var totalUsedSpace int64
					for i := 0; i < numToShutdown; i++ {
						piecesTotal, _, err := planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					require.NotZero(t, totalUsedSpace, "totalUsedSpace offline nodes")

					// Check that storage nodes which are online when deleting pieces don't
					// hold any piece
					totalUsedSpace = 0
					for i := numToShutdown; i < len(planet.StorageNodes); i++ {
						piecesTotal, _, err := planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					require.Zero(t, totalUsedSpace, "totalUsedSpace online nodes")
				})
			})
		}
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

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.caseDescription, func(t *testing.T) {
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
					createObject(ctx, t, planet, tc.objData)

					// Shutdown all the storage nodes before we delete the pieces
					for _, sn := range planet.StorageNodes {
						require.NoError(t, planet.StopPeer(sn))
					}

					deleteObject(ctx, t, planet)

					// Check that storage nodes that were offline when deleting the pieces
					// they are still holding data
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					require.NotZero(t, totalUsedSpace, "totalUsedSpace")
				})
			})
		}
	})
}

func TestDeleteBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
				testplanet.MaxSegmentSize(13*memory.KiB),
			),
		},
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		satelliteSys := planet.Satellites[0]
		uplnk := planet.Uplinks[0]

		expectedBucketName := "remote-segments-bucket"

		err := uplnk.Upload(ctx, planet.Satellites[0], expectedBucketName, "single-segment-object", testrand.Bytes(10*memory.KiB))
		require.NoError(t, err)
		err = uplnk.Upload(ctx, planet.Satellites[0], expectedBucketName, "multi-segment-object", testrand.Bytes(50*memory.KiB))
		require.NoError(t, err)
		err = uplnk.Upload(ctx, planet.Satellites[0], expectedBucketName, "remote-segment-inline-object", testrand.Bytes(33*memory.KiB))
		require.NoError(t, err)

		objects, err := satelliteSys.API.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 3)

		delResp, err := satelliteSys.API.Metainfo.Endpoint2.DeleteBucket(ctx, &pb.BucketDeleteRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Name:      []byte(expectedBucketName),
			DeleteAll: true,
		})
		require.NoError(t, err)
		require.Equal(t, int64(3), delResp.DeletedObjectsCount)

		// confirm the bucket is deleted
		buckets, err := satelliteSys.Metainfo.Endpoint2.ListBuckets(ctx, &pb.BucketListRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Direction: int32(storj.Forward),
		})
		require.NoError(t, err)
		require.Len(t, buckets.GetItems(), 0)
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
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]
		client, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], apiKey)
		require.NoError(t, err)
		defer ctx.Check(client.Close)

		err = planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "testbucket")
		require.NoError(t, err)

		beginObjectResponse, err := client.BeginObject(ctx, metaclient.BeginObjectParams{
			Bucket:        []byte("testbucket"),
			EncryptedPath: []byte("a/b/testobject"),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   256,
			},
		})
		require.NoError(t, err)

		response, err := client.BeginSegment(ctx, metaclient.BeginSegmentParams{
			StreamID: beginObjectResponse.StreamID,
			Position: storj.SegmentPosition{
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
