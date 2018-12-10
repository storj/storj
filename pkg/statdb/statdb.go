// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	pb "storj.io/storj/pkg/statdb/proto"
)

// DB interface for database operations
type DB interface {
	// Create a db entry for the provided storagenode
	Create(ctx context.Context, createReq *pb.CreateRequest) (resp *pb.CreateResponse, err error)

	// Get a storagenode's stats from the db
	Get(ctx context.Context, getReq *pb.GetRequest) (resp *pb.GetResponse, err error)

	// FindInvalidNodes finds a subset of storagenodes that fail to meet minimum reputation requirements
	FindInvalidNodes(ctx context.Context, getReq *pb.FindInvalidNodesRequest) (resp *pb.FindInvalidNodesResponse, err error)

	// Update a single storagenode's stats in the db
	Update(ctx context.Context, updateReq *pb.UpdateRequest) (resp *pb.UpdateResponse, err error)

	// UpdateUptime updates a single storagenode's uptime stats in the db
	UpdateUptime(ctx context.Context, updateReq *pb.UpdateUptimeRequest) (resp *pb.UpdateUptimeResponse, err error)

	// UpdateAuditSuccess updates a single storagenode's uptime stats in the db
	UpdateAuditSuccess(ctx context.Context, updateReq *pb.UpdateAuditSuccessRequest) (resp *pb.UpdateAuditSuccessResponse, err error)

	// UpdateBatch for updating multiple farmers' stats in the db
	UpdateBatch(ctx context.Context, updateBatchReq *pb.UpdateBatchRequest) (resp *pb.UpdateBatchResponse, err error)

	// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
	CreateEntryIfNotExists(ctx context.Context, createIfReq *pb.CreateEntryIfNotExistsRequest) (resp *pb.CreateEntryIfNotExistsResponse, err error)
}

// // LoadFromContext loads an existing StatDB from the Provider context
// // stack if one exists.
// func LoadFromContext(ctx context.Context) statdb.DB {
// 	db, ok := ctx.Value("masterdb").(interface {
// 		Statdb() statdb.DB
// 	})
// 	if !ok {
// 		return nil, errs.New("unable to get master db instance")
// 	}
// 	return db.Statdb()
// }
