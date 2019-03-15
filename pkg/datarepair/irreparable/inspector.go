// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"

	"github.com/golang/protobuf/ptypes/timestamp"
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
	segments, err := srv.irrdb.GetLimited(ctx, int(req.GetLimit()), int64(req.GetOffset()))
	if err != nil {
		return nil, err
	}

	var resp pb.ListSegmentsResponse
	resp.Segments = segments

	p := &pb.Pointer{
		Remote: &pb.RemoteSegment{
			Redundancy:   &pb.RedundancyScheme{},
			RemotePieces: []*pb.RemotePiece{&pb.RemotePiece{}},
		},
		CreationDate:   &timestamp.Timestamp{},
		ExpirationDate: &timestamp.Timestamp{},
	}

	for i := 0; i < 10; i++ {

		item := &pb.IrreparableSegment{
			Path:               []byte{'a', '/', 'l', '/', 'c'},
			SegmentDetail:      p,
			LostPieces:         3,
			LastRepairAttempt:  1234567890,
			RepairAttemptCount: 5,
		}
		resp.Segments = append(resp.Segments, item)

	}
	return &resp, err
}
