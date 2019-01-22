// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
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
	"storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storage/teststore"
)

func TestQueryNoAgreements(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	service := pointerdb.NewService(zap.NewNop(), teststore.New())
	overlayServer := mocks.NewOverlay([]*pb.Node{})
	db, err := satellitedb.NewInMemory()
	assert.NoError(t, err)
	defer ctx.Check(db.Close)
	assert.NoError(t, db.CreateTables())

	tally := newTally(zap.NewNop(), db.Accounting(), db.BandwidthAgreement(), service, overlayServer, 0, time.Second)

	err = tally.queryBW(ctx)
	assert.NoError(t, err)
}

func TestQueryWithBw(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	service := pointerdb.NewService(zap.NewNop(), teststore.New())
	overlayServer := mocks.NewOverlay([]*pb.Node{})

	db, err := satellitedb.NewInMemory()
	assert.NoError(t, err)
	defer ctx.Check(db.Close)

	assert.NoError(t, db.CreateTables())

	bwDb := db.BandwidthAgreement()
	tally := newTally(zap.NewNop(), db.Accounting(), bwDb, service, overlayServer, 0, time.Second)

	//get a private key
	fiC, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	k, ok := fiC.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)

	makeBWA(ctx, t, bwDb, "1", k, pb.PayerBandwidthAllocation_PUT)
	makeBWA(ctx, t, bwDb, "2", k, pb.PayerBandwidthAllocation_GET)
	makeBWA(ctx, t, bwDb, "3", k, pb.PayerBandwidthAllocation_GET_AUDIT)
	makeBWA(ctx, t, bwDb, "4", k, pb.PayerBandwidthAllocation_GET_REPAIR)
	makeBWA(ctx, t, bwDb, "5", k, pb.PayerBandwidthAllocation_PUT_REPAIR)

	//check the db
	err = tally.queryBW(ctx)
	assert.NoError(t, err)
}

func makeBWA(ctx context.Context, t *testing.T, bwDb bwagreement.DB, serialNum string, k *ecdsa.PrivateKey, action pb.PayerBandwidthAllocation_Action) {
	//generate an agreement with the key
	pba, err := test.GeneratePayerBandwidthAllocation(action, k, k, time.Hour)
	assert.NoError(t, err)
	rba, err := test.GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("StorageNodeID"), k)
	assert.NoError(t, err)
	//save to db
	err = bwDb.CreateAgreement(ctx, serialNum, bwagreement.Agreement{Signature: rba.GetSignature(), Agreement: rba.GetData()})
	assert.NoError(t, err)
}
