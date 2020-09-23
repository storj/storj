// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package estimatedpayout

import (
	"math"
)

// EstimatedPayout contains usage and estimated payout data for current and previous months.
type EstimatedPayout struct {
	CurrentMonth  PayoutMonthly `json:"currentMonth"`
	PreviousMonth PayoutMonthly `json:"previousMonth"`
}

// PayoutMonthly contains usage and estimated payout date.
type PayoutMonthly struct {
	EgressBandwidth         int64   `json:"egressBandwidth"`
	EgressBandwidthPayout   int64   `json:"egressBandwidthPayout"`
	EgressRepairAudit       int64   `json:"egressRepairAudit"`
	EgressRepairAuditPayout int64   `json:"egressRepairAuditPayout"`
	DiskSpace               float64 `json:"diskSpace"`
	DiskSpacePayout         int64   `json:"diskSpacePayout"`
	HeldRate                int64   `json:"heldRate"`
	Payout                  int64   `json:"payout"`
	Held                    int64   `json:"held"`
}

// SetEgressBandwidthPayout counts egress bandwidth payout for PayoutMonthly object.
func (pm *PayoutMonthly) SetEgressBandwidthPayout(egressPrice int64) {
	pm.EgressBandwidthPayout += int64(float64(pm.EgressBandwidth*egressPrice) / math.Pow10(12))
}

// SetEgressRepairAuditPayout counts audit and repair payout for PayoutMonthly object.
func (pm *PayoutMonthly) SetEgressRepairAuditPayout(auditRepairPrice int64) {
	pm.EgressRepairAuditPayout += int64(float64(pm.EgressRepairAudit*auditRepairPrice) / math.Pow10(12))
}

// SetDiskSpacePayout counts disk space payout for PayoutMonthly object.
func (pm *PayoutMonthly) SetDiskSpacePayout(diskSpacePrice int64) {
	pm.DiskSpacePayout += int64(pm.DiskSpace * float64(diskSpacePrice) / math.Pow10(12))
}

// SetHeldAmount counts held amount for PayoutMonthly object.
func (pm *PayoutMonthly) SetHeldAmount() {
	pm.Held = (pm.DiskSpacePayout + pm.EgressBandwidthPayout + pm.EgressRepairAuditPayout) * pm.HeldRate / 100
}

// SetPayout counts payout amount for PayoutMonthly object.
func (pm *PayoutMonthly) SetPayout() {
	pm.Payout = pm.DiskSpacePayout + pm.EgressBandwidthPayout + pm.EgressRepairAuditPayout - pm.Held
}
