// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/encryption"
	"storj.io/common/memory"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storage"
)

// TestGarbageCollection does the following:
// * Set up a network with one storagenode
// * Upload two objects
// * Delete one object from the metainfo service on the satellite
// * Wait for bloom filter generation
// * Check that pieces of the deleted object are deleted on the storagenode
// * Check that pieces of the kept object are not deleted on the storagenode
func TestGarbageCollection(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.GarbageCollection.FalsePositiveRate = 0.000000001
				config.GarbageCollection.Interval = 500 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		targetNode := planet.StorageNodes[0]
		gcService := satellite.GarbageCollection.Service
		gcService.Loop.Pause()

		// Upload two objects
		testData1 := testrand.Bytes(8 * memory.KiB)
		testData2 := testrand.Bytes(8 * memory.KiB)

		err := upl.Upload(ctx, satellite, "testbucket", "test/path/1", testData1)
		require.NoError(t, err)
		deletedEncPath, pointerToDelete := getPointer(ctx, t, satellite, upl, "testbucket", "test/path/1")
		var deletedPieceID storj.PieceID
		for _, p := range pointerToDelete.GetRemote().GetRemotePieces() {
			if p.NodeId == targetNode.ID() {
				deletedPieceID = pointerToDelete.GetRemote().RootPieceId.Derive(p.NodeId, p.PieceNum)
				break
			}
		}
		require.NotZero(t, deletedPieceID)

		err = upl.Upload(ctx, satellite, "testbucket", "test/path/2", testData2)
		require.NoError(t, err)
		_, pointerToKeep := getPointer(ctx, t, satellite, upl, "testbucket", "test/path/2")
		var keptPieceID storj.PieceID
		for _, p := range pointerToKeep.GetRemote().GetRemotePieces() {
			if p.NodeId == targetNode.ID() {
				keptPieceID = pointerToKeep.GetRemote().RootPieceId.Derive(p.NodeId, p.PieceNum)
				break
			}
		}
		require.NotZero(t, keptPieceID)

		// Delete one object from metainfo service on satellite
		err = satellite.Metainfo.Service.UnsynchronizedDelete(ctx, deletedEncPath)
		require.NoError(t, err)

		// Check that piece of the deleted object is on the storagenode
		pieceAccess, err := targetNode.DB.Pieces().Stat(ctx, storage.BlobRef{
			Namespace: satellite.ID().Bytes(),
			Key:       deletedPieceID.Bytes(),
		})
		require.NoError(t, err)
		require.NotNil(t, pieceAccess)

		// The pieceInfo.GetPieceIDs query converts piece creation and the filter creation timestamps
		// to datetime in sql. This chops off all precision beyond seconds.
		// In this test, the amount of time that elapses between piece uploads and the gc loop is
		// less than a second, meaning datetime(piece_creation) < datetime(filter_creation) is false unless we sleep
		// for a second.
		time.Sleep(1 * time.Second)

		// Wait for next iteration of garbage collection to finish
		gcService.Loop.Restart()
		gcService.Loop.TriggerWait()

		// Wait for the storagenode's RetainService queue to be empty
		targetNode.Storage2.RetainService.TestWaitUntilEmpty()

		// Check that piece of the deleted object is not on the storagenode
		pieceAccess, err = targetNode.DB.Pieces().Stat(ctx, storage.BlobRef{
			Namespace: satellite.ID().Bytes(),
			Key:       deletedPieceID.Bytes(),
		})
		require.Error(t, err)
		require.Nil(t, pieceAccess)

		// Check that piece of the kept object is on the storagenode
		pieceAccess, err = targetNode.DB.Pieces().Stat(ctx, storage.BlobRef{
			Namespace: satellite.ID().Bytes(),
			Key:       keptPieceID.Bytes(),
		})
		require.NoError(t, err)
		require.NotNil(t, pieceAccess)
	})
}

func getPointer(ctx *testcontext.Context, t *testing.T, satellite *testplanet.SatelliteSystem, upl *testplanet.Uplink, bucket, path string) (lastSegPath string, pointer *pb.Pointer) {
	projects, err := satellite.DB.Console().Projects().GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, projects, 1)

	encParameters := upl.GetConfig(satellite).GetEncryptionParameters()
	cipherSuite := encParameters.CipherSuite
	store := encryption.NewStore()
	store.SetDefaultKey(new(storj.Key))
	encryptedPath, err := encryption.EncryptPath(bucket, paths.NewUnencrypted(path), cipherSuite, store)
	require.NoError(t, err)

	lastSegPath = storj.JoinPaths(projects[0].ID.String(), "l", bucket, encryptedPath.Raw())
	pointer, err = satellite.Metainfo.Service.Get(ctx, lastSegPath)
	require.NoError(t, err)

	return lastSegPath, pointer
}
