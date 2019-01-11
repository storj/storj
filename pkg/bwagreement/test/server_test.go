// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"context"
	"crypto/ecdsa"
	"testing"

	//"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSameSerialNumberBandwidthAgreements(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		/* More than one storage node can submit bwagreements with the same serial number.
		   Uplink would like to download a file from 2 storage nodes.
		   Uplink requests a PayerBandwidthAllocation from the satellite. One serial number for all storage nodes.
		   Uplink signes 2 RenterBandwidthAllocation for both storage node. */
		satellitePubKey, satellitePrivKey, uplinkPrivKey := generateKeys(ctx, t)
		server := bwagreement.NewServer(db.BandwidthAgreement(), zap.NewNop(), satellitePubKey)

		pbaFile1, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey)
		assert.NoError(t, err)

		rbaNode1, err := GenerateRenterBandwidthAllocation(pbaFile1, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
		assert.NoError(t, err)

		rbaNode2, err := GenerateRenterBandwidthAllocation(pbaFile1, teststorj.NodeIDFromString("Storage node 2"), uplinkPrivKey)
		assert.NoError(t, err)

		reply, err := server.BandwidthAgreements(ctx, rbaNode1)
		assert.NoError(t, err)
		assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)

		reply, err = server.BandwidthAgreements(ctx, rbaNode2)
		assert.NoError(t, err)
		assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)

		/* Storage node can submit a second bwagreement with a different sequence value.
		   Uplink downloads another file. New PayerBandwidthAllocation with a new sequence. */
		pbaFile2, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey)
		assert.NoError(t, err)

		rbaNode1, err = GenerateRenterBandwidthAllocation(pbaFile2, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
		assert.NoError(t, err)

		reply, err = server.BandwidthAgreements(ctx, rbaNode1)
		assert.NoError(t, err)
		assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)

		/* Storage nodes can't submit a second bwagreement with the same sequence. */
		rbaNode1, err = GenerateRenterBandwidthAllocation(pbaFile1, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
		assert.NoError(t, err)

		//TODO: return custom error message in the event of "UNIQUE constraint failed..."
		reply, err = server.BandwidthAgreements(ctx, rbaNode1)
		assert.EqualError(t, err, "bwagreement error: SerialNumber already exist in the PayerBandwidthAllocation")
		assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)

		/* Storage nodes can't submit the same bwagreement twice.
		   This test is kind of duplicate cause it will most likely trigger the same sequence error.
		   For safety we will try it anyway to make sure nothing strange will happen */
		reply, err = server.BandwidthAgreements(ctx, rbaNode2)
		assert.EqualError(t, err, "bwagreement error: SerialNumber already exist in the PayerBandwidthAllocation")
		assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
	})
}

func TestInvalidBandwidthAgreements(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		/* Todo: Add more tests for bwagreement manipulations

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		satellitePubKey, satellitePrivKey, uplinkPrivKey := generateKeys(ctx, t)
		server := bwagreement.NewServer(db.BandwidthAgreement(), zap.NewNop(), satellitePubKey)

		pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey)
		assert.NoError(t, err)

		rba, err := GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
		assert.NoError(t, err)

		Make sure the bwagreement we are using as bluleprint is valid and avoid false positives that way.
		reply, err := server.BandwidthAgreements(ctx, rba)
		assert.NoError(t, err)
		assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)
		*/

		/* copy and unmarshal pba and rba to manipulate it without overwriting it */

		/* manipulate PayerBandwidthAllocation -> invalid signature */

		/* self signed. Storage node sends a self signed bwagreement to get a higher payout */

		/* malicious storage node would like to force a crash */

		/* corrupted signature. Storage node sends an corrupted signuature to force a satellite crash */

	})
}

func generateKeys(ctx context.Context, t *testing.T) (satellitePubKey *ecdsa.PublicKey, satellitePrivKey *ecdsa.PrivateKey, uplinkPrivKey *ecdsa.PrivateKey) {
	fiS, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)

	satellitePubKey, ok := fiS.Leaf.PublicKey.(*ecdsa.PublicKey)
	assert.True(t, ok)

	satellitePrivKey, ok = fiS.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)

	fiU, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)

	uplinkPrivKey, ok = fiU.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	return
}
