// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// DB interface for database operations
type DB interface {
	// Create a db entry for the provided storagenode
	Create(ctx context.Context, createReq *CreateRequest) (resp *CreateResponse, err error)

	// Get a storagenode's stats from the db
	Get(ctx context.Context, getReq *GetRequest) (resp *GetResponse, err error)

	// FindInvalidNodes finds a subset of storagenodes that fail to meet minimum reputation requirements
	FindInvalidNodes(ctx context.Context, getReq *FindInvalidNodesRequest) (resp *FindInvalidNodesResponse, err error)

	// Update a single storagenode's stats in the db
	Update(ctx context.Context, updateReq *UpdateRequest) (resp *UpdateResponse, err error)

	// UpdateUptime updates a single storagenode's uptime stats in the db
	UpdateUptime(ctx context.Context, updateReq *UpdateUptimeRequest) (resp *UpdateUptimeResponse, err error)

	// UpdateAuditSuccess updates a single storagenode's uptime stats in the db
	UpdateAuditSuccess(ctx context.Context, updateReq *UpdateAuditSuccessRequest) (resp *UpdateAuditSuccessResponse, err error)

	// UpdateBatch for updating multiple farmers' stats in the db
	UpdateBatch(ctx context.Context, updateBatchReq *UpdateBatchRequest) (resp *UpdateBatchResponse, err error)

	// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
	CreateEntryIfNotExists(ctx context.Context, createIfReq *CreateEntryIfNotExistsRequest) (resp *CreateEntryIfNotExistsResponse, err error)
}

// CreateRequest is a statdb create request message
type CreateRequest struct {
	Node  *pb.Node
	Stats *pb.NodeStats
}

// CreateResponse is a statdb create response message
type CreateResponse struct {
	Stats *pb.NodeStats
}

// GetRequest is a statdb get request message
type GetRequest struct {
	NodeId storj.NodeID
}

// GetResponse is a statdb get response message
type GetResponse struct {
	Stats *pb.NodeStats
}

// FindInvalidNodesRequest is a statdb find invalid node request message
type FindInvalidNodesRequest struct {
	NodeIds  []storj.NodeID
	MaxStats *pb.NodeStats
}

// FindInvalidNodesResponse is a statdb find invalid node response message
type FindInvalidNodesResponse struct {
	InvalidIds []storj.NodeID
}

// UpdateRequest is a statdb update request message
type UpdateRequest struct {
	Node *pb.Node
}

//GetNode returns the node info
func (m *UpdateRequest) GetNode() *pb.Node {
	if m != nil {
		return m.Node
	}
	return nil
}

// UpdateRequest is a statdb update response message
type UpdateResponse struct {
	Stats *pb.NodeStats
}

// UpdateUptimeRequest is a statdb uptime request message
type UpdateUptimeRequest struct {
	Node *pb.Node
}

//GetNode returns the node info
func (m *UpdateUptimeRequest) GetNode() *pb.Node {
	if m != nil {
		return m.Node
	}
	return nil
}

// UpdateUptimeResponse is a statdb uptime response message
type UpdateUptimeResponse struct {
	Stats *pb.NodeStats
}

// UpdateAuditSuccessRequest is a statdb audit request message
type UpdateAuditSuccessRequest struct {
	Node *pb.Node
}

//GetNode returns the node info
func (m *UpdateAuditSuccessRequest) GetNode() *pb.Node {
	if m != nil {
		return m.Node
	}
	return nil
}

// UpdateAuditSuccessResponse is a statdb audit response message
type UpdateAuditSuccessResponse struct {
	Stats *pb.NodeStats
}

// UpdateBatchRequest is a statdb update batch request message
type UpdateBatchRequest struct {
	NodeList []*pb.Node
}

// UpdateBatchResponse is a statdb update batch response message
type UpdateBatchResponse struct {
	StatsList   []*pb.NodeStats
	FailedNodes []*pb.Node
}

// GetFailedNodes returns failed node list
func (m *UpdateBatchResponse) GetFailedNodes() []*pb.Node {
	if m != nil {
		return m.FailedNodes
	}
	return nil
}

// CreateEntryIfNotExistsRequest is a statdb create entry request message
type CreateEntryIfNotExistsRequest struct {
	Node *pb.Node
}

// CreateEntryIfNotExistsResponse is a statdb create response message
type CreateEntryIfNotExistsResponse struct {
	Stats *pb.NodeStats
}
