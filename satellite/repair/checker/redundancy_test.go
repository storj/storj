// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/repair/checker"
)

func TestAdjustRedundancy(t *testing.T) {
	tests := []struct {
		name                     string
		redundancy               storj.RedundancyScheme
		repairThresholdOverrides checker.RepairThresholdOverrides
		repairTargetOverrides    checker.RepairTargetOverrides
		placement                nodeselection.Placement
		expectedRepairShares     int16
		expectedOptimalShares    int16
		expectedTotalShares      int16
	}{
		{
			name: "no overrides",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement:                nodeselection.Placement{},
			expectedRepairShares:     3,
			expectedOptimalShares:    4,
			expectedTotalShares:      5,
		},
		{
			name: "repair threshold override",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: createRepairThresholdOverrides(t, "2-6"),
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement:                nodeselection.Placement{},
			expectedRepairShares:     6,
			expectedOptimalShares:    7, // adjusted to repair + 1
			expectedTotalShares:      7, // adjusted to optimal
		},
		{
			name: "repair target override",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    createRepairTargetOverrides(t, "2-8"),
			placement:                nodeselection.Placement{},
			expectedRepairShares:     3,
			expectedOptimalShares:    8,
			expectedTotalShares:      8, // adjusted to optimal
		},
		{
			name: "both overrides",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: createRepairThresholdOverrides(t, "2-6"),
			repairTargetOverrides:    createRepairTargetOverrides(t, "2-10"),
			placement:                nodeselection.Placement{},
			expectedRepairShares:     6,
			expectedOptimalShares:    10,
			expectedTotalShares:      10,
		},
		{
			name: "placement EC repair override",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement: nodeselection.Placement{
				EC: nodeselection.ECParameters{
					Repair: func(requiredShares int) int {
						return requiredShares + 3 // return 5 for requiredShares=2
					},
				},
			},
			expectedRepairShares:  5,
			expectedOptimalShares: 6, // adjusted to repair + 1
			expectedTotalShares:   6, // adjusted to optimal
		},
		{
			name: "placement EC repair returns zero (no override)",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement: nodeselection.Placement{
				EC: nodeselection.ECParameters{
					Repair: func(requiredShares int) int {
						return 0 // no override
					},
				},
			},
			expectedRepairShares:  3,
			expectedOptimalShares: 4,
			expectedTotalShares:   5,
		},
		{
			name: "placement EC repair returns negative (no override)",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement: nodeselection.Placement{
				EC: nodeselection.ECParameters{
					Repair: func(requiredShares int) int {
						return -1 // no override
					},
				},
			},
			expectedRepairShares:  3,
			expectedOptimalShares: 4,
			expectedTotalShares:   5,
		},
		{
			name: "placement EC repair override takes precedence over repair threshold override",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: createRepairThresholdOverrides(t, "2-7"),
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement: nodeselection.Placement{
				EC: nodeselection.ECParameters{
					Repair: func(requiredShares int) int {
						return 5 // this overrides the threshold override
					},
				},
			},
			expectedRepairShares:  5, // from placement override, not threshold override
			expectedOptimalShares: 6, // adjusted to repair + 1
			expectedTotalShares:   6,
		},
		{
			name: "optimal equals repair adjustment",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   5,
				OptimalShares:  5, // same as repair
				TotalShares:    6,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement:                nodeselection.Placement{},
			expectedRepairShares:     5,
			expectedOptimalShares:    6, // adjusted to repair + 1
			expectedTotalShares:      6,
		},
		{
			name: "optimal less than repair adjustment",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   5,
				OptimalShares:  3, // less than repair
				TotalShares:    6,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement:                nodeselection.Placement{},
			expectedRepairShares:     5,
			expectedOptimalShares:    6, // adjusted to repair + 1
			expectedTotalShares:      6,
		},
		{
			name: "total less than optimal adjustment",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  8,
				TotalShares:    5, // less than optimal
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement:                nodeselection.Placement{},
			expectedRepairShares:     3,
			expectedOptimalShares:    8,
			expectedTotalShares:      8, // adjusted to optimal
		},
		{
			name: "nil placement EC repair function",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 2,
				RepairShares:   3,
				OptimalShares:  4,
				TotalShares:    5,
			},
			repairThresholdOverrides: checker.RepairThresholdOverrides{},
			repairTargetOverrides:    checker.RepairTargetOverrides{},
			placement: nodeselection.Placement{
				EC: nodeselection.ECParameters{
					Repair: nil, // nil function
				},
			},
			expectedRepairShares:  3,
			expectedOptimalShares: 4,
			expectedTotalShares:   5,
		},
		{
			name: "no override for this required shares value",
			redundancy: storj.RedundancyScheme{
				RequiredShares: 3, // different from override key
				RepairShares:   4,
				OptimalShares:  5,
				TotalShares:    6,
			},
			repairThresholdOverrides: createRepairThresholdOverrides(t, "2-8"), // override for k=2, not k=3
			repairTargetOverrides:    createRepairTargetOverrides(t, "2-10"),   // override for k=2, not k=3
			placement:                nodeselection.Placement{},
			expectedRepairShares:     4,
			expectedOptimalShares:    5,
			expectedTotalShares:      6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.AdjustRedundancy(tt.redundancy, tt.repairThresholdOverrides, tt.repairTargetOverrides, tt.placement)

			require.Equal(t, tt.redundancy.RequiredShares, result.RequiredShares, "RequiredShares should not change")
			require.Equal(t, tt.expectedRepairShares, result.RepairShares, "RepairShares mismatch")
			require.Equal(t, tt.expectedOptimalShares, result.OptimalShares, "OptimalShares mismatch")
			require.Equal(t, tt.expectedTotalShares, result.TotalShares, "TotalShares mismatch")
		})
	}
}

// Helper function to create RepairThresholdOverrides from string
func createRepairThresholdOverrides(t *testing.T, overrideStr string) checker.RepairThresholdOverrides {
	var overrides checker.RepairThresholdOverrides
	err := overrides.Set(overrideStr)
	require.NoError(t, err)
	return overrides
}

// Helper function to create RepairTargetOverrides from string
func createRepairTargetOverrides(t *testing.T, overrideStr string) checker.RepairTargetOverrides {
	var overrides checker.RepairTargetOverrides
	err := overrides.Set(overrideStr)
	require.NoError(t, err)
	return overrides
}
