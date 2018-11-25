// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"go.uber.org/zap"
	dbx "storj.io/storj/pkg/accounting/dbx"
)

// Server implements the accountingdb RPC service
type Server struct {
	DB     *dbx.DB
	logger *zap.Logger
}

// NewServer creates instance of Server
func NewServer(driver, source string, logger *zap.Logger) (*Server, error) {
	//TODO
	return &Server{}, nil
}

// UpdateBatch
// func (s *Server) UpdateBatch(ctx context.Context, updateBatchReq *pb.UpdateBatchRequest) (resp *pb.UpdateBatchResponse, err error) {

// }
