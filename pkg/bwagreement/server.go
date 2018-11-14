// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"sync"

	"github.com/golang/protobuf/proto"
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
	dbm    *DBManager
	pkey   crypto.PublicKey
	logger *zap.Logger
}

// DBManager is an implementation of the database access interface
type DBManager struct {
	DB     *dbx.DB
	mu     sync.Mutex
	logger *zap.Logger
}

// Agreement is a struct that contains a bandwidth agreement and the associated signature
type Agreement struct {
	Agreement []byte
	Signature []byte
}

// NewDBManager creates a new instance of a DatabaseManager
func NewDBManager(driver, source string, logger *zap.Logger) (*DBManager, error) {
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}

	err = migrate.Create("bwagreement", db)
	if err != nil {
		return nil, err
	}
	return &DBManager{
		DB:     db,
		logger: logger,
	}, nil
}

// NewServer creates instance of Server
func NewServer(driver, source string, logger *zap.Logger, pkey crypto.PublicKey) (*Server, error) {
	dbm, err := NewDBManager(driver, source, logger)
	if err != nil {
		return nil, err
	}

	return &Server{
		dbm:    dbm,
		logger: logger,
		pkey:   pkey,
	}, nil
}

func (dbm *DBManager) locked() func() {
	dbm.mu.Lock()
	return dbm.mu.Unlock
}

// Create a db entry for the provided storagenode
func (dbm *DBManager) Create(ctx context.Context, createBwAgreement *pb.RenterBandwidthAllocation) (bwagreement *dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	dbm.logger.Debug("entering bwagreement Create")

	signature := createBwAgreement.GetSignature()
	data := createBwAgreement.GetData()

	bwagreement, err = dbm.DB.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Signature(signature),
		dbx.Bwagreement_Data(data),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return bwagreement, nil
}

// GetBandwidthAllocations all bandwidth agreements and sorts by satellite
func (dbm *DBManager) GetBandwidthAllocations(ctx context.Context) (rows []*dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	defer dbm.locked()()
	rows, err = dbm.DB.All_Bwagreement(ctx)
	return rows, err
}

// BandwidthAgreements receives and stores bandwidth agreements from storage nodes
func (s *Server) BandwidthAgreements(stream pb.Bandwidth_BandwidthAgreementsServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)
	defer s.dbm.locked()()

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
			if err = s.verifySignature(ctx, agreement); err != nil {
				return err
			}
			_, err = s.dbm.Create(ctx, agreement)
			if err != nil {
				s.logger.Error("DB entry creation Error", zap.Error(err))
				return err
			}
		}
	}

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

	// verify Renter's (uplink) signature
	if ok := cryptopasta.Verify(ba.GetData(), ba.GetSignature(), k); !ok {
		return BwAgreementError.New("Failed to verify Renter's Signature")
	}

	// deserializing pbad you get satelliteID, uplinkID, max size, exp, serial# & action
	pba := &pb.PayerBandwidthAllocation{}
	if err := proto.Unmarshal(ba.GetData(), pba); err != nil {
		return err
	}

	k, ok = s.pkey.(*ecdsa.PublicKey)
	if !ok {
		return peertls.ErrUnsupportedKey.New("%T", s.pkey)
	}

	// verify Payer's (satellite) signature
	if ok := cryptopasta.Verify(pba.GetData(), pba.GetSignature(), k); !ok {
		return BwAgreementError.New("Failed to verify Payer's Signature")
	}
	return nil
}
