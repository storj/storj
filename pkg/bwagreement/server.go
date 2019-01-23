// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	// Error the default bwagreement errs class
	Error = errs.Class("bwagreement error")
	mon   = monkit.Package()
)

// Config is a configuration struct that is everything you need to start an
// agreement receiver responsibility
type Config struct {
}

// DB stores bandwidth agreements.
type DB interface {
	// CreateAgreement adds a new bandwidth agreement.
	CreateAgreement(context.Context, string, Agreement) error
	// GetAgreements gets all bandwidth agreements.
	GetAgreements(context.Context) ([]Agreement, error)
	// GetAgreementsSince gets all bandwidth agreements since specific time.
	GetAgreementsSince(context.Context, time.Time) ([]Agreement, error)
}

// Server is an implementation of the pb.BandwidthServer interface
type Server struct {
	db     DB
	NodeID storj.NodeID
	logger *zap.Logger
}

// Agreement is a struct that contains a bandwidth agreement and the associated signature
type Agreement struct {
	Agreement []byte
	Signature []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewServer creates instance of Server
func NewServer(db DB, logger *zap.Logger, nodeID storj.NodeID) *Server {
	// TODO: reorder arguments, rename logger -> log
	return &Server{db: db, logger: logger, NodeID: nodeID}
}

// Close closes resources
func (s *Server) Close() error { return nil }

// BandwidthAgreements receives and stores bandwidth agreements from storage nodes
func (s *Server) BandwidthAgreements(ctx context.Context, rba *pb.RenterBandwidthAllocation) (reply *pb.AgreementsSummary, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("Received Agreement...")
	reply = &pb.AgreementsSummary{
		Status: pb.AgreementsSummary_REJECTED,
	}
	rbad, pba, pbad, err := rba.Unpack()
	if err != nil {
		return reply, err
	}
	//verify message content
	pi, err := identity.PeerIdentityFromContext(ctx)
	if err != nil || rbad.StorageNodeId != pi.ID {
		return reply, pb.BadID.New("Storage Node ID: %s vs %s", rbad.StorageNodeId, pi.ID)
	}
	//todo:  use whitelist for uplinks?
	if pbad.SatelliteId != s.NodeID {
		return reply, pb.Payer.New("Satellite ID: %s vs %s", pbad.SatelliteId, s.NodeID)
	}
	serialNum := pbad.GetSerialNumber() + rbad.StorageNodeId.String()
	if len(pbad.SerialNumber) == 0 {
		return reply, pb.Payer.Wrap(pb.Missing.New("Serial"))
	}
	exp := time.Unix(pbad.GetExpirationUnixSec(), 0).UTC()
	if exp.Before(time.Now().UTC()) {
		return reply, pb.Payer.Wrap(pb.Expired.New("%v vs %v", exp, time.Now().UTC()))
	}
	//verify message crypto
	if err := pb.VerifyMsg(rba, pbad.UplinkId); err != nil {
		return reply, pb.Renter.Wrap(err)
	}
	if err := pb.VerifyMsg(pba, pbad.SatelliteId); err != nil {
		return reply, pb.Payer.Wrap(err)
	}
	//save and return rersults
	err = s.db.CreateAgreement(ctx, serialNum, Agreement{
		Signature: rba.GetSignature(),
		Agreement: rba.GetData(),
		ExpiresAt: exp,
	})
	if err != nil {
		//todo:  better classify transport errors (AgreementsSummary_FAIL) vs logical (AgreementsSummary_REJECTED)
		return reply, pb.Payer.Wrap(pb.Serial.Wrap(err))
	}
	reply.Status = pb.AgreementsSummary_OK
	s.logger.Debug("Stored Agreement...")
	return reply, nil
}
