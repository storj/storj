// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
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
	nodeA := &pb.Node{Id: "a", Address: &pb.NodeAddress{Address: "a"}}
	nodeB := &pb.Node{Id: "b", Address: &pb.NodeAddress{Address: "b"}}
	overlayServer := overlay.NewMockOverlay([]*pb.Node{nodeB, nodeA})
	logger := zap.NewNop()
	checker := NewChecker(params, pointerdb, repairQueue, overlayServer, logger)
	idA := kademlia.StringToNodeID("a")
	idB := kademlia.StringToNodeID("b")
	idC := kademlia.StringToNodeID("c")
	idD := kademlia.StringToNodeID("d")
	nodeIDs := []dht.NodeID{idD, idB, idA, idC}
	indices, err := checker.offlineNodes(ctx, nodeIDs)
	expected := []int32{0, 3}
	assert.NoError(t, err)
	assert.Equal(t, expected, indices)
}
