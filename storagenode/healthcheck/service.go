// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package healthcheck

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/reputation"
)

var (
	// Err defines sno service error.
	Err = errs.Class("healthcheck")

	mon = monkit.Package()
)

// Service is handling storage node estimation payouts logic.
//
// architecture: Service
type Service struct {
	reputationDB reputation.DB
	serveDetails bool
}

// NewService returns new instance of Service.
func NewService(reputationDB reputation.DB, serveDetails bool) *Service {
	return &Service{
		reputationDB: reputationDB,
		serveDetails: serveDetails,
	}
}

// Health represents the current status of the Storage ndoe.
type Health struct {
	Statuses   []SatelliteHealthStatus
	Help       string
	AllHealthy bool
}

// SatelliteHealthStatus is the health status reported by one satellite.
type SatelliteHealthStatus struct {
	OnlineScore    float64
	SatelliteID    storj.NodeID
	DisqualifiedAt *time.Time
	SuspendedAt    *time.Time
}

// GetHealth retrieves current health status based on DB records.
func (s *Service) GetHealth(ctx context.Context) (h Health, err error) {
	defer mon.Task()(&ctx)(&err)
	stats, err := s.reputationDB.All(ctx)

	h.AllHealthy = true

	if err != nil {
		return h, Err.Wrap(err)
	}
	for _, stat := range stats {
		if stat.DisqualifiedAt != nil || stat.SuspendedAt != nil || stat.OnlineScore < 0.9 {
			h.AllHealthy = false
		}

		if s.serveDetails {
			h.Statuses = append(h.Statuses, SatelliteHealthStatus{
				SatelliteID:    stat.SatelliteID,
				OnlineScore:    stat.OnlineScore,
				DisqualifiedAt: stat.DisqualifiedAt,
				SuspendedAt:    stat.SuspendedAt,
			})
		}
	}

	// sg is wrong if we didn't connect to any satellite
	if len(stats) == 0 {
		h.AllHealthy = false
	}

	h.Help = "To access Storagenode services, please use DRPC protocol!"

	return h, nil
}
