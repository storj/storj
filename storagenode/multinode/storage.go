// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/monitor"
)

var _ multinodepb.DRPCStorageServer = (*StorageEndpoint)(nil)

// StorageEndpoint implements multinode storage endpoint.
//
// architecture: Endpoint
type StorageEndpoint struct {
	log     *zap.Logger
	apiKeys *apikeys.Service
	monitor *monitor.Service
}

// NewStorageEndpoint creates new multinode storage endpoint.
func NewStorageEndpoint(log *zap.Logger, apiKeys *apikeys.Service, monitor *monitor.Service) *StorageEndpoint {
	return &StorageEndpoint{
		log:     log,
		apiKeys: apiKeys,
		monitor: monitor,
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
		Allocated:  diskSpace.Allocated,
		UsedPieces: diskSpace.UsedForPieces,
		UsedTrash:  diskSpace.UsedForTrash,
		Free:       diskSpace.Free,
		Available:  diskSpace.Available,
		Overused:   diskSpace.Overused,
	}, nil
}
