// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
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
	Error        = errs.Class("Endpoint error")
	finalSegment = int64(-1)
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

	var segmentHealthResponses []*pb.SegmentHealthResponse

	limit := int64(100)
	if in.GetLimit() > 0 {
		limit = int64(in.GetLimit())
	}

	var start int64
	if in.GetStartAfterSegment() > 0 {
		start = in.GetStartAfterSegment() + 1
	}

	end := int64(limit) + start
	if in.GetEndBeforeSegment() > 0 {
		end = in.GetEndBeforeSegment()
	}

	// Receive path
	i := start
	for i < end {
		if i-start >= limit {
			break
		}

		segment := &pb.SegmentHealthRequest{
			Bucket:        in.GetBucket(),
			EncryptedPath: in.GetEncryptedPath(),
			Segment:       i,
			ProjectId:     in.GetProjectId(),
		}

		segmentHealth, err := endpoint.SegmentStat(ctx, segment)
		if err != nil {
			if i == finalSegment {
				return nil, Error.Wrap(err)
			}

			i = finalSegment
			continue
		}

		segmentHealthResponses = append(segmentHealthResponses, segmentHealth)

		if i == finalSegment {
			break
		}

		i++
	}

	resp = &pb.ObjectHealthResponse{
		Segments: segmentHealthResponses,
	}

	return resp, nil
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

	projectID, err := uuid.Parse(string(in.GetProjectId()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	path, err := storj.CreatePath(*projectID, in.GetSegment(), in.GetBucket(), in.GetEncryptedPath())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// get pointer info
	pointer, err := endpoint.pointerdb.Get(path)
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
	resp.OnlineNodes = int32(len(nodes))

	return resp, nil
}
