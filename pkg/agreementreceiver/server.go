// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbx "storj.io/storj/pkg/agreementreceiver/dbx"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/auth"
)

// Server is an implementation of the pb.BandwidthServer interface
type Server struct {
	DB *dbx.DB
	//identity *provider.FullIdentity
	logger *zap.Logger
}

// NewServer creates instance of Server
func NewServer(driver, source string, logger *zap.Logger) (*Server, error) {
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(db.Schema())
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}

	return &Server{
		DB:     db,
		logger: logger,
	}, nil
}

func (s *Server) validateAuth(APIKeyBytes []byte) error {
	if !auth.ValidateAPIKey(string(APIKeyBytes)) {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
}

// Create a db entry for the provided storagenode
func (s *Server) Create(ctx context.Context, createBwAgreement *pb.RenterBandwidthAllocation) (bwagreement *dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb Create")

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
	s.logger.Debug("created an signature/data entry in the db")

	return bwagreement, nil
}

// BandwidthAgreements receives and stores bandwidth agreements from storage nodes
func (s *Server) BandwidthAgreements(stream pb.Bandwidth_BandwidthAgreementsServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	s.logger.Debug("Received the bw agreement msg from piecenode ")
	ch := make(chan *pb.RenterBandwidthAllocation, 1)
	go func() error {
		for {
			msg, err := stream.Recv()
			if err != nil {
				return err
			}
			ch <- msg
		}
	}()

	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("ctx<-Done()")
			return nil
		case agreement := <-ch:
			go func() (err error) {
				s.logger.Debug("about to create a postgres entry")
				_, err = s.Create(ctx, agreement)
				return err
			}()
		}
	}
	return err
}
