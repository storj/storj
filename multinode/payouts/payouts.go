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

// HeldAmountHistory contains held amount history of particular satellite.
type HeldAmountHistory struct {
	SatelliteID storj.NodeID `json:"satelliteId"`
	HeldAmounts []HeldAmount `json:"heldAmounts"`
}

// HeldAmount contains amount held for period.
type HeldAmount struct {
	Period string `json:"period"`
	Amount int64  `json:"amount"`
}

// HeldAmountSummary contains held amount summary for first 3 quarters.
type HeldAmountSummary struct {
	SatelliteID      storj.NodeID `json:"satelliteId"`
	SatelliteAddress string       `json:"satelliteAddress"`
	FirstQuarter     int64        `json:"firstQuarter"`
	SecondQuarter    int64        `json:"secondQuarter"`
	ThirdQuarter     int64        `json:"thirdQuarter"`
	PeriodCount      int          `json:"periodCount"`
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

// Paystub is node payouts data for satellite by specific period.
type Paystub struct {
	UsageAtRest    float64 `json:"usageAtRest"`
	UsageGet       int64   `json:"usageGet"`
	UsageGetRepair int64   `json:"usageGetRepair"`
	UsageGetAudit  int64   `json:"usageGetAudit"`
	CompAtRest     int64   `json:"compAtRest"`
	CompGet        int64   `json:"compGet"`
	CompGetRepair  int64   `json:"compGetRepair"`
	CompGetAudit   int64   `json:"compGetAudit"`
	Held           int64   `json:"held"`
	Paid           int64   `json:"paid"`
	Distributed    int64   `json:"distributed"`
	Disposed       int64   `json:"disposed"`
}
