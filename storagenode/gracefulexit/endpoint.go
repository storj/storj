// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// Error is the default error class for graceful exit package.
var Error = errs.Class("gracefulexit")

// Endpoint is
type Endpoint struct {
	log          *zap.Logger
	cacheService *pieces.CacheService
	trust        *trust.Pool
	satellites   satellites.DB
}

// NewEndpoint creates a new graceful exit endpoint.
func NewEndpoint(log *zap.Logger, trust *trust.Pool, satellites satellites.DB, cacheService *pieces.CacheService) *Endpoint {
	return &Endpoint{
		log:          log,
		cacheService: cacheService,
		trust:        trust,
		satellites:   satellites,
	}
}

// GetSatellites returns a list of satellites that haven't been exited.
func (s *Endpoint) GetSatellites(ctx context.Context) ([]storj.NodeID, error) {
	// get all trusted satellites
	trustedSatellites := s.trust.GetSatellites(ctx)
	availableSatellites := make([]storj.NodeID, 0, len(trustedSatellites))
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
			availableSatellites = append(availableSatellites, trusted)
		}
	}

	return availableSatellites, nil
}
