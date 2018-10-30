// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"
	"database/sql"
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

// FindValidNodes finds a subset of storagenodes that meet reputation requirements
func (s *Server) FindValidNodes(ctx context.Context, getReq *pb.FindValidNodesRequest) (resp *pb.FindValidNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb FindValidNodes")

	passedIds := [][]byte{}
	passedMap := make(map[string]bool)
	failedIds := [][]byte{}

	nodeIds := getReq.NodeIds
	minAuditCount := getReq.MinStats.AuditCount
	minAuditSuccess := getReq.MinStats.AuditSuccessRatio
	minUptime := getReq.MinStats.UptimeRatio

	rows, err := s.findValidNodesQuery(nodeIds, minAuditCount, minAuditSuccess, minUptime)

	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			s.logger.Error(err.Error())
		}
	}()

	for rows.Next() {
		node := &dbx.Node{}
		err = rows.Scan(&node.Id, &node.TotalAuditCount, &node.AuditSuccessRatio, &node.UptimeRatio, &node.CreatedAt)
		if err != nil {
			return nil, err
		}
		passedIds = append(passedIds, []byte(node.Id))
		passedMap[node.Id] = true
	}

	for _, id := range nodeIds {
		if !passedMap[string(id)] {
			failedIds = append(failedIds, id)
		}
	}

	return &pb.FindValidNodesResponse{
		PassedIds: passedIds,
		FailedIds: failedIds,
	}, nil
}

func (s *Server) findValidNodesQuery(nodeIds [][]byte, auditCount int64, auditSuccess, uptime float64) (*sql.Rows, error) {
	args := make([]interface{}, len(nodeIds))
	for i, id := range nodeIds {
		args[i] = string(id)
	}
	args = append(args, auditCount, auditSuccess, uptime)

	rows, err := s.DB.Query(`SELECT nodes.id, nodes.total_audit_count, 
		nodes.audit_success_ratio, nodes.uptime_ratio, nodes.created_at
		FROM nodes
		WHERE nodes.id IN (?`+strings.Repeat(", ?", len(nodeIds)-1)+`)
		AND nodes.total_audit_count >= ?
		AND nodes.audit_success_ratio >= ?
		AND nodes.uptime_ratio >= ?`, args...)

	return rows, err
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

	node := updateReq.GetNode()

	createIfReq := &pb.CreateEntryIfNotExistsRequest{
		Node:   updateReq.GetNode(),
		APIKey: APIKeyBytes,
	}

	_, err = s.CreateEntryIfNotExists(ctx, createIfReq)
	if err != nil {
		return nil, err
	}

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

// UpdateBatch for updating multiple farmers' stats in the db
func (s *Server) UpdateBatch(ctx context.Context, updateBatchReq *pb.UpdateBatchRequest) (resp *pb.UpdateBatchResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb UpdateBatch")

	APIKeyBytes := updateBatchReq.APIKey
	var nodeStatsList []*pb.NodeStats
	var failedNodes []*pb.Node
	for _, node := range updateBatchReq.NodeList {
		updateReq := &pb.UpdateRequest{
			Node:   node,
			APIKey: APIKeyBytes,
		}

		updateRes, err := s.Update(ctx, updateReq)
		if err != nil {
			s.logger.Error(err.Error())
			failedNodes = append(failedNodes, node)
		} else {
			nodeStatsList = append(nodeStatsList, updateRes.Stats)
		}
	}

	updateBatchRes := &pb.UpdateBatchResponse{
		FailedNodes: failedNodes,
		StatsList:   nodeStatsList,
	}
	return updateBatchRes, nil
}

// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
func (s *Server) CreateEntryIfNotExists(ctx context.Context, createIfReq *pb.CreateEntryIfNotExistsRequest) (resp *pb.CreateEntryIfNotExistsResponse, err error) {
	APIKeyBytes := createIfReq.APIKey
	getReq := &pb.GetRequest{
		NodeId: createIfReq.Node.NodeId,
		APIKey: APIKeyBytes,
	}
	getRes, err := s.Get(ctx, getReq)
	if err != nil {
		// TODO: figure out how to confirm error is type dbx.ErrorCode_NoRows
		if strings.Contains(err.Error(), "no rows in result set") {
			createReq := &pb.CreateRequest{
				Node:   createIfReq.Node,
				APIKey: APIKeyBytes,
			}
			res, err := s.Create(ctx, createReq)
			if err != nil {
				return nil, err
			}
			createEntryIfNotExistsRes := &pb.CreateEntryIfNotExistsResponse{
				Stats: res.Stats,
			}
			return createEntryIfNotExistsRes, nil
		}
		return nil, err
	}
	createEntryIfNotExistsRes := &pb.CreateEntryIfNotExistsResponse{
		Stats: getRes.Stats,
	}
	return createEntryIfNotExistsRes, nil
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
