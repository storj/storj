// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package health

import (
	"context"

	"storj.io/storj/pkg/pb"
)

type HealthEndpoint struct {
	// pointerdb
	// overlay cache
}

func (s *HealthEndpoint) ObjectStat(context.Context, *pb.ObjectHealthRequest) (*pb.ObjectHealthResponse, error) {

	return nil, nil
}

func (s *HealthEndpoint) SegmentStat(context.Context, *pb.SegmentHealthRequest) (*pb.SegmentInfo, error) {

	return nil, nil
}
