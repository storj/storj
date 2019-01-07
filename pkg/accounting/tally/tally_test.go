// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/teststorj"
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
	db, err := satellitedb.NewInMemory()
	assert.NoError(t, err)
	defer ctx.Check(db.Close)
	assert.NoError(t, db.CreateTables())

	tally := newTally(zap.NewNop(), db.Accounting(), db.BandwidthAgreement(), pointerdb, overlayServer, 0, time.Second)

	err = tally.queryBW(ctx)
	assert.NoError(t, err)
}

func TestQueryWithBw(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pointerdb := pointerdb.NewServer(teststore.New(), &overlay.Cache{}, zap.NewNop(), pointerdb.Config{}, nil)
	overlayServer := mocks.NewOverlay([]*pb.Node{})

	db, err := satellitedb.NewInMemory()
	assert.NoError(t, err)
	defer ctx.Check(db.Close)

	assert.NoError(t, db.CreateTables())

	bwDb := db.BandwidthAgreement()
	tally := newTally(zap.NewNop(), db.Accounting(), bwDb, pointerdb, overlayServer, 0, time.Second)

	//get a private key
	fiC, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	k, ok := fiC.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)

	//generate an agreement with the key
	pba, err := test.GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, k, k)
	assert.NoError(t, err)

	rba, err := test.GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("StorageNodeID"), k)
	assert.NoError(t, err)
	//save to db

	err = bwDb.CreateAgreement(ctx, "SerialNumber", bwagreement.Agreement{Signature: rba.GetSignature(), Agreement: rba.GetData()})
	assert.NoError(t, err)

	//check the db
	err = tally.queryBW(ctx)
	assert.NoError(t, err)
}
