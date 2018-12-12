// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"crypto/ecdsa"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	testidentity "storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
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

func TestCalculate(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	logger := zap.NewNop()
	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, logger, pointerdb.Config{}, nil)
	limit := 0
	interval := time.Second
	nodes := []*pb.Node{}
	nodeIDs := storj.NodeIDList{}
	expectedNodeData := make(map[string]int64)
	const N = 50

	for i := 0; i < N; i++ {
		nodeID := teststorj.NodeIDFromString(strconv.Itoa(i))
		n := &pb.Node{Id: nodeID, Type: pb.NodeType_STORAGE, Address: &pb.NodeAddress{Address: ""}}
		nodes = append(nodes, n)
		if i%(rand.Intn(5)+2) == 0 {
			//offline nodes
			id := teststorj.NodeIDFromString("id" + nodeID.String())
			nodeIDs = append(nodeIDs, id)
			expectedNodeData[id.String()] = 0
		} else {
			//online nodes
			nodeIDs = append(nodeIDs, nodeID)
			expectedNodeData[nodeID.String()] = 5
		}
	}
	overlayServer := mocks.NewOverlay(nodes)

	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(accountingDb.Close)
	

	masterDB, err := satellitedb.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(masterDB.Close)

	tally := newTally(logger, accountingDb, masterDB.BandwidthAgreement(), pointerdb, overlayServer, limit, interval)

	pointer := &pb.Pointer{
		Remote:      &pb.RemoteSegment{Redundancy: &pb.RedundancyScheme{MinReq: 2}},
		SegmentSize: 10,
	}

	nodeData, err := tally.calculate(ctx, pointer, nodeIDs)
	assert.NoError(t, err)
	assert.Equal(t, expectedNodeData, nodeData)
}

func TestUpdateRawTable(t *testing.T) {
	//TODO
}

func TestQueryNoAgreements(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})

	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(accountingDb.Close)
	

	masterDB, err := satellitedb.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(masterDB.Close)
	

	tally := newTally(zap.NewNop(), accountingDb, masterDB.BandwidthAgreement(), pointerdb, overlayServer, 0, time.Second)

	err = tally.Query(ctx)
	assert.NoError(t, err)
}

func TestQueryWithBw(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})

	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(accountingDb.Close)

	masterDB, err := satellitedb.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(masterDB.Close)
	err = masterDB.CreateTables()
	assert.NoError(t, err)

	bwDb := masterDB.BandwidthAgreement()
	tally := newTally(zap.NewNop(), accountingDb, bwDb, pointerdb, overlayServer, 0, time.Second)

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
