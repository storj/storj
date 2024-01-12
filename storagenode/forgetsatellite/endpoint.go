// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package forgetsatellite

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/internalpb"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()
)

// Endpoint implements private inspector for forget-satellite.
type Endpoint struct {
	internalpb.DRPCNodeForgetSatelliteUnimplementedServer

	log        *zap.Logger
	trust      *trust.Pool
	satellites satellites.DB
}

// NewEndpoint creates a new forget satellite endpoint.
func NewEndpoint(log *zap.Logger, trust *trust.Pool, satellites satellites.DB) *Endpoint {
	return &Endpoint{
		log:        log,
		trust:      trust,
		satellites: satellites,
	}
}

// InitForgetSatellite initializes the forget-satellite process for a satellite.
func (e *Endpoint) InitForgetSatellite(ctx context.Context, req *internalpb.InitForgetSatelliteRequest) (_ *internalpb.InitForgetSatelliteResponse, err error) {
	defer mon.Task()(&ctx, req.SatelliteId)(&err)

	logger := e.log.With(zap.Stringer("satelliteID", req.SatelliteId)).With(zap.String("action", "InitForgetSatellite"))

	satellite, err := e.satellites.GetSatellite(ctx, req.SatelliteId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if satellite.SatelliteID.IsZero() {
		logger.Debug("satellite not found")
		if !req.ForceCleanup {
			return nil, rpcstatus.Error(rpcstatus.NotFound, "satellite not found")
		}
		// if force cleanup is requested, we add the satellite to the database.
		err = e.satellites.SetAddressAndStatus(ctx, req.SatelliteId, "", satellites.Untrusted)
		if err != nil {
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
		satellite = satellites.Satellite{
			SatelliteID: req.SatelliteId,
			Status:      satellites.Untrusted,
		}
	}

	if satellite.Status == satellites.CleanupInProgress {
		logger.Debug("satellite is already being cleaned up")
		return nil, rpcstatus.Error(rpcstatus.AlreadyExists, "satellite is already being cleaned up")
	}

	if !req.ForceCleanup && satellite.Status != satellites.Untrusted {
		logger.Debug("satellite is not untrusted")
		return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, "satellite is not untrusted")
	}

	logger.Info("initializing forget satellite")
	err = e.satellites.UpdateSatelliteStatus(ctx, req.SatelliteId, satellites.CleanupInProgress)
	if err != nil {
		logger.Error("failed to update satellite status", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &internalpb.InitForgetSatelliteResponse{
		SatelliteId: req.SatelliteId,
		InProgress:  true,
	}, nil
}

// ForgetSatelliteStatus returns the status of the forget-satellite process for a satellite.
func (e *Endpoint) ForgetSatelliteStatus(ctx context.Context, req *internalpb.ForgetSatelliteStatusRequest) (_ *internalpb.ForgetSatelliteStatusResponse, err error) {
	defer mon.Task()(&ctx, req.SatelliteId)(&err)

	logger := e.log.With(zap.Stringer("satelliteID", req.SatelliteId)).With(zap.String("action", "ForgetSatelliteStatus"))

	satellite, err := e.satellites.GetSatellite(ctx, req.SatelliteId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if satellite.SatelliteID.IsZero() {
		logger.Debug("satellite not found")
		return nil, rpcstatus.Error(rpcstatus.NotFound, "satellite not found")
	}

	return &internalpb.ForgetSatelliteStatusResponse{
		SatelliteId: req.SatelliteId,
		InProgress:  satellite.Status == satellites.CleanupInProgress,
		Successful:  satellite.Status == satellites.CleanupSucceeded,
	}, nil
}

// GetUntrustedSatellites returns a list of untrusted satellites.
func (e *Endpoint) GetUntrustedSatellites(ctx context.Context, req *internalpb.GetUntrustedSatellitesRequest) (_ *internalpb.GetUntrustedSatellitesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	logger := e.log.With(zap.String("action", "GetUntrustedSatellites"))

	sats, err := e.satellites.GetSatellites(ctx)
	if err != nil {
		logger.Error("failed to get satellites", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var untrustedSatellites []storj.NodeID
	for _, satellite := range sats {
		if satellite.Status == satellites.Untrusted {
			untrustedSatellites = append(untrustedSatellites, satellite.SatelliteID)
		}
	}

	return &internalpb.GetUntrustedSatellitesResponse{
		SatelliteIds: untrustedSatellites,
	}, nil
}

// GetAllForgetSatelliteStatus returns the status of the forget-satellite process for all satellites.
func (e *Endpoint) GetAllForgetSatelliteStatus(ctx context.Context, _ *internalpb.GetAllForgetSatelliteStatusRequest) (_ *internalpb.GetAllForgetSatelliteStatusResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	logger := e.log.With(zap.String("action", "GetAllForgetSatelliteStatus"))

	sats, err := e.satellites.GetSatellites(ctx)
	if err != nil {
		logger.Error("failed to get satellites", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var statuses []*internalpb.ForgetSatelliteStatusResponse
	for _, satellite := range sats {
		if !isCleanupSatellite(&satellite) {
			continue
		}
		statuses = append(statuses, &internalpb.ForgetSatelliteStatusResponse{
			SatelliteId: satellite.SatelliteID,
			InProgress:  satellite.Status == satellites.CleanupInProgress,
			Successful:  satellite.Status == satellites.CleanupSucceeded,
		})
	}

	return &internalpb.GetAllForgetSatelliteStatusResponse{
		Statuses: statuses,
	}, nil
}

func isCleanupSatellite(satellite *satellites.Satellite) bool {
	return satellite.Status == satellites.CleanupInProgress || satellite.Status == satellites.CleanupSucceeded || satellite.Status == satellites.CleanupFailed
}
