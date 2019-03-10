// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"

	"github.com/golang/protobuf/proto"

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
func (srv *Inspector) List(ctx context.Context, req *pb.ListSegmentsRequest) (resp *pb.ListSegmentsResponse, err error) {
	segments, err := srv.irrdb.GetLimited(ctx, int(req.GetLimit()), req.GetOffset())
	if err != nil {
		return nil, err
	}

	var msg *pb.SegmentGroup
	for _, segment := range segments {
		item := &pb.IrreparableSegment{
			EncryptedPath:      segment.EncryptedSegmentPath,
			SegmentDetail:      segment.EncryptedSegmentDetail,
			LostPieces:         segment.LostPiecesCount,
			LastRepairAttempt:  segment.RepairUnixSec,
			RepairAttemptCount: segment.RepairAttemptCount,
		}
		msg.Segments = append(msg.Segments, item)
	}
	resp.Data, err = proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
