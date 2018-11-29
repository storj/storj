// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/storage/teststore"
)

var ctx = context.Background()

func TestIdentifyActiveNodes(t *testing.T) {

}
func TestOnlineNodes(t *testing.T) {
	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	const N = 50
	nodes := []*pb.Node{}
	nodeIDs := []dht.NodeID{}
	expectedOnline := []*pb.Node{}
	for i := 0; i < N; i++ {
		str := strconv.Itoa(i)
		n := &pb.Node{Id: str, Type: pb.NodeType_STORAGE, Address: &pb.NodeAddress{Address: str}}
		nodes = append(nodes, n)
		if i%(rand.Intn(5)+2) == 0 {
			id := node.IDFromString("id" + str)
			nodeIDs = append(nodeIDs, id)
		} else {
			id := node.IDFromString(str)
			nodeIDs = append(nodeIDs, id)
			expectedOnline = append(expectedOnline, n)
		}
	}
	overlayServer := mocks.NewOverlay(nodes)
	kad := &kademlia.Kademlia{}
	limit := 0
	interval := time.Second

	accountingDb, err := accounting.NewDb("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()
	tally, err := newTally(logger, accountingDb, pointerdb, overlayServer, kad, limit, interval)
	assert.NoError(t, err)
	online, err := tally.onlineNodes(ctx, nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expectedOnline, online)
}

func TestTallyAtRestStorage(t *testing.T) {

}

func TestNeedToContact(t *testing.T) {

}

func TestUpdateGranularTable(t *testing.T) {

}
