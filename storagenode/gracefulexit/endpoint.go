// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// Endpoint is
type Endpoint struct {
	log        *zap.Logger
	usageCache *pieces.BlobsUsageCache
	trust      *trust.Pool
	satellites satellites.DB
}

// NewEndpoint creates a new graceful exit endpoint.
func NewEndpoint(log *zap.Logger, trust *trust.Pool, satellites satellites.DB, usageCache *pieces.BlobsUsageCache) *Endpoint {
	return &Endpoint{
		log:        log,
		usageCache: usageCache,
		trust:      trust,
		satellites: satellites,
	}
}

// GetNonExitingSatellites returns a list of satellites that the storagenode has not begun a graceful exit for.
func (e *Endpoint) GetNonExitingSatellites(ctx context.Context, req *pb.GetNonExitingSatellitesRequest) (*pb.GetNonExitingSatellitesResponse, error) {
	e.log.Debug("initialize graceful exit: GetSatellitesList")
	// get all trusted satellites
	trustedSatellites := e.trust.GetSatellites(ctx)

	availableSatellites := make([]*pb.NonExitingSatellite, 0, len(trustedSatellites))

	// filter out satellites that are already exiting
	exitingSatellites, err := e.satellites.ListGracefulExits(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	for _, trusted := range trustedSatellites {
		var isExiting bool
		for _, exiting := range exitingSatellites {
			if trusted == exiting.SatelliteID {
				isExiting = true
				break
			}
		}

		if isExiting {
			continue
		}

		// get domain name
		domain, err := e.trust.GetAddress(ctx, trusted)
		if err != nil {
			e.log.Debug("graceful exit: get satellite domian name", zap.Stringer("Satellite ID", trusted), zap.Error(err))
			continue
		}
		// get space usage by satellites
		spaceUsed, err := e.usageCache.SpaceUsedBySatellite(ctx, trusted)
		if err != nil {
			e.log.Debug("graceful exit: get space used by satellite", zap.Stringer("Satellite ID", trusted), zap.Error(err))
			continue
		}
		availableSatellites = append(availableSatellites, &pb.NonExitingSatellite{
			DomainName: domain,
			NodeId:     trusted,
			SpaceUsed:  float64(spaceUsed),
		})
	}

	return &pb.GetNonExitingSatellitesResponse{
		Satellites: availableSatellites,
	}, nil
}

// InitiateGracefulExit updates one or more satellites in the storagenode's database to be gracefully exiting.
func (e *Endpoint) InitiateGracefulExit(ctx context.Context, req *pb.InitiateGracefulExitRequest) (*pb.ExitProgress, error) {
	e.log.Debug("initialize graceful exit: start", zap.Stringer("Satellite ID", req.NodeId))

	domain, err := e.trust.GetAddress(ctx, req.NodeId)
	if err != nil {
		e.log.Debug("initialize graceful exit: retrieve satellite address", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// get space usage by satellites
	spaceUsed, err := e.usageCache.SpaceUsedBySatellite(ctx, req.NodeId)
	if err != nil {
		e.log.Debug("initialize graceful exit: retrieve space used", zap.Stringer("Satellite ID", req.NodeId), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	err = e.satellites.InitiateGracefulExit(ctx, req.NodeId, time.Now().UTC(), spaceUsed)
	if err != nil {
		e.log.Debug("initialize graceful exit: save info into satellites table", zap.Stringer("Satellite ID", req.NodeId), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &pb.ExitProgress{
		DomainName:      domain,
		NodeId:          req.NodeId,
		PercentComplete: float32(0),
	}, nil
}

// GetExitProgress returns graceful exit progress on each satellite that a storagde node has started exiting.
func (e *Endpoint) GetExitProgress(ctx context.Context, req *pb.GetExitProgressRequest) (*pb.GetExitProgressResponse, error) {
	exitProgress, err := e.satellites.ListGracefulExits(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	resp := &pb.GetExitProgressResponse{
		Progress: make([]*pb.ExitProgress, 0, len(exitProgress)),
	}
	for _, progress := range exitProgress {
		domain, err := e.trust.GetAddress(ctx, progress.SatelliteID)
		if err != nil {
			e.log.Debug("graceful exit: get satellite domain name", zap.Stringer("Satellite ID", progress.SatelliteID), zap.Error(err))
			continue
		}

		var percentCompleted float32
		var hasCompleted bool

		if progress.StartingDiskUsage != 0 {
			percentCompleted = (float32(progress.BytesDeleted) / float32(progress.StartingDiskUsage)) * 100
		}
		if progress.CompletionReceipt != nil {
			hasCompleted = true
		}

		resp.Progress = append(resp.Progress,
			&pb.ExitProgress{
				DomainName:        domain,
				NodeId:            progress.SatelliteID,
				PercentComplete:   percentCompleted,
				Successful:        hasCompleted,
				CompletionReceipt: progress.CompletionReceipt,
			},
		)
	}
	return resp, nil
}
