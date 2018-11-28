// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/internal/storj"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/testqueue"
	"storj.io/storj/storage/teststore"
)

var ctx = context.Background()

func TestIdentifyInjuredSegments(t *testing.T) {
	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	repairQueue := queue.NewQueue(testqueue.New())

	const N = 25
	nodes := []*pb.Node{}
	segs := []*pb.InjuredSegment{}
	//fill a pointerdb
	for i := 0; i < N; i++ {
		s := strconv.Itoa(i)
		ids := teststorj.NodeIDsFromStrings([]string{s + "a", s + "b", s + "c", s + "d"}...)

		p := &pb.Pointer{
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
		for _, v := range ids[:selection] {
			n := &pb.Node{Id: v, Address: &pb.NodeAddress{Address: ""}}
			nodes = append(nodes, n)
		}
		pieces := []int32{0, 1, 2, 3}
		//expected injured segments
		if len(ids[:selection]) < int(p.Remote.Redundancy.RepairThreshold) {
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
	err := checker.identifyInjuredSegments(ctx)
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

func TestOfflineNodes(t *testing.T) {
	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	repairQueue := queue.NewQueue(testqueue.New())
	const N = 50
	nodes := []*pb.Node{}
	nodeIDs := storj.NodeIDList{}
	expectedOffline := []int32{}
	for i := 0; i < N; i++ {
		id := teststorj.NodeIDFromString(strconv.Itoa(i))
		n := &pb.Node{Id: id, Address: &pb.NodeAddress{Address: ""}}
		nodes = append(nodes, n)
		if i%(rand.Intn(5)+2) == 0 {
			nodeIDs = append(nodeIDs, teststorj.NodeIDFromString("id" + id.String()))
			expectedOffline = append(expectedOffline, int32(i))
		} else {
			nodeIDs = append(nodeIDs, id)
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
	client, err := redis.NewQueue(addr, "", 1)
	assert.NoError(b, err)
	repairQueue := queue.NewQueue(client)

	const N = 25
	nodes := []*pb.Node{}
	segs := []*pb.InjuredSegment{}
	//fill a pointerdb
	for i := 0; i < N; i++ {
		s := strconv.Itoa(i)
		ids := teststorj.NodeIDsFromStrings([]string{s + "a", s + "b", s + "c", s + "d"}...)

		p := &pb.Pointer{
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
		for _, v := range ids[:selection] {
			n := &pb.Node{Id: v, Address: &pb.NodeAddress{Address: ""}}
			nodes = append(nodes, n)
		}
		pieces := []int32{0, 1, 2, 3}
		//expected injured segments
		if len(ids[:selection]) < int(p.Remote.Redundancy.RepairThreshold) {
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
		err = checker.identifyInjuredSegments(ctx)
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
