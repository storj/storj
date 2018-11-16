// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/teststore"
)

var ctx = context.Background()

func TestIdentifyInjuredSegments(t *testing.T) {
	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	repairQueue := queue.NewQueue(teststore.New())

	const N = 25
	nodes := []storj.Node{}
	segs := []*pb.InjuredSegment{}
	//fill a pointerdb
	for i := 0; i < N; i++ {
		s := strconv.Itoa(i)
		idStrings := []string{s + "a", s + "b", s + "c", s + "d"}
		var nodeIDs storj.NodeIDList
		for _, s := range idStrings {
			nodeIDs = append(nodeIDs, teststorj.NodeIDFromString(s))
		}

		p := &pb.Pointer{
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					RepairThreshold: int32(2),
				},
				PieceId: strconv.Itoa(i),
				RemotePieces: []*pb.RemotePiece{
					{PieceNum: 0, NodeId: nodeIDs[0].Bytes()},
					{PieceNum: 1, NodeId: nodeIDs[1].Bytes()},
					{PieceNum: 2, NodeId: nodeIDs[2].Bytes()},
					{PieceNum: 3, NodeId: nodeIDs[3].Bytes()},
				},
			},
		}
		req := &pb.PutRequest{
			Path:    p.Remote.PieceId,
			Pointer: p,
		}
		ctx = auth.WithAPIKey(ctx, nil)
		resp, err := pointerdb.Put(ctx, req)
		assert.NotNil(t, resp)
		assert.NoError(t, err)

		//nodes for cache
		selection := rand.Intn(4)
		for _, v := range nodeIDs[:selection] {
			n := storj.NewNodeWithID(v, &pb.Node{Address: &pb.NodeAddress{Address: ""}})
			nodes = append(nodes, n)
		}
		pieces := []int32{0, 1, 2, 3}
		//expected injured segments
		if len(nodeIDs[:selection]) < int(p.Remote.Redundancy.RepairThreshold) {
			seg := &pb.InjuredSegment{
				Path:       p.Remote.PieceId,
				LostPieces: pieces[selection:],
			}
			segs = append(segs, seg)
		}
	}
	//fill a overlay cache
	overlayServer := mocks.NewOverlay(nodes)
	limit := 0
	interval := time.Second
	checker := newChecker(pointerdb, repairQueue, overlayServer, limit, logger, interval)
	err := checker.IdentifyInjuredSegments(ctx)
	assert.NoError(t, err)

	//check if the expected segments were added to the queue
	dequeued := []*pb.InjuredSegment{}
	for i := 0; i < len(segs); i++ {
		injSeg, err := repairQueue.Dequeue()
		assert.NoError(t, err)
		dequeued = append(dequeued, &injSeg)
	}
	sort.Slice(segs, func(i, k int) bool { return segs[i].Path < segs[k].Path })
	sort.Slice(dequeued, func(i, k int) bool { return dequeued[i].Path < dequeued[k].Path })

	for i := 0; i < len(segs); i++ {
		assert.True(t, proto.Equal(segs[i], dequeued[i]))
	}
}

func TestOfflineAndOnlineNodes(t *testing.T) {
	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	repairQueue := queue.NewQueue(teststore.New())
	const N = 50
	var (
		nodes           []storj.Node
		nodeIDs         storj.NodeIDList
		expectedOffline []int32
	)
	for i := 0; i < N; i++ {
		nodeID := teststorj.NodeIDFromString(strconv.Itoa(i))
		n := storj.NewNodeWithID(nodeID, &pb.Node{Address: &pb.NodeAddress{Address: ""}})
		nodes = append(nodes, n)
		if i%(rand.Intn(5)+2) == 0 {
			id := teststorj.NodeIDFromString(fmt.Sprintf("offline-%d", i))
			nodeIDs = append(nodeIDs, id)
			expectedOffline = append(expectedOffline, int32(i))
		} else {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	overlayServer := mocks.NewOverlay(nodes)
	limit := 0
	interval := time.Second
	checker := newChecker(pointerdb, repairQueue, overlayServer, limit, logger, interval)
	offline, err := checker.offlineNodes(ctx, nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expectedOffline, offline)
}

func BenchmarkIdentifyInjuredSegments(b *testing.B) {
	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	addr, cleanup, err := redisserver.Start()
	defer cleanup()
	assert.NoError(b, err)
	client, err := redis.NewClient(addr, "", 1)
	assert.NoError(b, err)
	repairQueue := queue.NewQueue(client)

	const N = 25
	nodes := []storj.Node{}
	segs := []*pb.InjuredSegment{}
	//fill a pointerdb
	for i := 0; i < N; i++ {
		s := strconv.Itoa(i)
		idStrings := []string{s + "a", s + "b", s + "c", s + "d"}
		var nodeIDs storj.NodeIDList
		for _, s := range idStrings {
			nodeIDs = append(nodeIDs, teststorj.NodeIDFromString(s))
		}

		p := &pb.Pointer{
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					RepairThreshold: int32(2),
				},
				PieceId: strconv.Itoa(i),
				RemotePieces: []*pb.RemotePiece{
					{PieceNum: 0, NodeId: nodeIDs[0].Bytes()},
					{PieceNum: 1, NodeId: nodeIDs[1].Bytes()},
					{PieceNum: 2, NodeId: nodeIDs[2].Bytes()},
					{PieceNum: 3, NodeId: nodeIDs[3].Bytes()},
				},
			},
		}
		req := &pb.PutRequest{
			Path:    p.Remote.PieceId,
			Pointer: p,
		}
		ctx = auth.WithAPIKey(ctx, nil)
		resp, err := pointerdb.Put(ctx, req)
		assert.NotNil(b, resp)
		assert.NoError(b, err)

		//nodes for cache
		selection := rand.Intn(4)
		for _, nodeID := range nodeIDs[:selection] {
			n := storj.NewNodeWithID(nodeID, &pb.Node{Address: &pb.NodeAddress{Address: ""}})
			nodes = append(nodes, n)
		}
		pieces := []int32{0, 1, 2, 3}
		//expected injured segments
		if len(nodeIDs[:selection]) < int(p.Remote.Redundancy.RepairThreshold) {
			seg := &pb.InjuredSegment{
				Path:       p.Remote.PieceId,
				LostPieces: pieces[selection:],
			}
			segs = append(segs, seg)
		}
	}
	//fill a overlay cache
	overlayServer := mocks.NewOverlay(nodes)
	limit := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interval := time.Second
		checker := newChecker(pointerdb, repairQueue, overlayServer, limit, logger, interval)
		err = checker.IdentifyInjuredSegments(ctx)
		assert.NoError(b, err)

		//check if the expected segments were added to the queue
		dequeued := []*pb.InjuredSegment{}
		for i := 0; i < len(segs); i++ {
			injSeg, err := repairQueue.Dequeue()
			assert.NoError(b, err)
			dequeued = append(dequeued, &injSeg)
		}
		sort.Slice(segs, func(i, k int) bool { return segs[i].Path < segs[k].Path })
		sort.Slice(dequeued, func(i, k int) bool { return dequeued[i].Path < dequeued[k].Path })

		for i := 0; i < len(segs); i++ {
			assert.True(b, proto.Equal(segs[i], dequeued[i]))
		}
	}
}
