// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

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
	Error = errs.Class("Endpoint error")
)

type Endpoint struct {
	pointerdb *pointerdb.Service
	cache     *overlay.Cache
	log       *zap.Logger
}

func NewEndpoint(log *zap.Logger, cache *overlay.Cache, pdb *pointerdb.Service) (*Endpoint, error) {
	return &Endpoint{
		log:       log,
		cache:     cache,
		pointerdb: pdb,
	}, nil
}

func (endpoint *Endpoint) ObjectStat(ctx context.Context, in *pb.ObjectHealthRequest) (resp *pb.ObjectHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return nil, nil
}

func (endpoint *Endpoint) SegmentStat(ctx context.Context, in *pb.SegmentHealthRequest) (resp *pb.SegmentHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	resp = &pb.SegmentHealthResponse{
		OnlineNodes:      0,
		MinimumRequired:  0,
		Total:            0,
		SuccessThreshold: 0,
		RepairThreshold:  0,
	}

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

	for _, node := range nodes {
		if node.GetIsUp() {
			resp.OnlineNodes += 1
		}
	}

	neededForRepair := resp.GetOnlineNodes() - int32(redundancy.RepairThreshold())
	if neededForRepair < 0 {
		neededForRepair = int32(0)
	}

	neededForSuccess := resp.GetOnlineNodes() - int32(redundancy.OptimalThreshold())
	if neededForSuccess < 0 {
		neededForSuccess = int32(0)
	}

	resp.MinimumRequired = int32(redundancy.RequiredCount())
	resp.Total = int32(redundancy.TotalCount())
	resp.RepairThreshold = neededForRepair
	resp.SuccessThreshold = neededForSuccess

	return resp, nil
}
