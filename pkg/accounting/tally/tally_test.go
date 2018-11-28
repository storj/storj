// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"crypto/ecdsa"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/provider"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/accounting"
	dbManager "storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/bwagreement/test"
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

func TestCategorize(t *testing.T) {
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
	rootdir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	kad, err := kademlia.NewKademlia(node.IDFromString("foo"), []pb.Node{}, "127.0.0.1:8080", nil, &provider.FullIdentity{}, rootdir, 1)
	assert.NoError(t, err)
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

func TestUpdateRawTable(t *testing.T) {

}

func TestQueryNoAgreements(t *testing.T) {
	//get stuff we need
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})
	kad := &kademlia.Kademlia{}
	accountingDb, err := accounting.NewDb("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()
	bwDb, err := dbManager.NewDBManager("sqlite3", "file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()
	tally := newTally(zap.NewNop(), accountingDb, bwDb, pointerdb, overlayServer, kad, 0, time.Second)

	//check the db
	err = tally.Query(ctx)
	assert.NoError(t, err)
}

func TestQueryWithBw(t *testing.T) {
	//get stuff we need
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})
	kad := &kademlia.Kademlia{}
	accountingDb, err := accounting.NewDb("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()
	bwDb, err := dbManager.NewDBManager("sqlite3", "file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()
	tally := newTally(zap.NewNop(), accountingDb, bwDb, pointerdb, overlayServer, kad, 0, time.Second)

	//get a private key
	fiC, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)
	k, ok := fiC.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	//generate an agreement with the key
	pba, err := test.GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, k)
	assert.NoError(t, err)
	rba, err := test.GenerateRenterBandwidthAllocation(pba, k)
	assert.NoError(t, err)
	//save to db
	_, err = bwDb.Create(ctx, rba)
	assert.NoError(t, err)

	//check the db
	err = tally.Query(ctx)
	assert.NoError(t, err)
}
