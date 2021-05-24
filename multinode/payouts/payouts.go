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
	NodeID   storj.NodeID `json:"nodeId"`
	NodeName string       `json:"nodeName"`
	Held     int64        `json:"held"`
	Paid     int64        `json:"paid"`
}

// Summary contains payouts page data.
type Summary struct {
	TotalEarned int64         `json:"totalEarned"`
	TotalHeld   int64         `json:"totalHeld"`
	TotalPaid   int64         `json:"totalPaid"`
	NodeSummary []NodeSummary `json:"nodeSummary"`
}

// Add appends node payout data to summary.
func (summary *Summary) Add(held, paid int64, id storj.NodeID, name string) {
	summary.TotalPaid += paid
	summary.TotalHeld += held
	summary.TotalEarned += paid + held
	summary.NodeSummary = append(summary.NodeSummary, NodeSummary{
		NodeID:   id,
		Held:     held,
		Paid:     paid,
		NodeName: name,
	})
}

// Expectations contains estimated and undistributed payouts.
type Expectations struct {
	CurrentMonthEstimation int64 `json:"currentMonthEstimation"`
	Undistributed          int64 `json:"undistributed"`
}
