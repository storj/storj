// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestIdentifyInjuredSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		checker.Loop.Stop()

		//add noise to pointerdb before bad record
		for x := 0; x < 1000; x++ {
			makePointer(t, planet, fmt.Sprintf("a-%d", x), false)
		}
		//create piece that needs repair
		makePointer(t, planet, fmt.Sprintf("b"), true)
		//add more noise to pointerdb after bad record
		for x := 0; x < 1000; x++ {
			makePointer(t, planet, fmt.Sprintf("c-%d", x), false)
		}
		err := checker.IdentifyInjuredSegments(ctx)
		assert.NoError(t, err)

		//check if the expected segments were added to the queue
		repairQueue := planet.Satellites[0].DB.RepairQueue()
		injuredSegment, err := repairQueue.Dequeue(ctx)
		assert.NoError(t, err)

		numValidNode := int32(len(planet.StorageNodes))
		assert.Equal(t, "b", injuredSegment.Path)
		assert.Equal(t, len(planet.StorageNodes), len(injuredSegment.LostPieces))
		for _, lostPiece := range injuredSegment.LostPieces {
			assert.True(t, lostPiece >= numValidNode, fmt.Sprintf("%d >= %d \n", lostPiece, numValidNode))
			assert.True(t, lostPiece < numValidNode*2, fmt.Sprintf("%d < %d \n", lostPiece, numValidNode*2))
		}
	})
}

func TestOfflineNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		checker.Loop.Stop()

		const numberOfNodes = 10
		nodeIDs := storj.NodeIDList{}

		// use online nodes
		for _, storagenode := range planet.StorageNodes {
			nodeIDs = append(nodeIDs, storagenode.Identity.ID)
		}

		// simulate offline nodes
		expectedOffline := make([]int32, 0)
		for i := len(nodeIDs); i < numberOfNodes; i++ {
			nodeIDs = append(nodeIDs, storj.NodeID{byte(i)})
			expectedOffline = append(expectedOffline, int32(i))
		}

		offline, err := checker.OfflineNodes(ctx, nodeIDs)
		assert.NoError(t, err)
		assert.Equal(t, expectedOffline, offline)
	})
}

func TestIdentifyIrreparableSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		checker.Loop.Stop()

		const numberOfNodes = 10
		pieces := make([]*pb.RemotePiece, 0, numberOfNodes)
		// use online nodes
		for i, storagenode := range planet.StorageNodes {
			pieces = append(pieces, &pb.RemotePiece{
				PieceNum: int32(i),
				NodeId:   storagenode.ID(),
			})
		}

		// simulate offline nodes
		expectedLostPieces := make(map[int32]bool)
		for i := len(pieces); i < numberOfNodes; i++ {
			pieces = append(pieces, &pb.RemotePiece{
				PieceNum: int32(i),
				NodeId:   storj.NodeID{byte(i)},
			})
			expectedLostPieces[int32(i)] = true
		}
		pointer := &pb.Pointer{
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					MinReq:          int32(4),
					RepairThreshold: int32(8),
				},
				PieceId:      "fake-piece-id",
				RemotePieces: pieces,
			},
		}

		// put test pointer to db
		pointerdb := planet.Satellites[0].Metainfo.Service
		err := pointerdb.Put(pointer.Remote.PieceId, pointer)
		assert.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		assert.NoError(t, err)

		// check if nothing was added to repair queue
		repairQueue := planet.Satellites[0].DB.RepairQueue()
		_, err = repairQueue.Dequeue(ctx)
		assert.True(t, storage.ErrEmptyQueue.Has(err))

		//check if the expected segments were added to the irreparable DB
		irreparable := planet.Satellites[0].DB.Irreparable()
		remoteSegmentInfo, err := irreparable.Get(ctx, []byte("fake-piece-id"))
		assert.NoError(t, err)

		assert.Equal(t, len(expectedLostPieces), int(remoteSegmentInfo.LostPiecesCount))
		assert.Equal(t, 1, int(remoteSegmentInfo.RepairAttemptCount))
		firstRepair := remoteSegmentInfo.RepairUnixSec

		// check irreparable once again but wait a second
		time.Sleep(1 * time.Second)
		err = checker.IdentifyInjuredSegments(ctx)
		assert.NoError(t, err)

		remoteSegmentInfo, err = irreparable.Get(ctx, []byte("fake-piece-id"))
		assert.NoError(t, err)

		assert.Equal(t, len(expectedLostPieces), int(remoteSegmentInfo.LostPiecesCount))
		// check if repair attempt count was incremented
		assert.Equal(t, 2, int(remoteSegmentInfo.RepairAttemptCount))
		assert.True(t, firstRepair < remoteSegmentInfo.RepairUnixSec)
	})
}

func makePointer(t *testing.T, planet *testplanet.Planet, peiceID string, createLost bool) {
	numOfStorageNodes := len(planet.StorageNodes)
	pieces := make([]*pb.RemotePiece, 0, numOfStorageNodes)
	// use online nodes
	for i := 0; i < numOfStorageNodes; i++ {
		pieces = append(pieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   planet.StorageNodes[i].Identity.ID,
		})
	}
	// simulate offline nodes equal to the number of online nodes
	if createLost {
		for i := 0; i < numOfStorageNodes; i++ {
			pieces = append(pieces, &pb.RemotePiece{
				PieceNum: int32(numOfStorageNodes + i),
				NodeId:   storj.NodeID{byte(i)},
			})
		}
	}
	minReq, repairThreshold := numOfStorageNodes-1, numOfStorageNodes-1
	if createLost {
		minReq, repairThreshold = numOfStorageNodes-1, numOfStorageNodes+1
	}
	pointer := &pb.Pointer{
		Remote: &pb.RemoteSegment{
			Redundancy: &pb.RedundancyScheme{
				MinReq:          int32(minReq),
				RepairThreshold: int32(repairThreshold),
			},
			PieceId:      peiceID,
			RemotePieces: pieces,
		},
	}
	// put test pointer to db
	pointerdb := planet.Satellites[0].Metainfo.Service
	err := pointerdb.Put(pointer.Remote.PieceId, pointer)
	require.NoError(t, err)
}
