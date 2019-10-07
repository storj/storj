// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"

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

// GetSatellitesList returns a list of satellites that haven't been exited.
func (s *Endpoint) GetSatellitesList(ctx context.Context, req *pb.GetSatellitesListRequest) (*pb.GetSatellitesListResponse, error) {
	// get all trusted satellites
	trustedSatellites := s.trust.GetSatellites(ctx)

	availableSatellites := make([]*pb.Satellite, 0, len(trustedSatellites))

	// filter out satellites that are already exiting
	existingSatellites, err := s.satellites.ListGracefulExits(ctx)
	if err != nil {
		return nil, err
	}

	for _, trusted := range trustedSatellites {
		var isExisting bool
		for _, existing := range existingSatellites {
			if trusted == existing.SatelliteID {
				isExisting = true
				break
			}
		}

		if !isExisting {
			// get domain name
			domain, err := s.trust.GetAddress(ctx, trusted)
			if err != nil {
				// TODO: deal with the error
				continue
			}
			// get space usage by satellites
			spaceUsed, err := s.usageCache.SpaceUsedBySatellite(ctx, trusted)
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
	}

	return &pb.GetSatellitesListResponse{
		Satellites: availableSatellites,
	}, nil
}
