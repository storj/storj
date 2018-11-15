// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"

	"github.com/golang/protobuf/proto"
	"github.com/gtank/cryptopasta"
	"go.uber.org/zap"

	"storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
)

// Server is an implementation of the pb.BandwidthServer interface
type Server struct {
	dbm    *dbmanager.DBManager
	pkey   crypto.PublicKey
	logger *zap.Logger
}

// Agreement is a struct that contains a bandwidth agreement and the associated signature
type Agreement struct {
	Agreement []byte
	Signature []byte
}

// NewServer creates instance of Server
func NewServer(dbm *dbmanager.DBManager, logger *zap.Logger, pkey crypto.PublicKey) (*Server, error) {
	return &Server{
		dbm:    dbm,
		logger: logger,
		pkey:   pkey,
	}, nil
}

// BandwidthAgreements receives and stores bandwidth agreements from storage nodes
func (s *Server) BandwidthAgreements(ctx context.Context, agreement *pb.RenterBandwidthAllocation) (reply *pb.AgreementsSummary, err error) {
	defer mon.Task()(&ctx)(&err)

	s.logger.Debug("Received Agreement...")

	reply = &pb.AgreementsSummary{
		Status: pb.AgreementsSummary_FAIL,
	}

	if err = s.verifySignature(ctx, agreement); err != nil {
		return reply, err
	}

	_, err = s.dbm.Create(ctx, agreement)
	if err != nil {
		return reply, err
	}

	reply.Status = pb.AgreementsSummary_OK

	s.logger.Debug("Stored Agreement...")

	return reply, nil
}

func (s *Server) verifySignature(ctx context.Context, ba *pb.RenterBandwidthAllocation) error {
	// TODO(security): detect replay attacks

	//Deserealize RenterBandwidthAllocation.GetData() so we can get public key
	rbad := &pb.RenterBandwidthAllocation_Data{}
	if err := proto.Unmarshal(ba.GetData(), rbad); err != nil {
		return BwAgreementError.New("Failed to unmarshal RenterBandwidthAllocation: %+v", err)
	}

	// Extract renter's public key from RenterBandwidthAllocation_Data
	pubkey, err := x509.ParsePKIXPublicKey(rbad.GetPubKey())
	if err != nil {
		return BwAgreementError.New("Failed to extract Public Key from RenterBandwidthAllocation: %+v", err)
	}

	// Typecast public key
	k, ok := pubkey.(*ecdsa.PublicKey)
	if !ok {
		return peertls.ErrUnsupportedKey.New("%T", pubkey)
	}

	// verify Renter's (uplink) signature
	if ok := cryptopasta.Verify(ba.GetData(), ba.GetSignature(), k); !ok {
		return BwAgreementError.New("Failed to verify Renter's Signature")
	}

	k, ok = s.pkey.(*ecdsa.PublicKey)
	if !ok {
		return peertls.ErrUnsupportedKey.New("%T", s.pkey)
	}

	// verify Payer's (satellite) signature
	if ok := cryptopasta.Verify(rbad.GetPayerAllocation().GetData(), rbad.GetPayerAllocation().GetSignature(), k); !ok {
		return BwAgreementError.New("Failed to verify Payer's Signature")
	}
	return nil
}
