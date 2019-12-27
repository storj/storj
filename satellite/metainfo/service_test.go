// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
	"storj.io/storj/uplink"
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

func TestUpdatePiecesCheckDuplicates(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		path := "test/path"

		rs := &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  1,
			SuccessThreshold: 2,
			MaxThreshold:     2,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "test1", path, testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		keys, err := satellite.Metainfo.Database.List(ctx, nil, 1)
		require.NoError(t, err)

		var pointer *pb.Pointer
		var encPath string
		for _, key := range keys {
			encPath = string(key)
			pointer, err = satellite.Metainfo.Service.Get(ctx, encPath)
			require.NoError(t, err)
			break
		}
		require.NotNil(t, pointer)
		require.NotNil(t, encPath)

		pieces := pointer.GetRemote().GetRemotePieces()
		require.False(t, hasDuplicates(pointer.GetRemote().GetRemotePieces()))

		piece := pieces[0]
		piece.PieceNum = 3

		// test no duplicates
		updPointer, err := satellite.Metainfo.Service.UpdatePiecesCheckDuplicates(ctx, encPath, pointer, []*pb.RemotePiece{piece}, nil, true)
		require.True(t, metainfo.ErrNodeAlreadyExists.Has(err))
		require.False(t, hasDuplicates(updPointer.GetRemote().GetRemotePieces()))

		// test allow duplicates
		updPointer, err = satellite.Metainfo.Service.UpdatePieces(ctx, encPath, pointer, []*pb.RemotePiece{piece}, nil)
		require.NoError(t, err)
		require.True(t, hasDuplicates(updPointer.GetRemote().GetRemotePieces()))
	})
}

func hasDuplicates(pieces []*pb.RemotePiece) bool {
	nodePieceCounts := make(map[storj.NodeID]int)
	for _, piece := range pieces {
		nodePieceCounts[piece.NodeId]++
	}

	for _, count := range nodePieceCounts {
		if count > 1 {
			return true
		}
	}

	return false
}
