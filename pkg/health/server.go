// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package health

import (
	"context"

	"go.uber.org/zap"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
)

var (
	// Error wraps errors returned from Server struct methods
	Error = errs.Class("HealthEndpoint error")
)

type HealthEndpoint struct {
	pointerdb *pointerdb.Service
	overlay   pb.OverlayServer
	log       *zap.Logger
}

func NewHealthEndpoint(pdb *pointerdb.Service, os pb.OverlayServer, log *zap.Logger) (*HealthEndpoint, error) {
	return &HealthEndpoint{
		log:       log,
		overlay:   os,
		pointerdb: pdb,
	}, nil
}

func (s *HealthEndpoint) ObjectStat(context.Context, *pb.ObjectHealthRequest) (*pb.ObjectHealthResponse, error) {

	return nil, nil
}

func (s *HealthEndpoint) SegmentStat(context.Context, *pb.SegmentHealthRequest) (*pb.SegmentInfo, error) {

	return nil, nil
}
