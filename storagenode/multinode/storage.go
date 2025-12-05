// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/storageusage"
)

var _ multinodepb.DRPCStorageServer = (*StorageEndpoint)(nil)

// StorageEndpoint implements multinode storage endpoint.
//
// architecture: Endpoint
type StorageEndpoint struct {
	multinodepb.DRPCStorageUnimplementedServer

	log     *zap.Logger
	apiKeys *apikeys.Service
	monitor *monitor.Service
	usage   storageusage.DB
}

// NewStorageEndpoint creates new multinode storage endpoint.
func NewStorageEndpoint(log *zap.Logger, apiKeys *apikeys.Service, monitor *monitor.Service, usage storageusage.DB) *StorageEndpoint {
	return &StorageEndpoint{
		log:     log,
		apiKeys: apiKeys,
		monitor: monitor,
		usage:   usage,
	}
}

// DiskSpace returns disk space state.
func (storage *StorageEndpoint) DiskSpace(ctx context.Context, req *multinodepb.DiskSpaceRequest) (_ *multinodepb.DiskSpaceResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, storage.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	diskSpace, err := storage.monitor.DiskSpace(ctx)
	if err != nil {
		storage.log.Error("disk space internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.DiskSpaceResponse{
		Allocated:       diskSpace.Allocated,
		Used:            diskSpace.Used,
		UsedPieces:      diskSpace.UsedForPieces,
		UsedReclaimable: diskSpace.UsedReclaimable,
		UsedTrash:       diskSpace.UsedForTrash,
		Free:            diskSpace.Free,
		Available:       diskSpace.Available,
		Overused:        diskSpace.Overused,
	}, nil
}

// Usage returns daily storage usage for a given interval.
func (storage *StorageEndpoint) Usage(ctx context.Context, req *multinodepb.StorageUsageRequest) (_ *multinodepb.StorageUsageResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, storage.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from := req.GetFrom()
	if from.IsZero() {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, errs.New("from timestamp is not provided"))
	}
	to := req.GetTo()
	if to.IsZero() {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, errs.New("to timestamp is not provided"))
	}

	stamps, err := storage.usage.GetDailyTotal(ctx, from, to)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}
	summary, averageUsageInBytes, err := storage.usage.Summary(ctx, from, to)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	var usage []*multinodepb.StorageUsage
	for _, stamp := range stamps {
		usage = append(usage, &multinodepb.StorageUsage{
			AtRestTotal:      stamp.AtRestTotal,
			AtRestTotalBytes: stamp.AtRestTotalBytes,
			IntervalStart:    stamp.IntervalStart,
		})
	}

	return &multinodepb.StorageUsageResponse{
		StorageUsage:      usage,
		Summary:           summary,
		AverageUsageBytes: averageUsageInBytes,
	}, nil
}

// UsageSatellite returns daily storage usage for a given interval and satellite.
func (storage *StorageEndpoint) UsageSatellite(ctx context.Context, req *multinodepb.StorageUsageSatelliteRequest) (_ *multinodepb.StorageUsageSatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, storage.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	if req.SatelliteId.IsZero() {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, errs.New("satellite id is not provided"))
	}

	from := req.GetFrom()
	if from.IsZero() {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, errs.New("from timestamp is not provided"))
	}
	to := req.GetTo()
	if to.IsZero() {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, errs.New("to timestamp is not provided"))
	}

	stamps, err := storage.usage.GetDaily(ctx, req.SatelliteId, from, to)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}
	summary, averageUsageInBytes, err := storage.usage.SatelliteSummary(ctx, req.SatelliteId, from, to)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	var usage []*multinodepb.StorageUsage
	for _, stamp := range stamps {
		usage = append(usage, &multinodepb.StorageUsage{
			AtRestTotal:      stamp.AtRestTotal,
			AtRestTotalBytes: stamp.AtRestTotalBytes,
			IntervalStart:    stamp.IntervalStart,
		})
	}

	return &multinodepb.StorageUsageSatelliteResponse{
		StorageUsage:      usage,
		Summary:           summary,
		AverageUsageBytes: averageUsageInBytes,
	}, nil
}
