// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package health

import (
	"context"

	"storj.io/storj/pkg/pb"
)

type HealthClient struct {
}

func NewHealthClient() *HealthClient {
	return &HealthClient{}
}

func (client *HealthClient) ObjectStat(ctx context.Context, in *pb.ObjectHealthRequest) (*pb.ObjectHealthResponse, error) {

	return nil, nil
}
func (client *HealthClient) SegmentStat(ctx context.Context, in *pb.SegmentHealthRequest) (*pb.SegmentInfo, error) {

	return nil, nil
}
