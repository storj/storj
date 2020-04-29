// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
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
						Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
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

					err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
						Client: testplanet.ClientConfig{
							SegmentSize: 10 * memory.KiB,
						},
					},
						bucketName, objectName, tc.objData,
					)
					require.NoError(t, err)

					// calculate the SNs total used space after data upload
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += piecesTotal
					}

					projectID, encryptedPath := getProjectIDAndEncPathFirstObject(ctx, t, satelliteSys)
					err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
						ctx, projectID, []byte(bucketName), encryptedPath,
					)
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
						Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
					},
				}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
					numToShutdown := 2

					var (
						uplnk        = planet.Uplinks[0]
						satelliteSys = planet.Satellites[0]
					)

					err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
						Client: testplanet.ClientConfig{
							SegmentSize: 10 * memory.KiB,
						},
					}, bucketName, objectName, tc.objData)
					require.NoError(t, err)

					// Shutdown the first numToShutdown storage nodes before we delete the pieces
					require.NoError(t, planet.StopPeer(planet.StorageNodes[0]))
					require.NoError(t, planet.StopPeer(planet.StorageNodes[1]))

					projectID, encryptedPath := getProjectIDAndEncPathFirstObject(ctx, t, satelliteSys)
					err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
						ctx, projectID, []byte(bucketName), encryptedPath,
					)
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
						Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
					},
				}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
					var (
						uplnk        = planet.Uplinks[0]
						satelliteSys = planet.Satellites[0]
					)

					err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
						Client: testplanet.ClientConfig{
							SegmentSize: 10 * memory.KiB,
						},
					}, bucketName, objectName, tc.objData)
					require.NoError(t, err)

					// Shutdown all the storage nodes before we delete the pieces
					for _, sn := range planet.StorageNodes {
						require.NoError(t, planet.StopPeer(sn))
					}

					projectID, encryptedPath := getProjectIDAndEncPathFirstObject(ctx, t, satelliteSys)
					err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
						ctx, projectID, []byte(bucketName), encryptedPath,
					)
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

func TestEndpoint_DeleteObjectPieces_ObjectWithoutLastSegment(t *testing.T) {
	t.Run("continuous segments", func(t *testing.T) {
		t.Parallel()

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			var (
				uplnk        = planet.Uplinks[0]
				satelliteSys = planet.Satellites[0]
			)

			const segmentSize = 10 * memory.KiB

			var testCases = []struct {
				caseDescription string
				objData         []byte
			}{
				{
					caseDescription: "one segment",
					objData:         testrand.Bytes(2 * segmentSize),
				},
				{
					caseDescription: "several segments",
					objData:         testrand.Bytes(4 * segmentSize),
				},
				{
					caseDescription: "several segments last inline",
					objData:         testrand.Bytes((2 * segmentSize) + (3 * memory.KiB)),
				},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.caseDescription, func(t *testing.T) {
					const bucketName = "a-bucket"
					// Use a different name for avoid collisions without having to run
					// testplanet for each test cases. We cannot upload to the same path
					// because it fails due to the zombie segments left by previous test
					// cases
					var objectName = tc.caseDescription

					projectID, encryptedPath := uploadFirstObjectWithoutLastSegmentPointer(
						ctx, t, uplnk, satelliteSys, segmentSize, bucketName, objectName, tc.objData,
					)

					// calculate the SNs total used space after data upload
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						usedSpace, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += usedSpace
					}

					err := satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
						ctx, projectID, []byte(bucketName), encryptedPath,
					)
					require.NoError(t, err)

					// confirm that the object was deleted
					err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
						ctx, projectID, []byte(bucketName), encryptedPath,
					)
					require.Error(t, err)
					require.Equal(t, rpcstatus.Code(err), rpcstatus.NotFound)

					planet.WaitForStorageNodeDeleters(ctx)

					// calculate the SNs used space after delete the pieces
					var totalUsedSpaceAfterDelete int64
					for _, sn := range planet.StorageNodes {
						usedSpace, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpaceAfterDelete += usedSpace
					}

					if totalUsedSpaceAfterDelete >= totalUsedSpace {
						t.Fatalf(
							"used space after deletion. want before > after, got %d <= %d",
							totalUsedSpace, totalUsedSpaceAfterDelete,
						)
					}
				})
			}
		})
	})

	t.Run("sparse segments", func(t *testing.T) {
		t.Parallel()

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				// Reconfigure RS for ensuring that we don't have long-tail cancellations
				// and the upload doesn't leave garbage in the SNs
				Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			var (
				uplnk        = planet.Uplinks[0]
				satelliteSys = planet.Satellites[0]
			)

			const segmentSize = 10 * memory.KiB

			var testCases = []struct {
				caseDescription          string
				objData                  []byte
				noSegmentsIndexes        []int64 // Witout the last segment which is always included
				expectedMaxGarbageFactor float64
				expectedNotFoundErr      bool
			}{
				{
					caseDescription:     "some firsts",
					objData:             testrand.Bytes(10 * segmentSize),
					noSegmentsIndexes:   []int64{3, 5, 6, 9}, // Object with no pointers: L, 3, 5, 6, 9
					expectedNotFoundErr: false,
				},
				{
					caseDescription:     "some firsts inline",
					objData:             testrand.Bytes((9 * segmentSize) + (3 * memory.KiB)),
					noSegmentsIndexes:   []int64{4, 5, 6}, // Object with no pointers: L, 4, 5, 6
					expectedNotFoundErr: false,
				},
				{
					caseDescription:     "no first",
					objData:             testrand.Bytes(10 * segmentSize),
					noSegmentsIndexes:   []int64{0}, // Object with no pointer to : L, 0
					expectedNotFoundErr: true,
				},
				{
					caseDescription:     "no firsts",
					objData:             testrand.Bytes(8 * segmentSize),
					noSegmentsIndexes:   []int64{0, 2, 5}, // Object with no pointer to : L, 0, 2, 5
					expectedNotFoundErr: true,
				},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.caseDescription, func(t *testing.T) {
					const bucketName = "a-bucket"
					// Use a different name for avoid collisions without having to run
					// testplanet for each test cases. We cannot upload to the same path
					// because it fails due to the zombie segments left by previous test
					// cases
					var objectName = tc.caseDescription

					// add the last segment to the indicated no segments to upload
					noSegmentsIndexes := []int64{-1}
					noSegmentsIndexes = append(noSegmentsIndexes, tc.noSegmentsIndexes...)
					projectID, encryptedPath := uploadFirstObjectWithoutSomeSegmentsPointers(
						ctx, t, uplnk, satelliteSys, segmentSize, bucketName, objectName, tc.objData, noSegmentsIndexes,
					)

					// calculate the SNs used space
					var totalUsedSpace int64
					for _, sn := range planet.StorageNodes {
						usedSpace, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpace += usedSpace
					}

					err := satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
						ctx, projectID, []byte(bucketName), encryptedPath,
					)
					if tc.expectedNotFoundErr {
						require.Error(t, err)
						require.Equal(t, rpcstatus.Code(err), rpcstatus.NotFound)
						return
					}

					require.NoError(t, err)

					// confirm that the object was deleted
					err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
						ctx, projectID, []byte(bucketName), encryptedPath,
					)
					require.Error(t, err)
					require.Equal(t, rpcstatus.Code(err), rpcstatus.NotFound)

					planet.WaitForStorageNodeDeleters(ctx)

					// calculate the SNs used space after delete the pieces
					var totalUsedSpaceAfterDelete int64
					for _, sn := range planet.StorageNodes {
						usedSpace, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
						require.NoError(t, err)
						totalUsedSpaceAfterDelete += usedSpace
					}

					if totalUsedSpaceAfterDelete >= totalUsedSpace {
						t.Fatalf(
							"used space after deletion. want before > after, got %d <= %d",
							totalUsedSpace, totalUsedSpaceAfterDelete,
						)
					}
				})
			}
		})
	})
}

