// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
)

// AdjustRedundancy modifies the redundancy scheme based on repair threshold and target overrides.
func AdjustRedundancy(redundancy storj.RedundancyScheme, repairThresholdOverrides RepairThresholdOverrides, repairTargetOverrides RepairTargetOverrides, placement nodeselection.Placement) storj.RedundancyScheme {
	repair := int(redundancy.RepairShares)
	optimal := int(redundancy.OptimalShares)
	total := int(redundancy.TotalShares)

	if overrideValue := repairThresholdOverrides.GetOverrideValue(redundancy); overrideValue != 0 {
		repair = int(overrideValue)
	}

	if overrideValue := repairTargetOverrides.GetOverrideValue(redundancy); overrideValue != 0 {
		optimal = int(overrideValue)
	}

	if placement.EC.Repair != nil {
		if r := placement.EC.Repair(int(redundancy.RequiredShares)); r > 0 {
			repair = r
		}
	}

	if optimal <= repair {
		// if a segment has exactly repair segments, we consider it in need of
		// repair. we don't want to upload a new object right into the state of
		// needing repair, so we need at least one more, though arguably this is
		// a misconfiguration.
		optimal = repair + 1
	}
	if total < optimal {
		total = optimal
	}

	return storj.RedundancyScheme{
		Algorithm:      redundancy.Algorithm,
		ShareSize:      redundancy.ShareSize,
		RequiredShares: redundancy.RequiredShares,
		RepairShares:   int16(repair),
		OptimalShares:  int16(optimal),
		TotalShares:    int16(total),
	}
}
