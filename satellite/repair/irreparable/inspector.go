// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/pb"
)

var (
	mon = monkit.Package()
)

// Inspector is a gRPC service for inspecting irreparable internals
//
// architecture: Endpoint
type Inspector struct {
	irrdb DB
}

// NewInspector creates an Inspector
func NewInspector(irrdb DB) *Inspector {
	return &Inspector{irrdb: irrdb}
}

// ListIrreparableSegments returns a number of irreparable segments by limit and offset
func (srv *Inspector) ListIrreparableSegments(ctx context.Context, req *pb.ListIrreparableSegmentsRequest) (_ *pb.ListIrreparableSegmentsResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	last := req.GetLastSeenSegmentPath()
	if len(req.GetLastSeenSegmentPath()) == 0 {
		last = []byte{}
	}
	segments, err := srv.irrdb.GetLimited(ctx, int(req.GetLimit()), last)
	if err != nil {
		return nil, err
	}

	return &pb.ListIrreparableSegmentsResponse{Segments: segments}, err
}
