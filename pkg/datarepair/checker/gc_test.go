// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO find correct package for this test
package checker_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/uplink"
)

// TestGarbageCollection does the following:
// * Upload two objects
// * Put one storagenode offline
// * Delete one object
// * Put the storagenode back online
// * Trigger a bloom filter generation
// * Check that pieces of the deleted object are deleted on the storagenode
// * Check that pieces of the kept object are not deleted on the storagenode
func TestGarbageCollection(t *testing.T) {

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// TODO see if we should reconfigure anything
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		uplConfig := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 5,
			MaxThreshold:     5,
		}
		targetNode := planet.StorageNodes[0]

		// Upload two objects
		testData1 := testrand.Bytes(8 * memory.KiB)
		testData2 := testrand.Bytes(8 * memory.KiB)

		err := upl.UploadWithConfig(ctx, satellite, uplConfig, "testbucket", "test/path/1", testData1)
		require.NoError(t, err)
		pointerToDelete := getPointer(ctx, t, satellite, upl, "testbucket", "test/path/1")
		var deletedPiece *pb.RemotePiece
		for _, p := range pointerToDelete.GetRemote().GetRemotePieces() {
			if p.NodeId == targetNode.ID() {
				deletedPiece = p
				break
			}
		}
		require.NotNil(t, deletedPiece)
		deletedPieceID := pointerToDelete.GetRemote().RootPieceId.Derive(targetNode.ID())

		err = upl.UploadWithConfig(ctx, satellite, uplConfig, "testbucket", "test/path/2", testData2)
		require.NoError(t, err)
		pointerToKeep := getPointer(ctx, t, satellite, upl, "testbucket", "test/path/2")
		var keptPiece *pb.RemotePiece
		for _, p := range pointerToKeep.GetRemote().GetRemotePieces() {
			if p.NodeId == targetNode.ID() {
				keptPiece = p
				break
			}
		}
		require.NotNil(t, keptPiece)
		keptPieceID := pointerToKeep.GetRemote().RootPieceId.Derive(targetNode.ID())

		// Take storagenode offline
		err = planet.StopPeer(targetNode)
		require.NoError(t, err)
		_, err = satellite.Overlay.Service.UpdateUptime(ctx, targetNode.ID(), false)
		require.NoError(t, err)

		// Delete object
		upl.Delete(ctx, satellite, "testbucket", "test/path/1")

		// Bring storagenode back online
		// TODO how do we do this?

		// Check that piece of the deleted object is on the storagenode
		pieceInfo, err := targetNode.DB.PieceInfo().Get(ctx, satellite.ID(), deletedPieceID)
		require.NoError(t, err)
		require.NotNil(t, pieceInfo)

		// Trigger bloom filter generation
		// TODO run checker to trigger bloom filter generation?

		// Check that piece of the deleted object is not on the storagenode
		pieceInfo, err = targetNode.DB.PieceInfo().Get(ctx, satellite.ID(), deletedPieceID)
		require.Error(t, err)
		require.Nil(t, pieceInfo)

		// Check that piece of the kept object is on the storagenode
		pieceInfo, err = targetNode.DB.PieceInfo().Get(ctx, satellite.ID(), keptPieceID)
		require.NoError(t, err)
		require.NotNil(t, pieceInfo)
	})
}

func getPointer(ctx *testcontext.Context, t *testing.T, satellite *satellite.Peer, upl *testplanet.Uplink, bucket, path string) *pb.Pointer {
	projects, err := satellite.DB.Console().Projects().GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, projects, 1)

	encScheme := upl.GetConfig(satellite).GetEncryptionScheme()
	cipher := encScheme.Cipher
	unencryptedPathWithBucket := storj.JoinPaths(bucket, path)

	encryptedAfterBucket, err := streams.EncryptAfterBucket(ctx, unencryptedPathWithBucket, cipher, &storj.Key{})
	require.NoError(t, err)

	lastSegPath := storj.JoinPaths(projects[0].ID.String(), "l", encryptedAfterBucket)
	pointer, err := satellite.Metainfo.Service.Get(ctx, lastSegPath)
	require.NoError(t, err)

	return pointer
}
