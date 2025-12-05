// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package estimatedpayouts

import (
	"math"
	"time"

	"storj.io/storj/private/date"
)

// EstimatedPayout contains usage and estimated payouts data for current and previous months.
type EstimatedPayout struct {
	CurrentMonth             PayoutMonthly `json:"currentMonth"`
	PreviousMonth            PayoutMonthly `json:"previousMonth"`
	CurrentMonthExpectations int64         `json:"currentMonthExpectations"`
}

// PayoutMonthly contains usage and estimated payouts date.
type PayoutMonthly struct {
	EgressBandwidth         int64   `json:"egressBandwidth"`
	EgressBandwidthPayout   float64 `json:"egressBandwidthPayout"`
	EgressRepairAudit       int64   `json:"egressRepairAudit"`
	EgressRepairAuditPayout float64 `json:"egressRepairAuditPayout"`
	DiskSpace               float64 `json:"diskSpace"`
	DiskSpacePayout         float64 `json:"diskSpacePayout"`
	HeldRate                float64 `json:"heldRate"`
	Payout                  float64 `json:"payout"`
	Held                    float64 `json:"held"`
}

// SetEgressBandwidthPayout counts egress bandwidth payouts for PayoutMonthly object.
func (pm *PayoutMonthly) SetEgressBandwidthPayout(egressPrice int64) {
	amount := float64(pm.EgressBandwidth*egressPrice) / math.Pow10(12)
	pm.EgressBandwidthPayout += RoundFloat(amount)
}

// SetEgressRepairAuditPayout counts audit and repair payouts for PayoutMonthly object.
func (pm *PayoutMonthly) SetEgressRepairAuditPayout(auditRepairPrice int64) {
	amount := float64(pm.EgressRepairAudit*auditRepairPrice) / math.Pow10(12)
	pm.EgressRepairAuditPayout += RoundFloat(amount)
}

// SetDiskSpacePayout counts disk space payouts for PayoutMonthly object.
func (pm *PayoutMonthly) SetDiskSpacePayout(diskSpacePrice int64) {
	amount := pm.DiskSpace * float64(diskSpacePrice) / math.Pow10(12)
	pm.DiskSpacePayout += RoundFloat(amount)
}

// SetHeldAmount counts held amount for PayoutMonthly object.
func (pm *PayoutMonthly) SetHeldAmount() {
	amount := (pm.DiskSpacePayout + pm.EgressBandwidthPayout + pm.EgressRepairAuditPayout) * pm.HeldRate / 100
	pm.Held = amount
}

// SetPayout counts payouts amount for PayoutMonthly object.
func (pm *PayoutMonthly) SetPayout() {
	amount := pm.DiskSpacePayout + pm.EgressBandwidthPayout + pm.EgressRepairAuditPayout - pm.Held
	pm.Payout = RoundFloat(amount)
}

// Add sums payout monthly data.
func (pm *PayoutMonthly) Add(monthly PayoutMonthly) {
	pm.Payout += monthly.Payout
	pm.EgressRepairAuditPayout += monthly.EgressRepairAuditPayout
	pm.DiskSpacePayout += monthly.DiskSpacePayout
	pm.DiskSpace += monthly.DiskSpace
	pm.EgressBandwidth += monthly.EgressBandwidth
	pm.EgressBandwidthPayout += monthly.EgressBandwidthPayout
	pm.EgressRepairAudit += monthly.EgressRepairAudit
	pm.Held += monthly.Held
}

// RoundFloat rounds float value till 2 signs after dot.
func RoundFloat(value float64) float64 {
	return math.Round(value*100) / 100
}

// Set set's estimated payout with current/previous PayoutMonthly's data and current month expectations.
func (estimatedPayout *EstimatedPayout) Set(current, previous PayoutMonthly, now, joinedAt time.Time) {
	estimatedPayout.CurrentMonth = current
	estimatedPayout.PreviousMonth = previous

	beginOfMonth := date.UTCBeginOfMonth(now)
	endOfMonth := date.UTCEndOfMonth(now)

	// Joined before the first day of this month
	if joinedAt.Before(beginOfMonth) {
		minutesPast := now.Sub(beginOfMonth).Minutes()
		estimatedMinutesJoinedCurrentMonth := endOfMonth.Sub(beginOfMonth).Minutes()

		payoutPerMin := estimatedPayout.CurrentMonth.Payout / minutesPast
		estimatedPayout.CurrentMonthExpectations += int64(payoutPerMin * estimatedMinutesJoinedCurrentMonth)
		return
	}
	// Joined this month
	minutesSinceJoined := now.Sub(joinedAt).Minutes()
	estimatedMinutesJoinedCurrentMonth := endOfMonth.Sub(joinedAt).Minutes()
	estimatedPayout.CurrentMonthExpectations += int64(estimatedPayout.CurrentMonth.Payout / minutesSinceJoined * estimatedMinutesJoinedCurrentMonth)
}

// Add adds estimate into the receiver.
func (estimatedPayout *EstimatedPayout) Add(other EstimatedPayout) {
	estimatedPayout.CurrentMonth.Add(other.CurrentMonth)
	estimatedPayout.PreviousMonth.Add(other.PreviousMonth)
	estimatedPayout.CurrentMonthExpectations += other.CurrentMonthExpectations
}
