// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/fork/crypto"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
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

//UplinkStat contains information about an uplink's returned Orders
type UplinkStat struct {
	NodeID            storj.NodeID
	TotalBytes        int64
	PutActionCount    int
	GetActionCount    int
	TotalTransactions int
}

//SavedOrder is information from an Order pertaining to accounting
type SavedOrder struct {
	Serialnum     string
	StorageNodeID storj.NodeID
	UplinkID      storj.NodeID
	Action        int64
	Total         int64
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// DB stores orders for accounting purposes
type DB interface {
	// SaveOrder saves an order for accounting
	SaveOrder(context.Context, *pb.Order) error
	// GetTotalsSince returns the sum of each bandwidth type after (exluding) a given date range
	GetTotals(context.Context, time.Time, time.Time) (map[storj.NodeID][]int64, error)
	//GetTotals returns stats about an uplink
	GetUplinkStats(context.Context, time.Time, time.Time) ([]UplinkStat, error)
	//GetExpired gets orders that are expired and were created before some time
	GetExpired(context.Context, time.Time, time.Time) ([]SavedOrder, error)
	//DeleteExpired deletes orders that are expired and were created before some time
	DeleteExpired(context.Context, time.Time, time.Time) error
}

// Server is an implementation of the pb.BandwidthServer interface
type Server struct {
	bwdb   DB
	certdb certdb.DB
	pkey   crypto.PublicKey
	NodeID storj.NodeID
	log    *zap.Logger
}

// NewServer creates instance of Server
func NewServer(db DB, upldb certdb.DB, pkey crypto.PublicKey, log *zap.Logger, nodeID storj.NodeID) *Server {
	// TODO: reorder arguments
	return &Server{bwdb: db, certdb: upldb, pkey: pkey, log: log, NodeID: nodeID}
}

// Close closes resources
func (s *Server) Close() error { return nil }

// BandwidthAgreements receives and stores bandwidth agreements from storage nodes
func (s *Server) BandwidthAgreements(ctx context.Context, rba *pb.Order) (reply *pb.AgreementsSummary, err error) {
	defer mon.Task()(&ctx)(&err)
	s.log.Debug("Received Agreement...")
	reply = &pb.AgreementsSummary{
		Status: pb.AgreementsSummary_REJECTED,
	}
	pba := rba.PayerAllocation
	//verify message content
	pi, err := identity.PeerIdentityFromContext(ctx)
	if err != nil || rba.StorageNodeId != pi.ID {
		return reply, auth.ErrBadID.New("Storage Node ID: %v vs %v", rba.StorageNodeId, pi.ID)
	}
	//todo:  use whitelist for uplinks?
	if pba.SatelliteId != s.NodeID {
		return reply, pb.ErrPayer.New("Satellite ID: %v vs %v", pba.SatelliteId, s.NodeID)
	}
	exp := time.Unix(pba.GetExpirationUnixSec(), 0).UTC()
	if exp.Before(time.Now().UTC()) {
		return reply, pb.ErrPayer.Wrap(auth.ErrExpired.New("%v vs %v", exp, time.Now().UTC()))
	}

	if err = s.verifySignature(ctx, rba); err != nil {
		return reply, err
	}

	//save and return rersults
	if err = s.bwdb.SaveOrder(ctx, rba); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "violates unique constraint") {
			return reply, pb.ErrPayer.Wrap(auth.ErrSerial.Wrap(err))
		}
		reply.Status = pb.AgreementsSummary_FAIL
		return reply, pb.ErrPayer.Wrap(err)
	}
	reply.Status = pb.AgreementsSummary_OK
	s.log.Debug("Stored Agreement...")
	return reply, nil
}

// Settlement receives and handles agreements.
func (s *Server) Settlement(client pb.Bandwidth_SettlementServer) (err error) {
	ctx := client.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	formatError := func(err error) error {
		if err == io.EOF {
			return nil
		}
		return status.Error(codes.Unknown, err.Error())
	}

	s.log.Debug("Settlement", zap.Any("storage node ID", peer.ID))
	for {
		request, err := client.Recv()
		if err != nil {
			return formatError(err)
		}

		if request == nil || request.Allocation == nil {
			return status.Error(codes.InvalidArgument, "allocation missing")
		}
		allocation := request.Allocation
		payerAllocation := allocation.PayerAllocation

		if allocation.StorageNodeId != peer.ID {
			return status.Error(codes.Unauthenticated, "only specified storage node can settle allocation")
		}

		allocationExpiration := time.Unix(payerAllocation.GetExpirationUnixSec(), 0)
		if allocationExpiration.Before(time.Now()) {
			s.log.Debug("allocation expired", zap.String("serial", payerAllocation.SerialNumber), zap.Error(err))
			err := client.Send(&pb.BandwidthSettlementResponse{
				SerialNumber: payerAllocation.SerialNumber,
				Status:       pb.AgreementsSummary_REJECTED,
			})
			if err != nil {
				return formatError(err)
			}
		}

		if err = s.verifySignature(ctx, allocation); err != nil {
			s.log.Debug("signature verification failed", zap.String("serial", payerAllocation.SerialNumber), zap.Error(err))
			err := client.Send(&pb.BandwidthSettlementResponse{
				SerialNumber: payerAllocation.SerialNumber,
				Status:       pb.AgreementsSummary_REJECTED,
			})
			if err != nil {
				return formatError(err)
			}
		}

		if err = s.bwdb.SaveOrder(ctx, allocation); err != nil {
			s.log.Debug("saving order failed", zap.String("serial", payerAllocation.SerialNumber), zap.Error(err))
			duplicateRequest := strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "violates unique constraint")
			if duplicateRequest {
				err := client.Send(&pb.BandwidthSettlementResponse{
					SerialNumber: payerAllocation.SerialNumber,
					Status:       pb.AgreementsSummary_REJECTED,
				})
				if err != nil {
					return formatError(err)
				}
			}
		}

		err = client.Send(&pb.BandwidthSettlementResponse{
			SerialNumber: payerAllocation.SerialNumber,
			Status:       pb.AgreementsSummary_OK,
		})
		if err != nil {
			return formatError(err)
		}
	}
}

func (s *Server) verifySignature(ctx context.Context, rba *pb.Order) error {
	pba := rba.GetPayerAllocation()

	// Get renter's public key from uplink agreement db
	uplinkInfo, err := s.certdb.GetPublicKey(ctx, pba.UplinkId)
	if err != nil {
		return pb.ErrRenter.Wrap(auth.ErrVerify.New("Failed to unmarshal OrderLimit: %+v", err))
	}

	// verify Renter's (uplink) signature
	rbad := *rba
	rbad.SetSignature(nil)
	rbad.SetCerts(nil)
	rbadBytes, err := proto.Marshal(&rbad)
	if err != nil {
		return Error.New("marshalling error: %+v", err)
	}

	if err := pkcrypto.HashAndVerifySignature(uplinkInfo, rbadBytes, rba.GetSignature()); err != nil {
		return pb.ErrRenter.Wrap(auth.ErrVerify.Wrap(err))
	}

	// verify Payer's (satellite) signature
	pbad := pba
	pbad.SetSignature(nil)
	pbad.SetCerts(nil)
	pbadBytes, err := proto.Marshal(&pbad)
	if err != nil {
		return Error.New("marshalling error: %+v", err)
	}

	if err := pkcrypto.HashAndVerifySignature(s.pkey, pbadBytes, pba.GetSignature()); err != nil {
		return pb.ErrPayer.Wrap(auth.ErrVerify.Wrap(err))
	}
	return nil
}
