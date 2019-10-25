// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
)

func TestIterate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saPeer := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		// Setup: create 2 test buckets
		err := uplinkPeer.CreateBucket(ctx, saPeer, "test1")
		require.NoError(t, err)
		err = uplinkPeer.CreateBucket(ctx, saPeer, "test2")
		require.NoError(t, err)

		// Setup: upload an object in one of the buckets
		expectedData := testrand.Bytes(50 * memory.KiB)
		err = uplinkPeer.Upload(ctx, saPeer, "test2", "test/path", expectedData)
		require.NoError(t, err)

		// Test: Confirm that only the objects are in pointerDB
		// and not the bucket metadata
		var itemCount int
		err = saPeer.Metainfo.Database.Iterate(ctx, storage.IterateOptions{Recurse: true},
			func(ctx context.Context, it storage.Iterator) error {
				var item storage.ListItem
				for it.Next(ctx, &item) {
					itemCount++
					pathElements := storj.SplitPath(storj.Path(item.Key))
					// there should not be any objects in pointerDB with less than 4 path
					// elements. i.e buckets should not be stored in pointerDB
					require.True(t, len(pathElements) > 3)
				}
				return nil
			})
		require.NoError(t, err)
		// There should only be 1 item in pointerDB, the one object
		require.Equal(t, 1, itemCount)
	})
}

func TestUpdatePiecesDuplicateNodeID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saPeer := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		// create test buckets
		err := uplinkPeer.CreateBucket(ctx, saPeer, "test1")
		require.NoError(t, err)

		// upload an object
		expectedData := testrand.Bytes(50 * memory.KiB)
		err = uplinkPeer.Upload(ctx, saPeer, "test1", "test/path", expectedData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, saPeer)

		duplicatedNodeID := storj.NodeID{}
		pieceToRemove := make([]*pb.RemotePiece, 1)
		pieceToAdd := make([]*pb.RemotePiece, 1)
		pieces := pointer.GetRemote().GetRemotePieces()
		for _, piece := range pieces {
			if pieceToRemove[0] == nil {
				pieceToRemove[0] = piece
				continue
			}

			if pieceToRemove[0].NodeId != piece.NodeId {
				duplicatedNodeID = piece.NodeId
				break
			}
		}

		// create a piece with deleted piece number and duplicated node ID from the pointer
		pieceToAdd[0] = &pb.RemotePiece{
			PieceNum: pieceToRemove[0].PieceNum,
			NodeId:   duplicatedNodeID,
		}

		updated, err := saPeer.Metainfo.Service.UpdatePieces(ctx, path, pointer, pieceToAdd, pieceToRemove)
		require.Error(t, err)
		require.True(t, metainfo.ErrDuplicatedNodeID.Has(err))
		require.Nil(t, updated)
	})
}

// getRemoteSegment returns a remote pointer its path from satellite.
func getRemoteSegment(
	ctx context.Context, t *testing.T, satellite *testplanet.SatelliteSystem,
) (_ *pb.Pointer, path string) {
	t.Helper()

	// get a remote segment from metainfo
	metainfo := satellite.Metainfo.Service
	listResponse, _, err := metainfo.List(ctx, "", "", "", true, 0, 0)
	require.NoError(t, err)

	for _, v := range listResponse {
		path := v.GetPath()
		pointer, err := metainfo.Get(ctx, path)
		require.NoError(t, err)
		if pointer.GetType() == pb.Pointer_REMOTE {
			return pointer, path
		}
	}

	t.Fatal("satellite doesn't have any remote segment")
	return nil, ""
}
