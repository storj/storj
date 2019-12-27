// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storage"
	"storj.io/storj/uplink"
)

func TestEndpoint_DeleteObjectPieces(t *testing.T) {
	t.Run("all nodes up", func(t *testing.T) {
		t.Parallel()

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)
		planet.Start(ctx)

		var (
			uplnk        = planet.Uplinks[0]
			satelliteSys = planet.Satellites[0]
		)

		var testCases = []struct {
			caseDescription string
			objData         []byte
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "one inline segment", objData: testrand.Bytes(3 * memory.KiB)},
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

				// Use RSConfig for ensuring that we don't have long-tail cancellations and the
				// upload doesn't leave garbage in the SNs
				err = uplnk.UploadWithClientConfig(ctx, satelliteSys, uplink.Config{
					Client: uplink.ClientConfig{
						SegmentSize: 10 * memory.KiB,
					},
					RS: uplink.RSConfig{
						MinThreshold:     2,
						RepairThreshold:  2,
						SuccessThreshold: 4,
						MaxThreshold:     4,
					},
				},
					bucketName, objectName, tc.objData,
				)
				require.NoError(t, err)

				projectID, encryptedPath := getProjectIDAndEncPathFirstObject(ctx, t, satelliteSys)
				err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
					ctx, *projectID, []byte(bucketName), encryptedPath,
				)
				require.NoError(t, err)

				// Check that storage nodes don't hold any data after the satellite
				// delete the pieces
				var totalUsedSpace int64
				for _, sn := range planet.StorageNodes {
					usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
					require.NoError(t, err)
					totalUsedSpace += usedSpace
				}

				require.Zero(t, totalUsedSpace, "totalUsedSpace")
			})
		}
	})

	t.Run("some nodes down", func(t *testing.T) {
		t.Skip("TODO: v3-3364")
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "one inline segment", objData: testrand.Bytes(3 * memory.KiB)},
			{caseDescription: "several segments (all remote)", objData: testrand.Bytes(50 * memory.KiB)},
			{caseDescription: "several segments (remote + inline)", objData: testrand.Bytes(33 * memory.KiB)},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.caseDescription, func(t *testing.T) {
				ctx := testcontext.New(t)
				defer ctx.Cleanup()

				planet, err := testplanet.New(t, 1, 4, 1)
				require.NoError(t, err)
				defer ctx.Check(planet.Shutdown)
				planet.Start(ctx)

				var (
					uplnk        = planet.Uplinks[0]
					satelliteSys = planet.Satellites[0]
				)

				const (
					bucketName = "a-bucket"
					objectName = "object-filename"
				)

				// Use RSConfig for ensuring that we don't have long-tail cancellations and the
				// upload doesn't leave garbage in the SNs
				err = uplnk.UploadWithClientConfig(ctx, satelliteSys, uplink.Config{
					Client: uplink.ClientConfig{
						SegmentSize: 10 * memory.KiB,
					},
					RS: uplink.RSConfig{
						MinThreshold:     2,
						RepairThreshold:  2,
						SuccessThreshold: 4,
						MaxThreshold:     4,
					},
				}, bucketName, objectName, tc.objData)
				require.NoError(t, err)

				// Shutdown the first 2 storage nodes before we delete the pieces
				require.NoError(t, planet.StopPeer(planet.StorageNodes[0]))
				require.NoError(t, planet.StopPeer(planet.StorageNodes[1]))

				projectID, encryptedPath := getProjectIDAndEncPathFirstObject(ctx, t, satelliteSys)
				err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
					ctx, *projectID, []byte(bucketName), encryptedPath,
				)
				require.NoError(t, err)

				// Check that storage nodes that were offline when deleting the pieces
				// they are still holding data
				var totalUsedSpace int64
				for i := 0; i < 2; i++ {
					usedSpace, err := planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
					require.NoError(t, err)
					totalUsedSpace += usedSpace
				}

				require.NotZero(t, totalUsedSpace, "totalUsedSpace offline nodes")

				// Check that storage nodes which are online when deleting pieces don't
				// hold any piece
				totalUsedSpace = 0
				for i := 2; i < len(planet.StorageNodes); i++ {
					usedSpace, err := planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
					require.NoError(t, err)
					totalUsedSpace += usedSpace
				}

				require.Zero(t, totalUsedSpace, "totalUsedSpace online nodes")
			})
		}
	})

	t.Run("all nodes down", func(t *testing.T) {
		t.Skip("TODO: v3-3364")
		t.Parallel()

		var testCases = []struct {
			caseDescription string
			objData         []byte
		}{
			{caseDescription: "one remote segment", objData: testrand.Bytes(10 * memory.KiB)},
			{caseDescription: "one inline segment", objData: testrand.Bytes(3 * memory.KiB)},
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

				ctx := testcontext.New(t)
				defer ctx.Cleanup()

				planet, err := testplanet.New(t, 1, 4, 1)
				require.NoError(t, err)
				defer ctx.Check(planet.Shutdown)
				planet.Start(ctx)

				var (
					uplnk        = planet.Uplinks[0]
					satelliteSys = planet.Satellites[0]
				)

				// Use RSConfig for ensuring that we don't have long-tail cancellations and the
				// upload doesn't leave garbage in the SNs
				err = uplnk.UploadWithClientConfig(ctx, satelliteSys, uplink.Config{
					Client: uplink.ClientConfig{
						SegmentSize: 10 * memory.KiB,
					},
					RS: uplink.RSConfig{
						MinThreshold:     2,
						RepairThreshold:  2,
						SuccessThreshold: 4,
						MaxThreshold:     4,
					},
				}, bucketName, objectName, tc.objData)
				require.NoError(t, err)

				// Shutdown all the storage nodes before we delete the pieces
				for _, sn := range planet.StorageNodes {
					require.NoError(t, planet.StopPeer(sn))
				}

				projectID, encryptedPath := getProjectIDAndEncPathFirstObject(ctx, t, satelliteSys)
				err = satelliteSys.Metainfo.Endpoint2.DeleteObjectPieces(
					ctx, *projectID, []byte(bucketName), encryptedPath,
				)
				require.NoError(t, err)

				// Check that storage nodes that were offline when deleting the pieces
				// they are still holding data
				var totalUsedSpace int64
				for _, sn := range planet.StorageNodes {
					usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
					require.NoError(t, err)
					totalUsedSpace += usedSpace
				}

				require.NotZero(t, totalUsedSpace, "totalUsedSpace")
			})
		}
	})
}

func getProjectIDAndEncPathFirstObject(
	ctx context.Context, t *testing.T, satellite *testplanet.SatelliteSystem,
) (projectID *uuid.UUID, encryptedPath []byte) {
	t.Helper()

	keys, err := satellite.Metainfo.Database.List(ctx, storage.Key{}, 1)
	require.NoError(t, err)
	keyParts := storj.SplitPath(keys[0].String())
	require.Len(t, keyParts, 4)
	projectID, err = uuid.Parse(keyParts[0])
	require.NoError(t, err)
	encryptedPath = []byte(keyParts[3])

	return projectID, encryptedPath
}
