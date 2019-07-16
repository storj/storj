// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"time"

	"storj.io/storj/pkg/storj"
)

// DiskSpaceInfo stores all info about storagenode disk space usage
type DiskSpaceInfo struct {
	Available int64 `json:"available"`
	Used      int64 `json:"used"`
}

// SpaceUsageStamp is space usage for satellite at some point in time
type SpaceUsageStamp struct {
	RollupID    int64
	SatelliteID storj.NodeID

	AtRestTotal float64

	Timestamp time.Time
}
