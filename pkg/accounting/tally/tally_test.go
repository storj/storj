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
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	dbManager "storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
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
	nodeIDs := storj.NodeIDList{}
	expectedOnline := []*pb.Node{}
	for i := 0; i < N; i++ {
		nodeID := teststorj.NodeIDFromString(strconv.Itoa(i))
		n := &pb.Node{Id: nodeID, Type: pb.NodeType_STORAGE, Address: &pb.NodeAddress{Address: ""}}
		nodes = append(nodes, n)
		if i%(rand.Intn(5)+2) == 0 {
			id := teststorj.NodeIDFromString("id" + nodeID.String())
			nodeIDs = append(nodeIDs, id)
		} else {
			nodeIDs = append(nodeIDs, nodeID)
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

	bwDb, err := dbManager.NewDBManager("sqlite3", "file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()

	tally := newTally(logger, accountingDb, bwDb, pointerdb, overlayServer, kad, limit, interval)

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

func TestQueryNoAgreements(t *testing.T) {
	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	const N = 50
	nodes := []*pb.Node{}
	overlayServer := mocks.NewOverlay(nodes)
	kad := &kademlia.Kademlia{}
	limit := 0
	interval := time.Second

	accountingDb, err := accounting.NewDb("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()

	bwDb, err := dbManager.NewDBManager("sqlite3", "file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()

	tally := newTally(logger, accountingDb, bwDb, pointerdb, overlayServer, kad, limit, interval)

	err = tally.Query(ctx)
	assert.NoError(t, err)
}

func TestQueryWithBw(t *testing.T) {
	TS := bwagreement.NewTestServer(t)
	defer TS.Stop()

	pba, err := bwagreement.GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, TS.k)
	assert.NoError(t, err)

	rba, err := bwagreement.GenerateRenterBandwidthAllocation(pba, TS.k)
	assert.NoError(t, err)

	/* emulate sending the bwagreement stream from piecestore node */
	_, err = TS.c.BandwidthAgreements(ctx, rba)
	assert.NoError(t, err)
}
