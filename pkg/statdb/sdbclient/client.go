// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package sdbclient

import (
	"context"

	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/provider"
	pb "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	mon = monkit.Package()
)

// StatDB creates a grpcClient
type StatDB struct {
	client pb.StatDBClient
	APIKey []byte
}

// Client services offerred for the interface
type Client interface {
	Create(ctx context.Context, id storj.NodeID) error
	CreateWithStats(ctx context.Context, id storj.NodeID, stats *pb.NodeStats) error
	Get(ctx context.Context, id storj.NodeID) (*pb.NodeStats, error)
	FindValidNodes(ctx context.Context, ids storj.NodeIDList, minStats *pb.NodeStats) (passedIDs storj.NodeIDList, err error)
	Update(ctx context.Context, id storj.NodeID, auditSuccess, isUp bool,
		latencyList []int64) (stats *pb.NodeStats, err error)
	UpdateUptime(ctx context.Context, id storj.NodeID, isUp bool) (*pb.NodeStats, error)
	UpdateAuditSuccess(ctx context.Context, id storj.NodeID, passed bool) (*pb.NodeStats, error)
	UpdateBatch(ctx context.Context, nodes []*pb.Node) ([]*pb.NodeStats, []*pb.Node, error)
	CreateEntryIfNotExists(ctx context.Context, id storj.NodeID) (stats *pb.NodeStats, err error)
}

// NewClient initializes a new statdb client
func NewClient(identity *provider.FullIdentity, address string, APIKey string) (Client, error) {
	apiKeyInjector := grpcauth.NewAPIKeyInjector(APIKey)
	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(
		context.Background(),
		address,
		grpc.WithUnaryInterceptor(apiKeyInjector),
	)
	if err != nil {
		return nil, err
	}

	return &StatDB{client: pb.NewStatDBClient(conn)}, nil
}

// a compiler trick to make sure *StatDB implements Client
var _ Client = (*StatDB)(nil)

// Create is used for creating a new entry in the stats db with default reputation
func (sdb *StatDB) Create(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	node := pb.Node{
		Id: id,
	}
	createReq := &pb.CreateRequest{
		Node: &node,
	}
	_, err = sdb.client.Create(ctx, createReq)

	return err
}

// CreateWithStats is used for creating a new entry in the stats db with a specific reputation
// stats must have AuditCount, AuditSuccessCount, UptimeCount, UptimeSuccessCount
func (sdb *StatDB) CreateWithStats(ctx context.Context, id storj.NodeID, stats *pb.NodeStats) (err error) {
	defer mon.Task()(&ctx)(&err)

	node := &pb.Node{
		Id: id,
	}
	createReq := &pb.CreateRequest{
		Node:  node,
		Stats: stats,
	}
	_, err = sdb.client.Create(ctx, createReq)

	return err
}

// Get is used for retrieving a new entry from the stats db
func (sdb *StatDB) Get(ctx context.Context, id storj.NodeID) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	getReq := &pb.GetRequest{
		NodeId: id,
	}
	res, err := sdb.client.Get(ctx, getReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, nil
}

// FindValidNodes is used for retrieving a subset of nodes that meet a minimum reputation requirement
// minStats must have AuditSuccessRatio, UptimeRatio, AuditCount
func (sdb *StatDB) FindValidNodes(ctx context.Context, ids storj.NodeIDList,
	minStats *pb.NodeStats) (passedIDs storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	findValidNodesReq := &pb.FindValidNodesRequest{
		NodeIds:  ids,
		MinStats: minStats,
	}

	res, err := sdb.client.FindValidNodes(ctx, findValidNodesReq)
	if err != nil {
		return nil, err
	}

	return res.PassedIds, nil
}

// Update is used for updating a node's stats in the stats db
func (sdb *StatDB) Update(ctx context.Context, id storj.NodeID,
	auditSuccess, isUp bool, latencyList []int64) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	node := pb.Node{
		Id:                 id,
		AuditSuccess:       auditSuccess,
		IsUp:               isUp,
		LatencyList:        latencyList,
		UpdateAuditSuccess: true,
		UpdateUptime:       true,
		UpdateLatency:      true,
	}
	updateReq := &pb.UpdateRequest{
		Node: &node,
	}

	res, err := sdb.client.Update(ctx, updateReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, nil
}

// UpdateUptime is used for updating a node's uptime in statdb
func (sdb *StatDB) UpdateUptime(ctx context.Context, id storj.NodeID,
	isUp bool) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	node := pb.Node{
		Id:   id,
		IsUp: isUp,
	}
	updateReq := &pb.UpdateUptimeRequest{
		Node: &node,
	}

	res, err := sdb.client.UpdateUptime(ctx, updateReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, nil
}

// UpdateAuditSuccess is used for updating a node's audit success in statdb
func (sdb *StatDB) UpdateAuditSuccess(ctx context.Context, id storj.NodeID,
	passed bool) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	node := pb.Node{
		Id:           id,
		AuditSuccess: passed,
	}
	updateReq := &pb.UpdateAuditSuccessRequest{
		Node: &node,
	}

	res, err := sdb.client.UpdateAuditSuccess(ctx, updateReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, nil
}

// UpdateBatch is used for updating multiple nodes' stats in the stats db
func (sdb *StatDB) UpdateBatch(ctx context.Context, nodes []*pb.Node) (statsList []*pb.NodeStats, failedNodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	updateBatchReq := &pb.UpdateBatchRequest{
		NodeList: nodes,
	}

	res, err := sdb.client.UpdateBatch(ctx, updateBatchReq)
	if err != nil {
		return nil, nil, err
	}

	return res.StatsList, res.FailedNodes, nil
}

// CreateEntryIfNotExists creates a db entry for a node if entry doesn't already exist
func (sdb *StatDB) CreateEntryIfNotExists(ctx context.Context, id storj.NodeID) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	node := &pb.Node{Id: id}
	createReq := &pb.CreateEntryIfNotExistsRequest{
		Node: node,
	}

	res, err := sdb.client.CreateEntryIfNotExists(ctx, createReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, nil
}
