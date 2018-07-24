// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "storj.io/storj/pkg/statdb/proto"
	dbx "storj.io/storj/pkg/statdb/dbx"
	"storj.io/storj/pointerdb/auth"

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
		s.logger.Error("unauthorized request: ", zap.Error(grpc.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return grpc.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
}

// Create a db entry for the provided farmer
func (s *Server) Create(ctx context.Context, createReq *pb.CreateRequest) (*pb.CreateResponse, error) {
	s.logger.Debug("entering statdb Create")

	APIKeyBytes := []byte(createReq.ApiKey)
	if err := s.validateAuth(APIKeyBytes); err != nil {
		return nil, err
	}

	node := createReq.Node

	auditSuccessCount, totalAuditCount, auditSuccessRatio := InitRatioVars(node.UpdateAuditSuccess, node.AuditSuccess)
	uptimeSuccessCount, totalUptimeCount, uptimeRatio := InitRatioVars(node.UpdateUptime, node.IsUp)

	dbNode, err := s.DB.Create_Node(
		ctx,
		dbx.Node_Id(string(node.NodeId)),
		dbx.Node_AuditSuccessCount(auditSuccessCount),
		dbx.Node_TotalAuditCount(totalAuditCount),
		dbx.Node_AuditSuccessRatio(auditSuccessRatio),
		dbx.Node_UptimeSuccessCount(uptimeSuccessCount),
		dbx.Node_TotalUptimeCount(totalUptimeCount),
		dbx.Node_UptimeRatio(uptimeRatio),
	)
	if err != nil {
		s.logger.Error("err creating node stats", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	s.logger.Debug("created in the db: " + string(node.NodeId))

	nodeStats := &pb.NodeStats{
		NodeId:            []byte(dbNode.Id),
		AuditSuccessRatio: dbNode.AuditSuccessRatio,
		UptimeRatio:       dbNode.UptimeRatio,
	}
	return &pb.CreateResponse{
		Stats: nodeStats,
	}, nil
}

// Get a farmer's stats from the db
func (s *Server) Get(ctx context.Context, getReq *pb.GetRequest) (*pb.GetResponse, error) {
	s.logger.Debug("entering statdb Get")

	APIKeyBytes := []byte(getReq.ApiKey)
	err := s.validateAuth(APIKeyBytes)
	if err != nil {
		return nil, err
	}

	dbNode, err := s.DB.Get_Node_By_Id(ctx, dbx.Node_Id(string(getReq.NodeId)))
	if err != nil {
		s.logger.Error("err getting node stats", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	nodeStats := &pb.NodeStats{
		NodeId:            []byte(dbNode.Id),
		AuditSuccessRatio: dbNode.AuditSuccessRatio,
		UptimeRatio:       dbNode.UptimeRatio,
	}
	return &pb.GetResponse{
		Stats: nodeStats,
	}, nil
}

// Update a single farmer's stats in the db
func (s *Server) Update(ctx context.Context, updateReq *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	s.logger.Debug("entering statdb Update")

	APIKeyBytes := []byte(updateReq.ApiKey)
	err := s.validateAuth(APIKeyBytes)
	if err != nil {
		return nil, err
	}

	node := updateReq.Node

	dbNode, err := s.DB.Get_Node_By_Id(ctx, dbx.Node_Id(string(node.NodeId)))
	if err != nil {
		s.logger.Error("err getting node stats", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	auditSuccessCount := dbNode.AuditSuccessCount
	totalAuditCount := dbNode.TotalAuditCount
	auditSuccessRatio := dbNode.AuditSuccessRatio
	uptimeSuccessCount := dbNode.UptimeSuccessCount
	totalUptimeCount := dbNode.TotalUptimeCount
	uptimeRatio := dbNode.UptimeRatio

	if node.UpdateAuditSuccess {
		auditSuccessCount, totalAuditCount, auditSuccessRatio =	UpdateRatioVars(
			node.AuditSuccess,
			auditSuccessCount,
			totalAuditCount,
		)
	}
	if node.UpdateUptime {
		uptimeSuccessCount, totalUptimeCount, uptimeRatio =	UpdateRatioVars(
			node.IsUp,
			uptimeSuccessCount,
			totalUptimeCount,
		)
	}

	updateFields := dbx.Node_Update_Fields{
		AuditSuccessCount: dbx.Node_AuditSuccessCount(auditSuccessCount),
		TotalAuditCount: dbx.Node_TotalAuditCount(totalAuditCount),
		AuditSuccessRatio: dbx.Node_AuditSuccessRatio(auditSuccessRatio),
		UptimeSuccessCount: dbx.Node_UptimeSuccessCount(uptimeSuccessCount),
		TotalUptimeCount: dbx.Node_TotalUptimeCount(totalUptimeCount),
		UptimeRatio: dbx.Node_UptimeRatio(uptimeRatio),
	}
	dbNode, err = s.DB.Update_Node_By_Id(ctx, dbx.Node_Id(string(node.NodeId)), updateFields)
	if err != nil {
		s.logger.Error("err updating node stats", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	nodeStats := &pb.NodeStats{
		NodeId:            []byte(dbNode.Id),
		AuditSuccessRatio: dbNode.AuditSuccessRatio,
		UptimeRatio:       dbNode.UptimeRatio,
	}
	return &pb.UpdateResponse{
		Stats: nodeStats,
	}, nil
}

// Update multiple farmers' stats in the db
func (s *Server) UpdateBatch(ctx context.Context, updateBatchReq *pb.UpdateBatchRequest) (*pb.UpdateBatchResponse, error) {
	s.logger.Debug("entering statdb UpdateBatch")

	APIKeyBytes := []byte(updateBatchReq.ApiKey)
	err := s.validateAuth(APIKeyBytes)
	if err != nil {
		return nil, err
	}

	// TODO try to implement using similar code as Update
	// Maybe call the same function from both Update and UpdateBatch to do update logic for individual farmers

	return &pb.UpdateBatchResponse{}, nil
}

func InitRatioVars(shouldUpdate, status bool) (int64, int64, float64) {
	var (
		successCount int64 = 0
		totalCount int64 = 0
		ratio float64 = 0.0
	)

	if shouldUpdate {
		return UpdateRatioVars(status, successCount, totalCount)
	}

	return successCount, totalCount, ratio
}

func UpdateRatioVars(newStatus bool, successCount, totalCount int64) (int64, int64, float64) {
	totalCount++
	if newStatus {
		successCount++
	}
	var newRatio float64 = float64(successCount) / float64(totalCount)
	return successCount, totalCount, newRatio
}
