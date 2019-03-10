// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"

	"storj.io/storj/pkg/pb"
)

// Inspector is a gRPC service for inspecting irreparable internals
type Inspector struct {
	irrdb DB
}

// NewInspector creates an Inspector
func NewInspector(irrdb DB) *Inspector {
	return &Inspector{irrdb: irrdb}
}

// List returns all files with an irreparable segment
func (srv *Inspector) List(ctx, req *pb.ListRequest) (*pb.ListResponse, error) {
	// need get all
	files, err := srv.irrdb.Get
}
