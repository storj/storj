// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pointerdb/auth"
	dbx "storj.io/storj/pkg/statdb/dbx"
	pb "storj.io/storj/pkg/statdb/proto"
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
func (s *Server) Create(ctx context.Context, createReq *pb.CreateRequest) (resp *pb.CreateResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb Create")

	APIKeyBytes := createReq.APIKey
	if err := s.validateAuth(APIKeyBytes); err != nil {
		return nil, err
	}

	node := createReq.Node

	auditSuccessCount, totalAuditCount, auditSuccessRatio := initRatioVars(node.UpdateAuditSuccess, node.AuditSuccess)
	uptimeSuccessCount, totalUptimeCount, uptimeRatio := initRatioVars(node.UpdateUptime, node.IsUp)

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

// Get a storagenode's stats from the db
func (s *Server) Get(ctx context.Context, getReq *pb.GetRequest) (resp *pb.GetResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb Get")

	APIKeyBytes := getReq.APIKey
	err = s.validateAuth(APIKeyBytes)
	if err != nil {
		return nil, err
	}

	dbNode, err := s.DB.Get_Node_By_Id(ctx, dbx.Node_Id(string(getReq.NodeId)))
	if err != nil {
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

// Update a single storagenode's stats in the db
func (s *Server) Update(ctx context.Context, updateReq *pb.UpdateRequest) (resp *pb.UpdateResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb Update")

	APIKeyBytes := updateReq.APIKey
	err = s.validateAuth(APIKeyBytes)
	if err != nil {
		return nil, err
	}

	node := updateReq.Node

	dbNode, err := s.DB.Get_Node_By_Id(ctx, dbx.Node_Id(string(node.NodeId)))
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	auditSuccessCount := dbNode.AuditSuccessCount
	totalAuditCount := dbNode.TotalAuditCount
	var auditSuccessRatio float64
	uptimeSuccessCount := dbNode.UptimeSuccessCount
	totalUptimeCount := dbNode.TotalUptimeCount
	var uptimeRatio float64

	updateFields := dbx.Node_Update_Fields{}

	if node.UpdateAuditSuccess {
		auditSuccessCount, totalAuditCount, auditSuccessRatio = updateRatioVars(
			node.AuditSuccess,
			auditSuccessCount,
			totalAuditCount,
		)

		updateFields.AuditSuccessCount = dbx.Node_AuditSuccessCount(auditSuccessCount)
		updateFields.TotalAuditCount = dbx.Node_TotalAuditCount(totalAuditCount)
		updateFields.AuditSuccessRatio = dbx.Node_AuditSuccessRatio(auditSuccessRatio)
	}
	if node.UpdateUptime {
		uptimeSuccessCount, totalUptimeCount, uptimeRatio = updateRatioVars(
			node.IsUp,
			uptimeSuccessCount,
			totalUptimeCount,
		)

		updateFields.UptimeSuccessCount = dbx.Node_UptimeSuccessCount(uptimeSuccessCount)
		updateFields.TotalUptimeCount = dbx.Node_TotalUptimeCount(totalUptimeCount)
		updateFields.UptimeRatio = dbx.Node_UptimeRatio(uptimeRatio)
	}

	dbNode, err = s.DB.Update_Node_By_Id(ctx, dbx.Node_Id(string(node.NodeId)), updateFields)
	if err != nil {
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

// UpdateBatch for updating  multiple farmers' stats in the db
func (s *Server) UpdateBatch(ctx context.Context, updateBatchReq *pb.UpdateBatchRequest) (resp *pb.UpdateBatchResponse, err error) {
	// todo(moby) how should we handle one node failing to update but not all?
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb UpdateBatch")

	APIKeyBytes := updateBatchReq.APIKey
	nodeStatsList := make([]*pb.NodeStats, len(updateBatchReq.NodeList))
	for i, node := range updateBatchReq.NodeList {
		updateReq := &pb.UpdateRequest{
			Node:   node,
			APIKey: APIKeyBytes,
		}

		updateRes, err := s.Update(ctx, updateReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}

		nodeStatsList[i] = updateRes.Stats
	}

	updateBatchRes := &pb.UpdateBatchResponse{
		StatsList: nodeStatsList,
	}
	return updateBatchRes, nil
}

func initRatioVars(shouldUpdate, status bool) (int64, int64, float64) {
	var (
		successCount int64
		totalCount   int64
		ratio        float64
	)

	if shouldUpdate {
		return updateRatioVars(status, successCount, totalCount)
	}

	return successCount, totalCount, ratio
}

func updateRatioVars(newStatus bool, successCount, totalCount int64) (int64, int64, float64) {
	totalCount++
	if newStatus {
		successCount++
	}
	newRatio := float64(successCount) / float64(totalCount)
	return successCount, totalCount, newRatio
}
