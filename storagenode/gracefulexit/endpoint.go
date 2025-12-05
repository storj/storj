// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/storagenode/internalpb"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// Endpoint implements private inspector for Graceful Exit.
type Endpoint struct {
	internalpb.DRPCNodeGracefulExitUnimplementedServer

	log        *zap.Logger
	usageCache *pieces.BlobsUsageCache
	trust      *trust.Pool
	satellites satellites.DB
	dialer     rpc.Dialer
}

// NewEndpoint creates a new graceful exit endpoint.
func NewEndpoint(log *zap.Logger, trust *trust.Pool, satellites satellites.DB, dialer rpc.Dialer, usageCache *pieces.BlobsUsageCache) *Endpoint {
	return &Endpoint{
		log:        log,
		usageCache: usageCache,
		trust:      trust,
		satellites: satellites,
		dialer:     dialer,
	}
}

// GetNonExitingSatellites returns a list of satellites that the storagenode has not begun a graceful exit for.
func (e *Endpoint) GetNonExitingSatellites(ctx context.Context, req *internalpb.GetNonExitingSatellitesRequest) (*internalpb.GetNonExitingSatellitesResponse, error) {
	e.log.Debug("initialize graceful exit: GetSatellitesList")
	// get all trusted satellites
	trustedSatellites := e.trust.GetSatellites(ctx)

	availableSatellites := make([]*internalpb.NonExitingSatellite, 0, len(trustedSatellites))

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
		nodeurl, err := e.trust.GetNodeURL(ctx, trusted)
		if err != nil {
			e.log.Error("graceful exit: get satellite address", zap.Stringer("Satellite ID", trusted), zap.Error(err))
			continue
		}
		// get space usage by satellites
		_, piecesContentSize, err := e.usageCache.SpaceUsedBySatellite(ctx, trusted)
		if err != nil {
			e.log.Debug("graceful exit: get space used by satellite", zap.Stringer("Satellite ID", trusted), zap.Error(err))
			continue
		}
		availableSatellites = append(availableSatellites, &internalpb.NonExitingSatellite{
			DomainName: nodeurl.Address,
			NodeId:     trusted,
			SpaceUsed:  float64(piecesContentSize),
		})
	}

	return &internalpb.GetNonExitingSatellitesResponse{
		Satellites: availableSatellites,
	}, nil
}

// InitiateGracefulExit updates one or more satellites in the storagenode's database to be gracefully exiting.
func (e *Endpoint) InitiateGracefulExit(ctx context.Context, req *internalpb.InitiateGracefulExitRequest) (*internalpb.ExitProgress, error) {
	e.log.Debug("initialize graceful exit: start", zap.Stringer("Satellite ID", req.NodeId))

	nodeurl, err := e.trust.GetNodeURL(ctx, req.NodeId)
	if err != nil {
		e.log.Debug("initialize graceful exit: retrieve satellite address", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	// get space usage by satellites
	_, piecesContentSize, err := e.usageCache.SpaceUsedBySatellite(ctx, req.NodeId)
	if err != nil {
		e.log.Debug("initialize graceful exit: retrieve space used", zap.Stringer("Satellite ID", req.NodeId), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	err = e.satellites.InitiateGracefulExit(ctx, req.NodeId, time.Now().UTC(), piecesContentSize)
	if err != nil {
		e.log.Debug("initialize graceful exit: save info into satellites table", zap.Stringer("Satellite ID", req.NodeId), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &internalpb.ExitProgress{
		DomainName:      nodeurl.Address,
		NodeId:          req.NodeId,
		PercentComplete: float32(0),
	}, nil
}

// GetExitProgress returns graceful exit progress on each satellite that a storagde node has started exiting.
func (e *Endpoint) GetExitProgress(ctx context.Context, req *internalpb.GetExitProgressRequest) (*internalpb.GetExitProgressResponse, error) {
	exitProgress, err := e.satellites.ListGracefulExits(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	resp := &internalpb.GetExitProgressResponse{
		Progress: make([]*internalpb.ExitProgress, 0, len(exitProgress)),
	}
	for _, progress := range exitProgress {
		nodeurl, err := e.trust.GetNodeURL(ctx, progress.SatelliteID)
		if err != nil {
			e.log.Debug("graceful exit: get satellite domain name", zap.Stringer("Satellite ID", progress.SatelliteID), zap.Error(err))
			continue
		}

		var percentCompleted float32
		var exitSucceeded bool

		if progress.StartingDiskUsage != 0 {
			percentCompleted = (float32(progress.BytesDeleted) / float32(progress.StartingDiskUsage)) * 100
		}
		if progress.Status == satellites.ExitSucceeded {
			exitSucceeded = true
			percentCompleted = float32(100)
		}

		resp.Progress = append(resp.Progress,
			&internalpb.ExitProgress{
				DomainName:        nodeurl.Address,
				NodeId:            progress.SatelliteID,
				PercentComplete:   percentCompleted,
				Successful:        exitSucceeded,
				CompletionReceipt: progress.CompletionReceipt,
			},
		)
	}
	return resp, nil
}

// GracefulExitFeasibility returns graceful exit feasibility by node's age on chosen satellite.
func (e *Endpoint) GracefulExitFeasibility(ctx context.Context, request *internalpb.GracefulExitFeasibilityRequest) (*internalpb.GracefulExitFeasibilityResponse, error) {
	nodeurl, err := e.trust.GetNodeURL(ctx, request.NodeId)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %w", request.NodeId, err)
	}

	conn, err := e.dialer.DialNodeURL(ctx, nodeurl)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	client := pb.NewDRPCSatelliteGracefulExitClient(conn)

	feasibility, err := client.GracefulExitFeasibility(ctx, &pb.GracefulExitFeasibilityRequest{})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	response := (internalpb.GracefulExitFeasibilityResponse)(*feasibility)
	return &response, nil
}
