// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"time"

	"storj.io/storj/pkg/storj"
)

// Stats encapsulates storagenode stats retrieved from the satellite
type Stats struct {
	SatelliteID storj.NodeID

	UptimeCheck ReputationStats
	AuditCheck  ReputationStats

	UpdatedAt time.Time
}

// ReputationStats encapsulates storagenode reputation metrics
type ReputationStats struct {
	TotalCount   int64
	SuccessCount int64

	ReputationAlpha float64
	ReputationBeta  float64
	ReputationScore float64
}
