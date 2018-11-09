// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package sdbclient

import (
	"context"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/provider"
	pb "storj.io/storj/pkg/statdb/proto"
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
	Create(ctx context.Context, nodeID []byte) error
	Get(ctx context.Context, nodeID []byte) (*pb.NodeStats, error)
	FindValidNodes(ctx context.Context, nodeIDs [][]byte, minAuditCount int64,
		minAuditSuccess, minUptime float64) (passedIDs [][]byte, err error)
	Update(ctx context.Context, nodeID []byte, auditSuccess, isUp bool, latencyList []int64,
		updateAuditSuccess, updateUptime, updateLatency bool) (*pb.NodeStats, error)
	UpdateBatch(ctx context.Context, nodes []*pb.Node) ([]*pb.NodeStats, []*pb.Node, error)
	CreateEntryIfNotExists(ctx context.Context, node *pb.Node) (stats *pb.NodeStats, err error)
}

// NewClient initializes a new statdb client
func NewClient(identity *provider.FullIdentity, address string, APIKey []byte) (Client, error) {
	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(context.Background(), address)
	if err != nil {
		return nil, err
	}

	return &StatDB{
		client: pb.NewStatDBClient(conn),
		APIKey: APIKey,
	}, nil
}

// a compiler trick to make sure *StatDB implements Client
var _ Client = (*StatDB)(nil)

// Create is used for creating a new entry in the stats db
func (sdb *StatDB) Create(ctx context.Context, nodeID []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	node := pb.Node{
		NodeId:             nodeID,
		UpdateAuditSuccess: false,
		UpdateUptime:       false,
	}
	createReq := &pb.CreateRequest{
		Node:   &node,
		APIKey: sdb.APIKey,
	}
	_, err = sdb.client.Create(ctx, createReq)

	return err
}

// Get is used for retrieving a new entry from the stats db
func (sdb *StatDB) Get(ctx context.Context, nodeID []byte) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	getReq := &pb.GetRequest{
		NodeId: nodeID,
		APIKey: sdb.APIKey,
	}
	res, err := sdb.client.Get(ctx, getReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, err
}

// FindValidNodes is used for retrieving a subset of nodes that meet a minimum reputation requirement
func (sdb *StatDB) FindValidNodes(ctx context.Context, nodeIDs [][]byte, minAuditCount int64,
	minAuditSuccess, minUptime float64) (passedIDs [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)

	findValidNodesReq := &pb.FindValidNodesRequest{
		NodeIds: nodeIDs,
		MinStats: &pb.NodeStats{
			AuditSuccessRatio: minAuditSuccess,
			UptimeRatio:       minUptime,
			AuditCount:        minAuditCount,
		},
		APIKey: sdb.APIKey,
	}

	res, err := sdb.client.FindValidNodes(ctx, findValidNodesReq)
	if err != nil {
		return nil, err
	}

	return res.PassedIds, nil
}

// Update is used for updating a node's stats in the stats db
func (sdb *StatDB) Update(ctx context.Context, nodeID []byte, auditSuccess, isUp bool, latencyList []int64,
	updateAuditSuccess, updateUptime, updateLatency bool) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	node := pb.Node{
		NodeId:             nodeID,
		AuditSuccess:       auditSuccess,
		IsUp:               isUp,
		LatencyList:        latencyList,
		UpdateAuditSuccess: updateAuditSuccess,
		UpdateUptime:       updateUptime,
		UpdateLatency:      updateLatency,
	}
	updateReq := &pb.UpdateRequest{
		Node:   &node,
		APIKey: sdb.APIKey,
	}

	res, err := sdb.client.Update(ctx, updateReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, err
}

// UpdateBatch is used for updating multiple nodes' stats in the stats db
func (sdb *StatDB) UpdateBatch(ctx context.Context, nodes []*pb.Node) (statsList []*pb.NodeStats, failedNodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	updateBatchReq := &pb.UpdateBatchRequest{
		NodeList: nodes,
		APIKey:   sdb.APIKey,
	}

	res, err := sdb.client.UpdateBatch(ctx, updateBatchReq)
	if err != nil {
		return nil, nil, err
	}

	return res.StatsList, res.FailedNodes, err
}

// CreateEntryIfNotExists creates a db entry for a node if entry doesn't already exist
func (sdb *StatDB) CreateEntryIfNotExists(ctx context.Context, node *pb.Node) (stats *pb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	createReq := &pb.CreateEntryIfNotExistsRequest{
		Node:   node,
		APIKey: sdb.APIKey,
	}

	res, err := sdb.client.CreateEntryIfNotExists(ctx, createReq)
	if err != nil {
		return nil, err
	}

	return res.Stats, err
}
