// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"crypto/ecdsa"
	"sync"

	"github.com/gtank/cryptopasta"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/bwagreement/dbx"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
)

// OK - Success!
const OK = "OK"

// Server is an implementation of the pb.BandwidthServer interface
type Server struct {
	DB     *dbx.DB
	mu     sync.Mutex
	logger *zap.Logger
}

// Agreement is a struct that contains a bandwidth agreement and the associated signature
type Agreement struct {
	Agreement []byte
	Signature []byte
}

// NewServer creates instance of Server
func NewServer(driver, source string, logger *zap.Logger) (*Server, error) {
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}

	err = migrate.Create("bwagreement", db)
	if err != nil {
		return nil, err
	}

	return &Server{
		DB:     db,
		logger: logger,
	}, nil
}

func (s *Server) locked() func() {
	s.mu.Lock()
	return s.mu.Unlock
}

// Create a db entry for the provided storagenode
func (s *Server) Create(ctx context.Context, createBwAgreement *pb.RenterBandwidthAllocation) (bwagreement *dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering bwagreement Create")

	signature := createBwAgreement.GetSignature()
	data := createBwAgreement.GetData()

	bwagreement, err = s.DB.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Signature(signature),
		dbx.Bwagreement_Data(data),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return bwagreement, nil
}

// BandwidthAgreements receives and stores bandwidth agreements from storage nodes
func (s *Server) BandwidthAgreements(stream pb.Bandwidth_BandwidthAgreementsServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)
	defer s.locked()()

	ch := make(chan *pb.RenterBandwidthAllocation, 1)
	errch := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				s.logger.Error("Grpc Receive Error", zap.Error(err))
				errch <- err
				return
			}
			ch <- msg
		}
	}()

	for {
		select {
		case err := <-errch:
			return err
		case <-ctx.Done():
			return nil
		case agreement := <-ch:
			err = s.verifySignature(ctx, agreement)
			if err != nil {
				return err
			}
			_, err = s.Create(ctx, agreement)
			if err != nil {
				s.logger.Error("DB entry creation Error", zap.Error(err))
				return err
			}
		}
	}

}

// GetBandwidthAllocations all bandwidth agreements and sorts by satellite
func (s *Server) GetBandwidthAllocations(ctx context.Context) (rows []*dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	defer s.locked()()
	rows, err = s.DB.All_Bwagreement(ctx)
	return rows, err
}

func (s *Server) verifySignature(ctx context.Context, ba *pb.RenterBandwidthAllocation) error {
	// TODO(security): detect replay attacks
	pi, err := provider.PeerIdentityFromContext(ctx)
	if err != nil {
		return err
	}

	k, ok := pi.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return peertls.ErrUnsupportedKey.New("%T", pi.Leaf.PublicKey)
	}

	if ok := cryptopasta.Verify(ba.GetData(), ba.GetSignature(), k); !ok {
		return BwAgreementError.New("Failed to verify Signature")
	}
	return nil
}
