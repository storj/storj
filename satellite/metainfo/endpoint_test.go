// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/uplink/private/etag"
	"storj.io/uplink/private/multipart"
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
		project, err := planet.Uplinks[0].GetProject(ctx, planet.Satellites[0])
		require.NoError(t, err, "failed to retrieve project")

		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err, "failed to create bucket")

		info, err := multipart.NewMultipartUpload(ctx, project, bucketName, "object-filename", &multipart.UploadOptions{})
		require.NoError(t, err, "failed to start multipart upload")

		_, err = multipart.PutObjectPart(ctx, project, bucketName, bucketName, info.StreamID, 1,
			etag.NewHashReader(bytes.NewReader(data), sha256.New()))
		require.NoError(t, err, "failed to put object part")
	}
	deleteObject := func(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
		projectID := planet.Uplinks[0].Projects[0].ID
		items, err := planet.Satellites[0].Metainfo.Metabase.TestingAllPendingObjects(ctx, projectID, bucketName)
		require.NoError(t, err)
		require.Len(t, items, 1)

		deletedObjects, err := planet.Satellites[0].Metainfo.Endpoint2.DeletePendingObject(ctx, projectID, bucketName, items[0].ObjectKey, 1, items[0].StreamID)
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
		project, err := planet.Uplinks[0].GetProject(ctx, planet.Satellites[0])
		require.NoError(t, err, "failed to retrieve project")

		_, err = project.CreateBucket(ctx, bucketName)
		require.NoError(t, err, "failed to create bucket")

		info, err := multipart.NewMultipartUpload(ctx, project, bucketName, "object-filename", &multipart.UploadOptions{})
		require.NoError(t, err, "failed to start multipart upload")

		_, err = multipart.PutObjectPart(ctx, project, bucketName, bucketName, info.StreamID, 1,
			etag.NewHashReader(bytes.NewReader(data), sha256.New()))
		require.NoError(t, err, "failed to put object part")
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
