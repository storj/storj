// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"crypto/ecdsa"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"


	testidentity "storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"

	//"storj.io/storj/internal/identity"
	//"storj.io/storj/internal/testcontext"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/bwagreement/test"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storage/teststore"
)

func TestIdentifyActiveNodes(t *testing.T) {
	//TODO
}

func TestCategorizeNodes(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	kad := planet.Satellites[0].Kademlia

	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)

	nodes := []*pb.Node{}
	nodeIDs := storj.NodeIDList{}
	expectedOnline := []*pb.Node{}
	var expectedNodeMap = make(map[string]int64)
	ns := planet.StorageNodes

	for i, n := range ns {
		node := &pb.Node{Id: n.ID(), Type: pb.NodeType_STORAGE, Address: &pb.NodeAddress{Address: n.Addr()}}
		nodes = append(nodes, node)
		if i%(rand.Intn(5)+2) == 0 {
			id := teststorj.NodeIDFromString("id" + n.ID().String())
			nodeIDs = append(nodeIDs, id)
			expectedNodeMap[id.String()] = 0
		} else {
			nodeIDs = append(nodeIDs, n.ID())
			expectedOnline = append(expectedOnline, node)
		}
	}

	overlayServer := mocks.NewOverlay(nodes)
	limit := 0
	interval := time.Second

	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(accountingDb.Close)

	masterDB, err := satellitedb.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	tally, err := newTally(ctx, logger, accountingDb, bwDb, pointerdb, overlayServer, kad, limit, interval)
	assert.NoError(t, err)
	online, nodeMap, err := tally.categorize(ctx, nodeIDs)

	assert.NoError(t, err)
	assert.Equal(t, expectedOnline, online)
	assert.Equal(t, expectedNodeMap, nodeMap)
}

func TestTallyAtRestStorage(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	kad := planet.Satellites[0].Kademlia

	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})
	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()
	bwDb, err := dbManager.NewDBManager("sqlite3", "file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer func() { _ = accountingDb.Close() }()
	tally, err := newTally(ctx, zap.NewNop(), accountingDb, bwDb, pointerdb, overlayServer, kad, 0, time.Second)
	assert.NoError(t, err)

	pointer := &pb.Pointer{
		Remote:      &pb.RemoteSegment{Redundancy: &pb.RedundancyScheme{MinReq: 1}},
		SegmentSize: 5,
	}
	nodes := []*pb.Node{}
	ns := planet.StorageNodes
	nodeMap := make(map[string]int64)
	expectedNodeData := make(map[string]int64)
	for _, n := range ns {
		node := &pb.Node{Id: n.ID(), Type: pb.NodeType_STORAGE, Address: &pb.NodeAddress{Address: n.Addr()}}
		nodes = append(nodes, node)
		expectedNodeData[n.ID().String()] = 5
	}
	nodeData, err := tally.tallyAtRestStorage(ctx, pointer, nodes, nodeMap)
	assert.NoError(t, err)
	assert.Equal(t, expectedNodeData, nodeData)

}

func TestUpdateRawTable(t *testing.T) {
	//TODO
}

func TestQueryNoAgreements(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	kad := planet.Satellites[0].Kademlia

	//get stuff we need
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})
	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(accountingDb.Close)

	masterDB, err := satellitedb.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)

	defer func() { _ = accountingDb.Close() }()
	tally, err := newTally(ctx, zap.NewNop(), accountingDb, bwDb, pointerdb, overlayServer, kad, 0, time.Second)
	assert.NoError(t, err)

	//defer ctx.Check(masterDB.Close)

	//check the db
	err = tally.Query(ctx)
	assert.NoError(t, err)
}

func TestQueryWithBw(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	kad := planet.Satellites[0].Kademlia

	//get stuff we need
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})
	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(accountingDb.Close)

	masterDB, err := satellitedb.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)

	defer func() { _ = accountingDb.Close() }()
	tally, err := newTally(ctx, zap.NewNop(), accountingDb, bwDb, pointerdb, overlayServer, kad, 0, time.Second)
	assert.NoError(t, err)

	//err = masterDB.CreateTables()
	//assert.NoError(t, err)
	//defer ctx.Check(masterDB.Close)

	//bwDb := masterDB.BandwidthAgreement()
	//tally := newTally(zap.NewNop(), accountingDb, bwDb, pointerdb, overlayServer, kad, 0, time.Second)


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
	err = bwDb.CreateAgreement(ctx, bwagreement.Agreement{Signature: rba.GetSignature(), Agreement: rba.GetData()})
	assert.NoError(t, err)

	//check the db
	err = tally.Query(ctx)
	assert.NoError(t, err)
}
