// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/storj/satellite/internalpb"
)

var (
	mon = monkit.Package()
)

// Inspector is a RPC service for inspecting irreparable internals
//
// architecture: Endpoint
type Inspector struct {
	irrdb DB
}

// NewInspector creates an Inspector.
func NewInspector(irrdb DB) *Inspector {
	return &Inspector{irrdb: irrdb}
}

// ListIrreparableSegments returns a number of irreparable segments by limit and offset.
func (srv *Inspector) ListIrreparableSegments(ctx context.Context, req *internalpb.ListIrreparableSegmentsRequest) (_ *internalpb.ListIrreparableSegmentsResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	last := req.GetLastSeenSegmentPath()
	if len(req.GetLastSeenSegmentPath()) == 0 {
		last = []byte{}
	}
	segments, err := srv.irrdb.GetLimited(ctx, int(req.GetLimit()), last)
	if err != nil {
		return nil, err
	}

	return &internalpb.ListIrreparableSegmentsResponse{Segments: segments}, err
}
