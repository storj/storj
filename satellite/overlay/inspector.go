// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
)

// Inspector is a gRPC service for inspecting overlay internals
//
// architecture: Endpoint
type Inspector struct {
	service *Service
}

// NewInspector creates an Inspector
func NewInspector(service *Service) *Inspector {
	return &Inspector{service: service}
}

// CountNodes returns the number of nodes in the overlay.
func (srv *Inspector) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (_ *pb.CountNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	overlayKeys, err := srv.service.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.CountNodesResponse{
		Count: int64(len(overlayKeys)),
	}, nil
}

// DumpNodes returns all of the nodes in the overlay.
func (srv *Inspector) DumpNodes(ctx context.Context, req *pb.DumpNodesRequest) (_ *pb.DumpNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.DumpNodesResponse{}, errs.New("Not Implemented")
}
