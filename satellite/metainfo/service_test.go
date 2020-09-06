// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
)

const lastSegmentIndex = -1

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

// TestGetItems_ReturnValueOrder ensures the return value
// of GetItems will always be the same order as the requested paths.
// The test does following steps:
// - Uploads test data (multi-segment objects)
// - Gather all object paths with an extra invalid path at random position
// - Retrieve pointers using above paths
// - Ensure the nil pointer and last segment paths are in the same order as their
// corresponding paths.
func TestGetItems_ReturnValueOrder(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
				testplanet.MaxSegmentSize(3*memory.KiB),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		numItems := 5
		for i := 0; i < numItems; i++ {
			path := fmt.Sprintf("test/path_%d", i)
			err := uplinkPeer.Upload(ctx, satellite, "bucket", path, testrand.Bytes(15*memory.KiB))
			require.NoError(t, err)
		}

		keys, err := satellite.Metainfo.Database.List(ctx, nil, numItems)
		require.NoError(t, err)

		var segmentKeys = make([]metabase.SegmentKey, 0, numItems+1)
		var lastSegmentPathIndices []int

		// Random nil pointer
		nilPointerIndex := testrand.Intn(numItems + 1)

		for i, key := range keys {
			segmentKeys = append(segmentKeys, metabase.SegmentKey(key))
			segmentIdx, err := parseSegmentPath([]byte(key.String()))
			require.NoError(t, err)

			if segmentIdx == lastSegmentIndex {
				lastSegmentPathIndices = append(lastSegmentPathIndices, i)
			}

			// set a random path to be nil.
			if nilPointerIndex == i {
				segmentKeys[nilPointerIndex] = nil
			}
		}

		pointers, err := satellite.Metainfo.Service.GetItems(ctx, segmentKeys)
		require.NoError(t, err)

		for i, p := range pointers {
			if p == nil {
				require.Equal(t, nilPointerIndex, i)
				continue
			}

			meta := pb.StreamMeta{}
			metaInBytes := p.GetMetadata()
			err = pb.Unmarshal(metaInBytes, &meta)
			require.NoError(t, err)

			lastSegmentMeta := meta.GetLastSegmentMeta()
			if lastSegmentMeta != nil {
				require.Equal(t, lastSegmentPathIndices[i], i)
			}
		}
	})
}

func TestUpdatePiecesCheckDuplicates(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 1, 3, 3),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		path := "test/path"

		err := uplinkPeer.Upload(ctx, satellite, "test1", path, testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		keys, err := satellite.Metainfo.Database.List(ctx, nil, 1)
		require.NoError(t, err)
		require.Equal(t, 1, len(keys))

		encPath, err := metabase.ParseSegmentKey(metabase.SegmentKey(keys[0]))
		require.NoError(t, err)
		pointer, err := satellite.Metainfo.Service.Get(ctx, encPath.Encode())
		require.NoError(t, err)

		pieces := pointer.GetRemote().GetRemotePieces()
		require.False(t, hasDuplicates(pointer.GetRemote().GetRemotePieces()))

		// Remove second piece in the list and replace it with
		// a piece on the first node.
		// This way we can ensure that we use a valid piece num.
		removePiece := &pb.RemotePiece{
			PieceNum: pieces[1].PieceNum,
			NodeId:   pieces[1].NodeId,
		}
		addPiece := &pb.RemotePiece{
			PieceNum: pieces[1].PieceNum,
			NodeId:   pieces[0].NodeId,
		}

		// test no duplicates
		updPointer, err := satellite.Metainfo.Service.UpdatePiecesCheckDuplicates(ctx, encPath.Encode(), pointer, []*pb.RemotePiece{addPiece}, []*pb.RemotePiece{removePiece}, true)
		require.True(t, metainfo.ErrNodeAlreadyExists.Has(err))
		require.False(t, hasDuplicates(updPointer.GetRemote().GetRemotePieces()))

		// test allow duplicates
		updPointer, err = satellite.Metainfo.Service.UpdatePieces(ctx, encPath.Encode(), pointer, []*pb.RemotePiece{addPiece}, []*pb.RemotePiece{removePiece})
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

func TestCountBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saPeer := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		projectID := planet.Uplinks[0].Projects[0].ID
		count, err := saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 0, count)
		// Setup: create 2 test buckets
		err = uplinkPeer.CreateBucket(ctx, saPeer, "test1")
		require.NoError(t, err)
		count, err = saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		err = uplinkPeer.CreateBucket(ctx, saPeer, "test2")
		require.NoError(t, err)
		count, err = saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})
}

func parseSegmentPath(segmentPath []byte) (segmentIndex int64, err error) {
	elements := storj.SplitPath(string(segmentPath))
	if len(elements) < 4 {
		return -1, errs.New("invalid path %q", string(segmentPath))
	}

	// var segmentIndex int64
	if elements[1] == "l" {
		segmentIndex = lastSegmentIndex
	} else {
		segmentIndex, err = strconv.ParseInt(elements[1][1:], 10, 64)
		if err != nil {
			return lastSegmentIndex, errs.Wrap(err)
		}
	}
	return segmentIndex, nil
}
