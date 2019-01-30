// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
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

	planet, err := testplanet.New(t, 1, 5, 0)
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
	assert.NoError(t, err)
	offline, err := checker.OfflineNodes(tctx, nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expectedOffline, offline)
}

func BenchmarkIdentifyInjuredSegments(b *testing.B) {
	//b.Skip("needs update")

	tctx := testcontext.New(b)
	defer tctx.Cleanup()

	planet, err := testplanet.New(b, 1, 0, 0)
	require.NoError(b, err)
	defer tctx.Check(planet.Shutdown)

	planet.Start(tctx)
	time.Sleep(2 * time.Second)

	pointerdb := planet.Satellites[0].Metainfo.Service
	repairQueue := planet.Satellites[0].DB.RepairQueue()

	// creating in-memory db and opening connection
	// db, err := satellitedb.NewInMemory()
	// defer func() {
	// 	err = db.Close()
	// 	assert.NoError(b, err)
	// }()
	// err = db.CreateTables()
	// assert.NoError(b, err)

	const N = 25
	nodes := []*pb.Node{}
	segs := []*pb.InjuredSegment{}
	//fill a pointerdb
	for i := 0; i < N; i++ {
		s := strconv.Itoa(i)
		ids := teststorj.NodeIDsFromStrings([]string{s + "a", s + "b", s + "c", s + "d"}...)

		pointer := &pb.Pointer{
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					RepairThreshold: int32(2),
				},
				PieceId: strconv.Itoa(i),
				RemotePieces: []*pb.RemotePiece{
					{PieceNum: 0, NodeId: ids[0]},
					{PieceNum: 1, NodeId: ids[1]},
					{PieceNum: 2, NodeId: ids[2]},
					{PieceNum: 3, NodeId: ids[3]},
				},
			},
		}

		err := pointerdb.Put(pointer.Remote.PieceId, pointer)
		assert.NoError(b, err)

		//nodes for cache
		selection := rand.Intn(4)
		for _, v := range ids[:selection] {
			n := &pb.Node{Id: v, Type: pb.NodeType_STORAGE, Address: &pb.NodeAddress{Address: ""}}
			nodes = append(nodes, n)
		}
		pieces := []int32{0, 1, 2, 3}
		//expected injured segments
		if len(ids[:selection]) < int(pointer.Remote.Redundancy.RepairThreshold) {
			seg := &pb.InjuredSegment{
				Path:       pointer.Remote.PieceId,
				LostPieces: pieces[selection:],
			}
			segs = append(segs, seg)
		}
	}
	//fill a overlay cache
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker := planet.Satellites[0].Repair.Checker
		assert.NoError(b, err)

		err = checker.IdentifyInjuredSegments(tctx)
		assert.NoError(b, err)

		//check if the expected segments were added to the queue
		dequeued := []*pb.InjuredSegment{}
		for i := 0; i < len(segs); i++ {
			injSeg, err := repairQueue.Dequeue(tctx)
			assert.NoError(b, err)
			dequeued = append(dequeued, &injSeg)
		}
		sort.Slice(segs, func(i, k int) bool { return segs[i].Path < segs[k].Path })
		sort.Slice(dequeued, func(i, k int) bool { return dequeued[i].Path < dequeued[k].Path })

		for i := 0; i < len(segs); i++ {
			assert.True(b, pb.Equal(segs[i], dequeued[i]))
		}
	}
}