func getProjectIDAndEncPathFirstObject(
	ctx context.Context, t *testing.T, satellite *testplanet.Satellite,
) (projectID uuid.UUID, encryptedPath []byte) {
	t.Helper()

	keys, err := satellite.Metainfo.Database.List(ctx, storage.Key{}, 1)
	require.NoError(t, err)
	keyParts := storj.SplitPath(keys[0].String())
	require.Len(t, keyParts, 4)

	projectID, err = uuid.FromString(keyParts[0])
	require.NoError(t, err)
	encryptedPath = []byte(keyParts[3])

	return projectID, encryptedPath
}

func uploadFirstObjectWithoutLastSegmentPointer(
	ctx context.Context, t *testing.T, uplnk *testplanet.Uplink,
	satelliteSys *testplanet.Satellite, segmentSize memory.Size,
	bucketName string, objectName string, objectData []byte,
) (projectID uuid.UUID, encryptedPath []byte) {
	t.Helper()

	return uploadFirstObjectWithoutSomeSegmentsPointers(
		ctx, t, uplnk, satelliteSys, segmentSize, bucketName, objectName, objectData, []int64{-1},
	)
}

func uploadFirstObjectWithoutSomeSegmentsPointers(
	ctx context.Context, t *testing.T, uplnk *testplanet.Uplink,
	satelliteSys *testplanet.Satellite, segmentSize memory.Size,
	bucketName string, objectName string, objectData []byte, noSegmentsIndexes []int64,
) (projectID uuid.UUID, encryptedPath []byte) {
	t.Helper()

	if len(noSegmentsIndexes) < 1 {
		t.Fatal("noSegments list must have at least one segment")
	}

	err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
		Client: testplanet.ClientConfig{
			SegmentSize: segmentSize,
		},
	},
		bucketName, objectName, objectData,
	)
	require.NoError(t, err)

	projectID, encryptedPath = getProjectIDAndEncPathFirstObject(ctx, t, satelliteSys)
	for _, segIndx := range noSegmentsIndexes {
		path, err := metainfo.CreatePath(ctx, projectID, segIndx, []byte(bucketName), encryptedPath)
		require.NoError(t, err)
		err = satelliteSys.Metainfo.Service.UnsynchronizedDelete(ctx, path)
		require.NoError(t, err)
	}

	return projectID, encryptedPath
}
