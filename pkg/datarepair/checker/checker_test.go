// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestIdentifyInjuredSegments(t *testing.T) {
	// TODO note satellite's: own sub-systems need to be disabled
	// TODO test irreparable ??

	tctx := testcontext.New(t)
	defer tctx.Cleanup()

	const numberOfNodes = 10
	planet, err := testplanet.New(t, 1, 4, 0)
	require.NoError(t, err)
	defer tctx.Check(planet.Shutdown)

	planet.Start(tctx)
	time.Sleep(2 * time.Second)

	pieces := make([]*pb.RemotePiece, 0, numberOfNodes)
	// use online nodes
	for i, storagenode := range planet.StorageNodes {
		pieces = append(pieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   storagenode.Identity.ID,
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
	err = pointerdb.Put(pointer.Remote.PieceId, pointer)
	assert.NoError(t, err)

	checker := planet.Satellites[0].Repair.Checker
	err = checker.IdentifyInjuredSegments(tctx)
	assert.NoError(t, err)

	//check if the expected segments were added to the queue
	repairQueue := planet.Satellites[0].DB.RepairQueue()
	injuredSegment, err := repairQueue.Dequeue(tctx)
	assert.NoError(t, err)

	assert.Equal(t, "fake-piece-id", injuredSegment.Path)
	assert.Equal(t, len(expectedLostPieces), len(injuredSegment.LostPieces))
	for _, lostPiece := range injuredSegment.LostPieces {
		if !expectedLostPieces[lostPiece] {
			t.Error("should be lost: ", lostPiece)
		}
	}
}

func TestOfflineNodes(t *testing.T) {
	tctx := testcontext.New(t)
	defer tctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	require.NoError(t, err)
	defer tctx.Check(planet.Shutdown)

	planet.Start(tctx)
	time.Sleep(2 * time.Second)

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

	checker := planet.Satellites[0].Repair.Checker
	offline, err := checker.OfflineNodes(tctx, nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expectedOffline, offline)
}

func BenchmarkIdentifyInjuredSegments(b *testing.B) {
	tctx := testcontext.New(b)
	defer tctx.Cleanup()

	const numberOfNodes = 10
	planet, err := testplanet.New(b, 1, 4, 0)
	require.NoError(b, err)
	defer tctx.Check(planet.Shutdown)

	planet.Start(tctx)
	time.Sleep(2 * time.Second)

	pieces := make([]*pb.RemotePiece, 0, numberOfNodes)
	// use online nodes
	for i, storagenode := range planet.StorageNodes {
		pieces = append(pieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   storagenode.Identity.ID,
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

	pointerdb := planet.Satellites[0].Metainfo.Service
	err = pointerdb.Put(pointer.Remote.PieceId, pointer)
	assert.NoError(b, err)

	repairQueue := planet.Satellites[0].DB.RepairQueue()
	checker := planet.Satellites[0].Repair.Checker

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = checker.IdentifyInjuredSegments(tctx)
		assert.NoError(b, err)

		injuredSegment, err := repairQueue.Dequeue(tctx)
		assert.NoError(b, err)

		assert.Equal(b, "fake-piece-id", injuredSegment.Path)
		assert.Equal(b, len(expectedLostPieces), len(injuredSegment.LostPieces))
		for _, lostPiece := range injuredSegment.LostPieces {
			assert.Equal(b, true, expectedLostPieces[lostPiece])
		}
	}
}
