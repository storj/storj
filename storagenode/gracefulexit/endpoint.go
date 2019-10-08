// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// Error is the default error class for graceful exit package.
var Error = errs.Class("gracefulexit")

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

// GetSatellitesList returns a list of satellites that the storagenode has not begun a graceful exit for.
func (e *Endpoint) GetSatellitesList(ctx context.Context, req *pb.GetSatellitesListRequest) (*pb.GetSatellitesListResponse, error) {
	e.log.Debug("initialize graceful exit: GetSatellitesList")
	// get all trusted satellites
	trustedSatellites := e.trust.GetSatellites(ctx)

	availableSatellites := make([]*pb.Satellite, 0, len(trustedSatellites))

	// filter out satellites that are already exiting
	exitingSatellites, err := e.satellites.ListGracefulExits(ctx)
	if err != nil {
		return nil, err
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
			// TODO: deal with the error
			continue
		}
		// get space usage by satellites
		spaceUsed, err := e.usageCache.SpaceUsedBySatellite(ctx, trusted)
		if err != nil {
			// TODO: deal with the error
			continue
		}
		availableSatellites = append(availableSatellites, &pb.Satellite{
			DomainName: domain,
			NodeId:     trusted,
			SpaceUsed:  float64(spaceUsed),
		})
	}

	return &pb.GetSatellitesListResponse{
		Satellites: availableSatellites,
	}, nil
}

// StartExit updates one or more satellites in the storagenode's database to be gracefully exiting.
func (e *Endpoint) StartExit(ctx context.Context, req *pb.StartExitRequest) (*pb.StartExitResponse, error) {
	e.log.Debug("initialize graceful exit: StartExit", zap.String("satellite count", len(req.NodeIds)))
	// save satellites info into db
	resp := &pb.StartExitResponse{}
	for _, satelliteID := range req.NodeIds {
		e.log.Debug("initialize graceful exit: StartExit", zap.String("satellite ID", satelliteID.String()))
		domain, err := e.trust.GetAddress(ctx, satelliteID)
		if err != nil {
			// TODO: deal with the error
			continue
		}
		status := &pb.StartExitStatus{
			DomainName: domain,
			Success:    false,
		}
		// get space usage by satellites
		spaceUsed, err := e.usageCache.SpaceUsedBySatellite(ctx, satelliteID)
		if err != nil {
			// TODO: deal with the error
			continue
		}
		err = e.satellites.InitiateGracefulExit(ctx, satelliteID, time.Now().UTC(), spaceUsed)
		if err != nil {
			continue
		}
		status.Success = true
		resp.Statuses = append(resp.Statuses, status)
	}

	return resp, nil
}
