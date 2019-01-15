// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/gtank/cryptopasta"
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

func TestBandwidthAgreement(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testDatabase(ctx, t, db.BandwidthAgreement())
	})
}

func testDatabase(ctx context.Context, t *testing.T, bwdb bwagreement.DB) {
	satellitePubKey, satellitePrivKey, uplinkPrivKey := generateKeys(ctx, t)
	satellite := bwagreement.NewServer(bwdb, zap.NewNop(), satellitePubKey)

	{ // TestSameSerialNumberBandwidthAgreements
		pbaFile1, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey, "1h")
		assert.NoError(t, err)

		rbaNode1, err := GenerateRenterBandwidthAllocation(pbaFile1, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
		assert.NoError(t, err)

		rbaNode2, err := GenerateRenterBandwidthAllocation(pbaFile1, teststorj.NodeIDFromString("Storage node 2"), uplinkPrivKey)
		assert.NoError(t, err)

		/* More than one storage node can submit bwagreements with the same serial number.
		   Uplink would like to download a file from 2 storage nodes.
		   Uplink requests a PayerBandwidthAllocation from the satellite. One serial number for all storage nodes.
		   Uplink signes 2 RenterBandwidthAllocation for both storage node. */
		{
			reply, err := satellite.BandwidthAgreements(ctx, rbaNode1)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)

			reply, err = satellite.BandwidthAgreements(ctx, rbaNode2)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)
		}

		/* Storage node can submit a second bwagreement with a different sequence value.
		   Uplink downloads another file. New PayerBandwidthAllocation with a new sequence. */
		{
			pbaFile2, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey, "1h")
			assert.NoError(t, err)

			rbaNode1, err := GenerateRenterBandwidthAllocation(pbaFile2, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctx, rbaNode1)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)
		}

		/* Storage nodes can't submit a second bwagreement with the same sequence. */
		{
			rbaNode1, err := GenerateRenterBandwidthAllocation(pbaFile1, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctx, rbaNode1)
			assert.EqualError(t, err, "bwagreement error: SerialNumber already exists in the PayerBandwidthAllocation")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage nodes can't submit the same bwagreement twice.
		   This test is kind of duplicate cause it will most likely trigger the same sequence error.
		   For safety we will try it anyway to make sure nothing strange will happen */
		{
			reply, err := satellite.BandwidthAgreements(ctx, rbaNode2)
			assert.EqualError(t, err, "bwagreement error: SerialNumber already exists in the PayerBandwidthAllocation")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}

	{ // TestExpiredBandwidthAgreements
		{ // storage nodes can submit a bwagreement that will expire in one second
			pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey, "1s")
			assert.NoError(t, err)

			rba, err := GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctx, rba)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)
		}

		{ // storage nodes can't submit a bwagreement that expires right now
			pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey, "0s")
			assert.NoError(t, err)

			rba, err := GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctx, rba)
			assert.Error(t, err)
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		{ // storage nodes can't submit a bwagreement that expires yesterday
			pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey, "-23h55s")
			assert.NoError(t, err)

			rba, err := GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctx, rba)
			assert.Error(t, err)
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}

	{ // TestManipulatedBandwidthAgreements
		pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey, "1h")
		assert.NoError(t, err)

		rba, err := GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
		assert.NoError(t, err)

		// Unmarschal Renter and Payer bwagreements
		rbaData := &pb.RenterBandwidthAllocation_Data{}
		err = proto.Unmarshal(rba.GetData(), rbaData)
		assert.NoError(t, err)

		pbaData := &pb.PayerBandwidthAllocation_Data{}
		err = proto.Unmarshal(pba.GetData(), pbaData)
		assert.NoError(t, err)

		// Storage node manipulates the bwagreement
		rbaData.Total = 1337

		// Marschal the manipulated bwagreement
		maniprba, err := proto.Marshal(rbaData)
		assert.NoError(t, err)

		// Generate a new keypair for self signing bwagreements
		_, manipPrivKey, _ := generateKeys(ctx, t)
		manipPubKey, err := getUplinkPubKey(manipPrivKey)
		assert.NoError(t, err)

		/* Storage node can't manipulate the bwagreement size (or any other field)
		   Satellite will verify Renter's Signature */
		{
			// Using uplink signature
			reply, err := satellite.BandwidthAgreements(ctx, &pb.RenterBandwidthAllocation{
				Signature: rba.GetSignature(),
				Data:      maniprba,
			})

			assert.EqualError(t, err, "bwagreement error: Failed to verify Renter's Signature")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't sign the manipulated bwagreement
		   Satellite will verify Renter's Signature */
		{
			manipSignature, err := cryptopasta.Sign(maniprba, manipPrivKey)
			assert.NoError(t, err)

			// Using self created signature
			reply, err := satellite.BandwidthAgreements(ctx, &pb.RenterBandwidthAllocation{
				Signature: manipSignature,
				Data:      maniprba,
			})

			assert.EqualError(t, err, "bwagreement error: Failed to verify Renter's Signature")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't replace uplink PubKey
		   Satellite will verify Payer's Signature */
		{
			// Overwrite the uplink public key with our own keypair
			pbaData.PubKey = manipPubKey

			manippba, err := proto.Marshal(pbaData)
			assert.NoError(t, err)

			rbaData.PayerAllocation = &pb.PayerBandwidthAllocation{
				Signature: pba.GetSignature(),
				Data:      manippba,
			}

			maniprba, err := proto.Marshal(rbaData)
			assert.NoError(t, err)

			manipSignature, err := cryptopasta.Sign(maniprba, manipPrivKey)
			assert.NoError(t, err)

			// Using self created signature + public key
			reply, err := satellite.BandwidthAgreements(ctx, &pb.RenterBandwidthAllocation{
				Signature: manipSignature,
				Data:      maniprba,
			})

			assert.EqualError(t, err, "bwagreement error: Failed to verify Payer's Signature")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't self sign the PayerBandwidthAllocation.
		   Satellite will verify the Payer's Signature with his own public key. */
		{
			// Overwrite the uplink public key with our own keypair
			pbaData.PubKey = manipPubKey

			manippba, err := proto.Marshal(pbaData)
			assert.NoError(t, err)

			manipSignature, err := cryptopasta.Sign(manippba, manipPrivKey)
			assert.NoError(t, err)

			rbaData.PayerAllocation = &pb.PayerBandwidthAllocation{
				Signature: manipSignature,
				Data:      manippba,
			}

			maniprba, err := proto.Marshal(rbaData)
			assert.NoError(t, err)

			manipSignature, err = cryptopasta.Sign(maniprba, manipPrivKey)
			assert.NoError(t, err)

			// Using self created Payer and Renter bwagreement signatures
			reply, err := satellite.BandwidthAgreements(ctx, &pb.RenterBandwidthAllocation{
				Signature: manipSignature,
				Data:      maniprba,
			})

			assert.EqualError(t, err, "bwagreement error: Failed to verify Payer's Signature")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}

	{ //TestInvalidBandwidthAgreements
		pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satellitePrivKey, uplinkPrivKey, "1h")
		assert.NoError(t, err)

		{ // Storage node sends an corrupted signuature to force a satellite crash
			rba, err := GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("Storage node 1"), uplinkPrivKey)
			assert.NoError(t, err)

			rba.Signature = []byte("invalid")

			reply, err := satellite.BandwidthAgreements(ctx, rba)
			assert.EqualError(t, err, "bwagreement error: Invalid Renter's Signature Length")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		{ // Storage node sends an corrupted uplink pubkey to force a crash
			rba, err := GenerateRenterBandwidthAllocation(pba, teststorj.NodeIDFromString("Storage node 2"), uplinkPrivKey)
			assert.NoError(t, err)

			rbaData := &pb.RenterBandwidthAllocation_Data{}
			err = proto.Unmarshal(rba.GetData(), rbaData)
			assert.NoError(t, err)

			pbaData := &pb.PayerBandwidthAllocation_Data{}
			err = proto.Unmarshal(pba.GetData(), pbaData)
			assert.NoError(t, err)

			pbaData.PubKey = nil

			invalidpba, err := proto.Marshal(pbaData)
			assert.NoError(t, err)

			rbaData.PayerAllocation = &pb.PayerBandwidthAllocation{
				Signature: pba.GetSignature(),
				Data:      invalidpba,
			}

			invalidrba, err := proto.Marshal(rbaData)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctx, &pb.RenterBandwidthAllocation{
				Signature: rba.GetSignature(),
				Data:      invalidrba,
			})
			assert.EqualError(t, err, "bwagreement error: Failed to extract Public Key from RenterBandwidthAllocation: asn1: syntax error: sequence truncated")
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}
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
