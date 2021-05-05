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

// NodeSummary contains node's payout information.
type NodeSummary struct {
	NodeID storj.NodeID `json:"nodeId"`
	Held   int64        `json:"held"`
	Paid   int64        `json:"paid"`
}

// Summary contains payouts page data.
type Summary struct {
	TotalEarned int64         `json:"totalEarned"`
	TotalHeld   int64         `json:"totalHeld"`
	TotalPaid   int64         `json:"totalPaid"`
	NodeSummary []NodeSummary `json:"nodeSummary"`
}
