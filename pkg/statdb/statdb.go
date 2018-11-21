// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"
	"database/sql"
	"strings"
	"crypto/subtle"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/pkg/auth"
	dbx "storj.io/storj/pkg/statdb/dbx"
	pb "storj.io/storj/pkg/statdb/proto"
)

var (
	mon             = monkit.Package()
	errAuditSuccess = errs.Class("statdb audit success error")
	errUptime       = errs.Class("statdb uptime error")
)

// Server implements the statdb RPC service
type Server struct {
	DB     *dbx.DB
	logger *zap.Logger
	apiKey string
}

// NewServer creates instance of Server
func NewServer(driver, source, apiKey string, logger *zap.Logger) (*Server, error) {
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}

	// TODO(moby) move db creation to "setup" stage. NewServer should only try to open an existing db
	err = migrate.Create("statdb", db)
	if err != nil {
		return nil, err
	}

	return &Server{
		DB:     db,
		logger: logger,
		apiKey: apiKey,
	}, nil
}

func (s *Server) validateAuth(ctx context.Context) error {
	apiKey, ok := auth.GetAPIKey(ctx)
	if !ok {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	expected := []byte(s.apiKey)
	actual := []byte(apiKey)
	matches := (1 == subtle.ConstantTimeCompare(expected, actual))
	if !matches {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
}

// Create a db entry for the provided storagenode
func (s *Server) Create(ctx context.Context, createReq *pb.CreateRequest) (resp *pb.CreateResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb Create")

	if err := s.validateAuth(ctx); err != nil {
		return nil, err
	}

	var (
		totalAuditCount    int64
		auditSuccessCount  int64
		auditSuccessRatio  float64
		totalUptimeCount   int64
		uptimeSuccessCount int64
		uptimeRatio        float64
	)

	stats := createReq.Stats
	if stats != nil {
		totalAuditCount = stats.AuditCount
		auditSuccessCount = stats.AuditSuccessCount
		auditSuccessRatio, err = checkRatioVars(auditSuccessCount, totalAuditCount)
		if err != nil {
			return nil, errAuditSuccess.Wrap(err)
		}

		totalUptimeCount = stats.UptimeCount
		uptimeSuccessCount = stats.UptimeSuccessCount
		uptimeRatio, err = checkRatioVars(uptimeSuccessCount, totalUptimeCount)
		if err != nil {
			return nil, errUptime.Wrap(err)
		}
	}

	node := createReq.Node

	dbNode, err := s.DB.Create_Node(
		ctx,
		dbx.Node_Id(node.NodeId),
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
		NodeId:            dbNode.Id,
		AuditCount:        dbNode.TotalAuditCount,
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

	err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	dbNode, err := s.DB.Get_Node_By_Id(ctx, dbx.Node_Id(getReq.NodeId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	nodeStats := &pb.NodeStats{
		NodeId:            dbNode.Id,
		AuditCount:        dbNode.TotalAuditCount,
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

	// TODO(moby) determine whether we need this functionality here. We are not currently using this

	passedIds := [][]byte{}

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
		passedIds = append(passedIds, node.Id)
	}

	return &pb.FindValidNodesResponse{
		PassedIds: passedIds,
	}, nil
}

func (s *Server) findValidNodesQuery(nodeIds [][]byte, auditCount int64, auditSuccess, uptime float64) (*sql.Rows, error) {
	args := make([]interface{}, len(nodeIds))
	for i, id := range nodeIds {
		args[i] = id
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

	// TODO(moby) this function contains a lot of unnecessary cluttering functionality that we *might* not need:
	//     1. If Update is called and the node does not exist, the node is created. Maybe we just want to return an error and not create the node
	//     2. We allow the caller to configure whether each field is updated. We can make these decisions on our own by making assumptions:
	//          a) If called with isUp = true, update all fields
	//          b) If called with isUp = false, update uptime, but not latency or audit success
	//          c) In other words, assume that if a user ONLY wants to update uptime/auditsuccess/latency, they will call the functions dedicated to those operations

	err = s.validateAuth(ctx)
	if err != nil {
		return nil, err
	}

	node := updateReq.GetNode()

	createIfReq := &pb.CreateEntryIfNotExistsRequest{
		Node:   updateReq.GetNode(),
	}

	_, err = s.CreateEntryIfNotExists(ctx, createIfReq)
	if err != nil {
		return nil, err
	}

	dbNode, err := s.DB.Get_Node_By_Id(ctx, dbx.Node_Id(node.NodeId))
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

	dbNode, err = s.DB.Update_Node_By_Id(ctx, dbx.Node_Id(node.NodeId), updateFields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	nodeStats := &pb.NodeStats{
		NodeId:            dbNode.Id,
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

	var nodeStatsList []*pb.NodeStats
	var failedNodes []*pb.Node
	for _, node := range updateBatchReq.NodeList {
		updateReq := &pb.UpdateRequest{
			Node:   node,
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
	defer mon.Task()(&ctx)(&err)
	s.logger.Debug("entering statdb CreateEntryIfNotExists")

	getReq := &pb.GetRequest{
		NodeId: createIfReq.Node.NodeId,
	}
	getRes, err := s.Get(ctx, getReq)
	// TODO: figure out better way to confirm error is type dbx.ErrorCode_NoRows
	if err != nil && strings.Contains(err.Error(), "no rows in result set") {
		createReq := &pb.CreateRequest{
			Node:   createIfReq.Node,
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
	if err != nil {
		return nil, err
	}
	createEntryIfNotExistsRes := &pb.CreateEntryIfNotExistsResponse{
		Stats: getRes.Stats,
	}
	return createEntryIfNotExistsRes, nil
}

func updateRatioVars(newStatus bool, successCount, totalCount int64) (int64, int64, float64) {
	totalCount++
	if newStatus {
		successCount++
	}
	newRatio := float64(successCount) / float64(totalCount)
	return successCount, totalCount, newRatio
}

func checkRatioVars(successCount, totalCount int64) (ratio float64, err error) {
	if successCount < 0 {
		return 0, errs.New("success count less than 0")
	}
	if totalCount < 0 {
		return 0, errs.New("total count less than 0")
	}
	if successCount > totalCount {
		return 0, errs.New("success count greater than total count")
	}

	ratio = float64(successCount) / float64(totalCount)
	return ratio, nil
}
