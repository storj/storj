// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage/teststore"
)

var ctx = context.Background()

func TestIdentifyInjuredSegments(t *testing.T) {
	//fill a pointerdb with segments to check
	//fill a overlay cache with some nodes
	//mock a repair threshold
	//some segments: missing pieces surpass repair threshold
	//other segments: missing pieces do not surpass repair threshold
	//check if the expected segments were added to the queue
}

func BenchmarkIdentifyInjuredSegments(b *testing.B) {

}

func TestOfflineNodes(t *testing.T) {
	params := &pb.IdentifyRequest{Recurse: true}
	pointerdb := teststore.New()
	repairQueue := queue.NewQueue(teststore.New())
	logger := zap.NewNop()
	const N = 50
	nodes := []*pb.Node{}
	nodeIDs := []dht.NodeID{}
	expectedIndices := []int32{}
	for i := 0; i < N; i++ {
		str := strconv.Itoa(i)
		n := &pb.Node{Id: str, Address: &pb.NodeAddress{Address: str}}
		nodes = append(nodes, n)
		if i%(rand.Intn(5)+2) == 0 {
			id := kademlia.StringToNodeID("id" + str)
			nodeIDs = append(nodeIDs, id)
			expectedIndices = append(expectedIndices, int32(i))
		} else {
			id := kademlia.StringToNodeID(str)
			nodeIDs = append(nodeIDs, id)
		}
	}
	overlayServer := overlay.NewMockOverlay(nodes)
	checker := NewChecker(params, pointerdb, repairQueue, overlayServer, logger)
	indices, err := checker.offlineNodes(ctx, nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expectedIndices, indices)
}
