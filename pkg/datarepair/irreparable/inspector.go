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

// ListSegments returns a number of irreparable segments by limit and offset
func (srv *Inspector) ListSegments(ctx context.Context, req *pb.ListSegmentsRequest) (*pb.ListSegmentsResponse, error) {
	segments, err := srv.irrdb.GetLimited(ctx, int(req.GetLimit()), req.GetOffset())
	if err != nil {
		return nil, err
	}

	var resp pb.ListSegmentsResponse
	for _, segment := range segments {
		item := &pb.IrreparableSegment{
			EncryptedPath:      segment.EncryptedSegmentPath,
			SegmentDetail:      segment.EncryptedSegmentDetail,
			LostPieces:         segment.LostPiecesCount,
			LastRepairAttempt:  segment.RepairUnixSec,
			RepairAttemptCount: segment.RepairAttemptCount,
		}
		resp.Segments = append(resp.Segments, item)
	}

	return &resp, err
}
