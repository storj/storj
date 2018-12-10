// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestBandwidthAgreements(t *testing.T) {
	satellitedbtest.Run(t, func(db *satellitedb.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		satellitePubKey, satellitePrivKey, uplinkPrivKey := generateKeys(ctx, t)
		server := bwagreement.NewServer(db.BandwidthAgreement(), zap.NewNop(), satellitePubKey)

		pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey)
		assert.NoError(t, err)

		rba, err := GenerateRenterBandwidthAllocation(pba, uplinkPrivKey)
		assert.NoError(t, err)

		/* emulate sending the bwagreement stream from piecestore node */
		replay, err := server.BandwidthAgreements(ctx, rba)
		assert.NoError(t, err)
		assert.Equal(t, pb.AgreementsSummary_OK, replay.Status)
	})
}

func generateKeys(ctx context.Context, t *testing.T) (satellitePubKey *ecdsa.PublicKey, satellitePrivKey *ecdsa.PrivateKey, uplinkPrivKey *ecdsa.PrivateKey) {
	fiS, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)

	satellitePubKey, ok := fiS.Leaf.PublicKey.(*ecdsa.PublicKey)
	assert.True(t, ok)

	satellitePrivKey, ok = fiS.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)

	fiU, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)

	uplinkPrivKey, ok = fiU.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	return
}
