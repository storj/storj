// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparabledb

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/irreparabledb/dbx"
	pb "storj.io/storj/pkg/irreparabledb/proto"
	"storj.io/storj/pkg/pointerdb/auth"
)

var (
	mon = monkit.Package()
)

// Server implements the statdb RPC service
type Server struct {
	DB     *dbx.DB
	logger *zap.Logger
}

// NewServer creates instance of Server
func NewServer(driver, source string, logger *zap.Logger) (*Server, error) {
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}

	err = migrate.Create("irreparabledb", db)
	if err != nil {
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

// Create a db entry for the provided remote segment info
func (s *Server) Create(ctx context.Context, createReq *pb.CreateRequest) (resp *pb.CreateResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering irreparabledb Create")

	APIKeyBytes := createReq.APIKey
	if err := s.validateAuth(APIKeyBytes); err != nil {
		return nil, err
	}

	info := createReq.Rmtseginfo
	_, err = s.DB.Create_Irreparabledb(
		ctx,
		dbx.Irreparabledb_Segmentkey(info.RmtSegKey),
		dbx.Irreparabledb_Segmentval(info.RmtSegVal),
		dbx.Irreparabledb_PiecesLostCount(info.RmtSegLostPiecesCount),
		dbx.Irreparabledb_SegDamagedUnixSec(info.RmtSegRepairUnixSec),
		dbx.Irreparabledb_RepairAttemptCount(info.RmtSegRepairAttemptCount),
	)
	if err != nil {
		return &pb.CreateResponse{
			Status: pb.CreateResponse_FAIL,
		}, status.Errorf(codes.Internal, err.Error())
	}

	s.logger.Debug("created in the db: " + string(info.RmtSegKey))
	return &pb.CreateResponse{
		Status: pb.CreateResponse_OK,
	}, nil
}

// Get a irreparable's segment info from the db
func (s *Server) Get(ctx context.Context, getReq *pb.GetRequest) (resp *pb.GetResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering irreparabaledb Get")

	APIKeyBytes := getReq.APIKey
	err = s.validateAuth(APIKeyBytes)
	if err != nil {
		return nil, err
	}

	dbSegInfo, err := s.DB.Get_Irreparabledb_By_Segmentkey(ctx, dbx.Irreparabledb_Segmentkey(getReq.GetRmtSegKey()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	rmtseginfo := &pb.RmtSegInfo{
		RmtSegKey:                dbSegInfo.Segmentkey,
		RmtSegVal:                dbSegInfo.Segmentval,
		RmtSegLostPiecesCount:    dbSegInfo.PiecesLostCount,
		RmtSegRepairUnixSec:      dbSegInfo.SegDamagedUnixSec,
		RmtSegRepairAttemptCount: dbSegInfo.RepairAttemptCount,
	}
	return &pb.GetResponse{
		Rmtseginfo: rmtseginfo,
		Status:     pb.GetResponse_OK,
	}, nil
}

// Delete a irreparable's segment info from the db
func (s *Server) Delete(ctx context.Context, delReq *pb.DeleteRequest) (resp *pb.DeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering irreparabaledb Delete")

	APIKeyBytes := delReq.APIKey
	err = s.validateAuth(APIKeyBytes)
	if err != nil {
		return nil, err
	}

	dbSegInfo, err := s.DB.Delete_Irreparabledb_By_Segmentkey(ctx, dbx.Irreparabledb_Segmentkey(delReq.GetRmtSegKey()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.DeleteResponse{
		Status: pb.DeleteResponse_OK,
	}, nil
}
