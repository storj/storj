// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts

import (
	"storj.io/common/storj"
)

// SatelliteSummary contains satellite id and earned amount.
type SatelliteSummary struct {
	SatelliteID storj.NodeID `json:"satelliteID"`
	Earned      int64        `json:"earned"`
}
