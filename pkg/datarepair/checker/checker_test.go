// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

var ctx = context.Background()

func TestIdentifyInjuredSegments(t *testing.T) {
	params := &pb.IdentifyRequest{Recurse: true}
	pointerdb := teststore.New()
	repairQueue := queue.NewQueue(teststore.New())
	logger := zap.NewNop()
	const N = 25
	nodes := []*pb.Node{}
	segs := []*pb.InjuredSegment{}
	//fill a pointerdb
	for i := 0; i < N; i++ {
		s := strconv.Itoa(i)
		ids := []string{s + "a", s + "b", s + "c", s + "d"}

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
		val, err := proto.Marshal(p)
		assert.NoError(t, err)
		err = pointerdb.Put(storage.Key(p.Remote.PieceId), val)
		assert.NoError(t, err)

		//nodes for cache
		selection := rand.Intn(4)
		for _, v := range ids[:selection] {
			n := &pb.Node{Id: v, Address: &pb.NodeAddress{Address: v}}
			nodes = append(nodes, n)
		}
		pieces := []int32{0, 1, 2, 3}
		//expected injured segments
		if len(ids[:selection]) < int(p.Remote.Redundancy.RepairThreshold) {
			seg := &pb.InjuredSegment{
				Path:          p.Remote.PieceId,
				LostPieces:    pieces[selection:],
				HealthyPieces: pieces[:selection],
			}
			segs = append(segs, seg)
		}
	}
	//fill a overlay cache
	overlayServer := overlay.NewMockOverlay(nodes)
	checker := NewChecker(params, pointerdb, repairQueue, overlayServer, logger)
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
	params := &pb.IdentifyRequest{Recurse: true}
	pointerdb := teststore.New()
	repairQueue := queue.NewQueue(teststore.New())
	logger := zap.NewNop()
	const N = 50
	nodes := []*pb.Node{}
	nodeIDs := []dht.NodeID{}
	expectedOffline := []int32{}
	expectedOnline := []int32{}
	for i := 0; i < N; i++ {
		str := strconv.Itoa(i)
		n := &pb.Node{Id: str, Address: &pb.NodeAddress{Address: str}}
		nodes = append(nodes, n)
		if i%(rand.Intn(5)+2) == 0 {
			id := kademlia.StringToNodeID("id" + str)
			nodeIDs = append(nodeIDs, id)
			expectedOffline = append(expectedOffline, int32(i))
		} else {
			id := kademlia.StringToNodeID(str)
			nodeIDs = append(nodeIDs, id)
			expectedOnline = append(expectedOnline, int32(i))
		}
	}
	overlayServer := overlay.NewMockOverlay(nodes)
	checker := NewChecker(params, pointerdb, repairQueue, overlayServer, logger)
	offline, online, err := checker.offlineAndOnlineNodes(ctx, nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expectedOffline, offline)
	assert.Equal(t, expectedOnline, online)
}
