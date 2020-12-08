// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestEndpoint_DeleteObjectPieces(t *testing.T) {
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

		for i, tc := range testCases {
			i := i
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
						uplnk        = planet.Uplinks[0]
						satelliteSys = planet.Satellites[0]
					)

					var (
						bucketName = "a-bucket"
						objectName = "object-filename" + strconv.Itoa(i)
						percentExp = 0.75
					)

					err := uplnk.Upload(ctx, satelliteSys, bucketName, objectName, tc.objData)
					require.NoError(t, err)

					// calculate the SNs total used space after data upload
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					projectID := uplnk.Projects[0].ID
					items, err := satelliteSys.Metainfo.Metabase.TestingAllCommittedObjects(ctx, projectID, bucketName)
					require.NoError(t, err)
					require.Len(t, items, 1)

					_, err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(ctx, projectID, bucketName, items[0].ObjectKey)
					require.NoError(t, err)

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

				const (
					bucketName = "a-bucket"
					objectName = "object-filename"
				)

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

					var (
						uplnk        = planet.Uplinks[0]
						satelliteSys = planet.Satellites[0]
					)

					err := uplnk.Upload(ctx, satelliteSys, bucketName, objectName, tc.objData)
					require.NoError(t, err)

					// Shutdown the first numToShutdown storage nodes before we delete the pieces
					require.NoError(t, planet.StopPeer(planet.StorageNodes[0]))
					require.NoError(t, planet.StopPeer(planet.StorageNodes[1]))

					projectID := uplnk.Projects[0].ID
					items, err := satelliteSys.Metainfo.Metabase.TestingAllCommittedObjects(ctx, projectID, bucketName)
					require.NoError(t, err)
					require.Len(t, items, 1)

					_, err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(ctx, projectID, bucketName, items[0].ObjectKey)
					require.NoError(t, err)

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
				const (
					bucketName = "a-bucket"
					objectName = "object-filename"
				)
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
						uplnk        = planet.Uplinks[0]
						satelliteSys = planet.Satellites[0]
					)

					err := uplnk.Upload(ctx, satelliteSys, bucketName, objectName, tc.objData)
					require.NoError(t, err)

					// Shutdown all the storage nodes before we delete the pieces
					for _, sn := range planet.StorageNodes {
						require.NoError(t, planet.StopPeer(sn))
					}

					projectID := uplnk.Projects[0].ID
					items, err := satelliteSys.Metainfo.Metabase.TestingAllCommittedObjects(ctx, projectID, bucketName)
					require.NoError(t, err)
					require.Len(t, items, 1)

					_, err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(ctx, projectID, bucketName, items[0].ObjectKey)
					require.NoError(t, err)

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

		listResp, err := satelliteSys.API.Metainfo.Endpoint2.ListObjects(ctx, &pb.ObjectListRequest{
			Header: &pb.RequestHeader{
				ApiKey: apiKey.SerializeRaw(),
			},
			Bucket: []byte(expectedBucketName),
		})
		require.NoError(t, err)
		require.Len(t, listResp.GetItems(), 3)

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
