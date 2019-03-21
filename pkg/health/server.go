// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package health

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
	// Error wraps errors returned from Server struct methods
	Error = errs.Class("HealthEndpoint error")
)

type HealthEndpoint struct {
	pointerdb *pointerdb.Service
	cache     *overlay.Cache
	log       *zap.Logger
}

func NewHealthEndpoint(pdb *pointerdb.Service, cache *overlay.Cache, log *zap.Logger) (*HealthEndpoint, error) {
	return &HealthEndpoint{
		log:       log,
		cache:     cache,
		pointerdb: pdb,
	}, nil
}

func (endpoint *HealthEndpoint) ObjectStat(ctx context.Context, in *pb.ObjectHealthRequest) (resp *pb.ObjectHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return nil, nil
}

func (endpoint *HealthEndpoint) SegmentStat(ctx context.Context, in *pb.SegmentHealthRequest) (resp *pb.SegmentInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	// get pointer info
	pointer, err := endpoint.pointerdb.Get(string(in.GetEncryptedPath()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return nil, Error.New("cannot check health of inline segment")
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var nodeIDs storj.NodeIDList
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		nodeIDs = append(nodeIDs, piece.NodeId)
	}

	nodes, err := endpoint.cache.GetAll(ctx, nodeIDs)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	resp.MinReq = 0
	resp.Total = 0
	resp.RepairThreshold = 0
	resp.SuccessThreshold = 0
	resp.OnlineNodes = 0

	return resp, nil
}
