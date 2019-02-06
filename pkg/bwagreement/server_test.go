// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gtank/cryptopasta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/bwagreement/testbwagreement"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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

func getPeerContext(ctx context.Context, t *testing.T) (context.Context, storj.NodeID) {
	ident, err := testidentity.NewTestIdentity(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, ident) {
		t.Fatal(err)
	}
	grpcPeer := &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5},
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		},
	}
	nodeID, err := identity.NodeIDFromKey(ident.CA.PublicKey)
	assert.NoError(t, err)
	return peer.NewContext(ctx, grpcPeer), nodeID
}

func testDatabase(ctx context.Context, t *testing.T, bwdb bwagreement.DB) {
	upID, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	satID, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	satellite := bwagreement.NewServer(bwdb, zap.NewNop(), satID.ID)

	{ // TestSameSerialNumberBandwidthAgreements
		pbaFile1, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, satID, upID, time.Hour)
		assert.NoError(t, err)

		ctxSN1, storageNode1 := getPeerContext(ctx, t)
		rbaNode1, err := testbwagreement.GenerateRenterBandwidthAllocation(pbaFile1, storageNode1, upID, 666)
		assert.NoError(t, err)

		ctxSN2, storageNode2 := getPeerContext(ctx, t)
		rbaNode2, err := testbwagreement.GenerateRenterBandwidthAllocation(pbaFile1, storageNode2, upID, 666)
		assert.NoError(t, err)

		/* More than one storage node can submit bwagreements with the same serial number.
		   Uplink would like to download a file from 2 storage nodes.
		   Uplink requests a PayerBandwidthAllocation from the satellite. One serial number for all storage nodes.
		   Uplink signes 2 RenterBandwidthAllocation for both storage node. */
		{
			reply, err := satellite.BandwidthAgreements(ctxSN1, rbaNode1)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)

			reply, err = satellite.BandwidthAgreements(ctxSN2, rbaNode2)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)
		}

		/* Storage node can submit a second bwagreement with a different sequence value.
		   Uplink downloads another file. New PayerBandwidthAllocation with a new sequence. */
		{
			pbaFile2, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, satID, upID, time.Hour)
			assert.NoError(t, err)

			rbaNode1, err := testbwagreement.GenerateRenterBandwidthAllocation(pbaFile2, storageNode1, upID, 666)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctxSN1, rbaNode1)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)
		}

		/* Storage nodes can't submit a second bwagreement with the same sequence. */
		{
			rbaNode1, err := testbwagreement.GenerateRenterBandwidthAllocation(pbaFile1, storageNode1, upID, 666)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctxSN1, rbaNode1)
			assert.True(t, auth.ErrSerial.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage nodes can't submit the same bwagreement twice.
		   This test is kind of duplicate cause it will most likely trigger the same sequence error.
		   For safety we will try it anyway to make sure nothing strange will happen */
		{
			reply, err := satellite.BandwidthAgreements(ctxSN2, rbaNode2)
			assert.True(t, auth.ErrSerial.Has(err))
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}

	{ // TestExpiredBandwidthAgreements
		{ // storage nodes can submit a bwagreement that will expire in 30 seconds
			pba, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, satID, upID, 30*time.Second)
			assert.NoError(t, err)

			ctxSN1, storageNode1 := getPeerContext(ctx, t)
			rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, storageNode1, upID, 666)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctxSN1, rba)
			assert.NoError(t, err)
			assert.Equal(t, pb.AgreementsSummary_OK, reply.Status)
		}

		{ // storage nodes can't submit a bwagreement that expires right now
			pba, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, satID, upID, 0*time.Second)
			assert.NoError(t, err)

			ctxSN1, storageNode1 := getPeerContext(ctx, t)
			rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, storageNode1, upID, 666)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctxSN1, rba)
			assert.Error(t, err)
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		{ // storage nodes can't submit a bwagreement that expires yesterday
			pba, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, satID, upID, -23*time.Hour-55*time.Second)
			assert.NoError(t, err)

			ctxSN1, storageNode1 := getPeerContext(ctx, t)
			rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, storageNode1, upID, 666)
			assert.NoError(t, err)

			reply, err := satellite.BandwidthAgreements(ctxSN1, rba)
			assert.Error(t, err)
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}

	{ // TestManipulatedBandwidthAgreements
		pba, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, satID, upID, time.Hour)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}

		ctxSN1, storageNode1 := getPeerContext(ctx, t)
		rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, storageNode1, upID, 666)
		assert.NoError(t, err)

		// Storage node manipulates the bwagreement
		rba.Total = 1337

		// Generate a new keypair for self signing bwagreements
		manipID, err := testidentity.NewTestIdentity(ctx)
		assert.NoError(t, err)
		manipCerts := manipID.ChainRaw()
		manipPrivKey, ok := manipID.Key.(*ecdsa.PrivateKey)
		assert.True(t, ok)

		/* Storage node can't manipulate the bwagreement size (or any other field)
		   Satellite will verify Renter's Signature. */
		{
			manipRBA := *rba
			// Using uplink signature
			reply, err := callBWA(ctxSN1, t, satellite, rba.GetSignature(), &manipRBA, rba.GetCerts())
			assert.True(t, auth.ErrVerify.Has(err) && pb.ErrRenter.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't sign the manipulated bwagreement
		   Satellite will verify Renter's Signature. */
		{
			manipRBA := *rba
			manipSignature := GetSignature(t, &manipRBA, manipPrivKey)
			assert.NoError(t, err)
			// Using self created signature
			reply, err := callBWA(ctxSN1, t, satellite, manipSignature, rba, rba.GetCerts())
			assert.True(t, auth.ErrVerify.Has(err) && pb.ErrRenter.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't replace uplink Certs
		   Satellite will check uplink Certs against uplinkeId. */
		{
			manipRBA := *rba
			manipSignature := GetSignature(t, &manipRBA, manipPrivKey)
			// Using self created signature + public key
			reply, err := callBWA(ctxSN1, t, satellite, manipSignature, &manipRBA, manipCerts)
			assert.True(t, auth.ErrSigner.Has(err) && pb.ErrRenter.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't replace uplink NodeId
		   Satellite will verify the Payer's Signature. */
		{
			manipRBA := *rba
			// Overwrite the uplinkId with our own keypair
			manipRBA.PayerAllocation.UplinkId = manipID.ID
			manipSignature := GetSignature(t, &manipRBA, manipPrivKey)
			// Using self created signature + public key
			reply, err := callBWA(ctxSN1, t, satellite, manipSignature, &manipRBA, manipCerts)
			assert.True(t, auth.ErrVerify.Has(err) && pb.ErrPayer.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't self sign the PayerBandwidthAllocation.
		   Satellite will verify the Payer's Signature. */
		{
			manipRBA := *rba
			// Overwrite the uplinkId with our own keypair
			manipRBA.PayerAllocation.UplinkId = manipID.ID
			manipSignature := GetSignature(t, &manipRBA.PayerAllocation, manipPrivKey)
			manipRBA.PayerAllocation.Signature = manipSignature
			manipSignature = GetSignature(t, &manipRBA, manipPrivKey)
			// Using self created Payer and Renter bwagreement signatures
			reply, err := callBWA(ctxSN1, t, satellite, manipSignature, &manipRBA, manipCerts)
			assert.True(t, auth.ErrVerify.Has(err) && pb.ErrPayer.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't replace the satellite Certs.
		   Satellite will check satellite certs against satelliteId. */
		{
			manipRBA := *rba
			// Overwrite the uplinkId with our own keypair
			manipRBA.PayerAllocation.UplinkId = manipID.ID
			manipSignature := GetSignature(t, &manipRBA.PayerAllocation, manipPrivKey)
			manipRBA.PayerAllocation.Signature = manipSignature
			manipRBA.PayerAllocation.Certs = manipCerts
			manipSignature = GetSignature(t, &manipRBA, manipPrivKey)
			// Using self created Payer and Renter bwagreement signatures
			reply, err := callBWA(ctxSN1, t, satellite, manipSignature, &manipRBA, manipCerts)
			assert.True(t, auth.ErrSigner.Has(err) && pb.ErrPayer.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		/* Storage node can't replace the satellite.
		   Satellite will verify the Satellite Id. */
		{
			manipRBA := *rba
			// Overwrite the uplinkId and satelliteID with our own keypair
			manipRBA.PayerAllocation.UplinkId = manipID.ID
			manipRBA.PayerAllocation.SatelliteId = manipID.ID
			manipSignature := GetSignature(t, &manipRBA.PayerAllocation, manipPrivKey)
			manipRBA.PayerAllocation.Signature = manipSignature
			manipRBA.PayerAllocation.Certs = manipCerts
			manipSignature = GetSignature(t, &manipRBA, manipPrivKey)
			// Using self created Payer and Renter bwagreement signatures
			reply, err := callBWA(ctxSN1, t, satellite, manipSignature, &manipRBA, manipCerts)
			assert.True(t, pb.ErrPayer.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}

	{ //TestInvalidBandwidthAgreements
		ctxSN1, storageNode1 := getPeerContext(ctx, t)
		ctxSN2, storageNode2 := getPeerContext(ctx, t)
		pba, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, satID, upID, time.Hour)
		assert.NoError(t, err)

		{ // Storage node sends an corrupted signuature to force a satellite crash
			rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, storageNode1, upID, 666)
			assert.NoError(t, err)
			rba.Signature = []byte("invalid")
			reply, err := satellite.BandwidthAgreements(ctxSN1, rba)
			assert.Error(t, err)
			assert.True(t, pb.ErrRenter.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}

		{ // Storage node sends an corrupted uplink Certs to force a crash
			rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, storageNode2, upID, 666)
			assert.NoError(t, err)
			rba.PayerAllocation.Certs = nil
			reply, err := callBWA(ctxSN2, t, satellite, rba.GetSignature(), rba, rba.GetCerts())
			assert.True(t, auth.ErrVerify.Has(err) && pb.ErrRenter.Has(err), err.Error())
			assert.Equal(t, pb.AgreementsSummary_REJECTED, reply.Status)
		}
	}
}

func callBWA(ctx context.Context, t *testing.T, sat *bwagreement.Server, signature []byte, rba *pb.RenterBandwidthAllocation, certs [][]byte) (*pb.AgreementsSummary, error) {
	rba.SetCerts(certs)
	rba.SetSignature(signature)
	return sat.BandwidthAgreements(ctx, rba)
}

//GetSignature returns the signature of the signed message
func GetSignature(t *testing.T, msg auth.SignableMessage, privECDSA *ecdsa.PrivateKey) []byte {
	require.NotNil(t, msg)
	oldSignature := msg.GetSignature()
	certs := msg.GetCerts()
	msg.SetSignature(nil)
	msg.SetCerts(nil)
	msgBytes, err := proto.Marshal(msg)
	require.NoError(t, err)
	signature, err := cryptopasta.Sign(msgBytes, privECDSA)
	require.NoError(t, err)
	msg.SetSignature(oldSignature)
	msg.SetCerts(certs)
	return signature
}
