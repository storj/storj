// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	testidentity "storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/bwagreement/test"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storage/teststore"
)

func TestQueryNoAgreements(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})

	accountingDb, err := accounting.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)
	defer ctx.Check(accountingDb.Close)
	

	masterDB, err := satellitedb.NewInMemory()
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

	masterDB, err := satellitedb.NewInMemory()
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
