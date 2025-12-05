// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/repair/checker"
)

func TestRepairOverrideConfigValidation(t *testing.T) {
	tests := []struct {
		description    string
		overrideConfig string
		expectError    bool
		size           int
	}{
		{
			description:    "valid multi repair override config",
			overrideConfig: "2/5/20-3,1/4/10-2",
			expectError:    false,
			size:           2,
		},
		{
			description:    "valid single repair override config",
			overrideConfig: "2/5/20-3",
			expectError:    false,
			size:           1,
		},
		{
			description:    "invalid repair override config - starts at 0",
			overrideConfig: "0/5/6-3",
			expectError:    true,
		},
		{
			description:    "invalid repair override config - strings",
			overrideConfig: "1/2/4-a",
			expectError:    true,
		},
		{
			description:    "invalid repair override config - floating point numbers",
			overrideConfig: "1/5/6-3.2",
			expectError:    true,
		},
		{
			description:    "invalid repair override config - no override value",
			overrideConfig: "1/2/4",
			expectError:    true,
		},
		{
			description:    "invalid repair override config - override < min",
			overrideConfig: "2/5/20-1",
			expectError:    true,
		},
		{
			description:    "valid repair override config - empty items in multi value",
			overrideConfig: ",2/5/20-4,,3/6/7-4",
			expectError:    false,
			size:           2,
		},
		{
			description:    "valid repair override config - empty",
			overrideConfig: "",
			expectError:    false,
			size:           0,
		},
	}

	for _, tt := range tests {
		t.Log(tt.description)

		newOverrides := checker.RepairOverrides{}
		err := newOverrides.Set(tt.overrideConfig)
		if tt.expectError {
			require.Error(t, err, tt.description)
		} else {
			require.NoError(t, err)
			require.Len(t, newOverrides.Values, tt.size)
		}

	}
}

func TestRepairOverrideOldFormat(t *testing.T) {
	overrideConfig := "29/80/95-52,10/30/40-25"
	newOverrides := checker.RepairOverrides{}
	err := newOverrides.Set(overrideConfig)
	require.NoError(t, err)

	schemes := [][]int16{
		{10, 20, 30, 40},
		{29, 35, 80, 95},
		{29, 60, 80, 95},
		{2, 5, 10, 30},
	}
	storjSchemes := []storj.RedundancyScheme{}
	pbSchemes := []*pb.RedundancyScheme{}
	for _, scheme := range schemes {
		newStorj := storj.RedundancyScheme{
			RequiredShares: scheme[0],
			RepairShares:   scheme[1],
			OptimalShares:  scheme[2],
			TotalShares:    scheme[3],
		}
		storjSchemes = append(storjSchemes, newStorj)

		newPB := &pb.RedundancyScheme{
			MinReq:           int32(scheme[0]),
			RepairThreshold:  int32(scheme[1]),
			SuccessThreshold: int32(scheme[2]),
			Total:            int32(scheme[3]),
		}
		pbSchemes = append(pbSchemes, newPB)
	}

	ro := newOverrides
	require.EqualValues(t, 25, ro.GetOverrideValue(storjSchemes[0]))
	require.EqualValues(t, 25, ro.GetOverrideValuePB(pbSchemes[0]))

	// second and third schemes should have the same override value (52) despite having a different repair threshold.
	require.EqualValues(t, 52, ro.GetOverrideValue(storjSchemes[1]))
	require.EqualValues(t, 52, ro.GetOverrideValuePB(pbSchemes[1]))
	require.EqualValues(t, 52, ro.GetOverrideValue(storjSchemes[2]))
	require.EqualValues(t, 52, ro.GetOverrideValuePB(pbSchemes[2]))

	// fourth scheme has no matching override config.
	require.EqualValues(t, 0, ro.GetOverrideValue(storjSchemes[3]))
	require.EqualValues(t, 0, ro.GetOverrideValuePB(pbSchemes[3]))
}

func TestRepairOverride(t *testing.T) {
	overrideConfig := "10-15,29-52"
	newOverrides := checker.RepairOverrides{}
	err := newOverrides.Set(overrideConfig)
	require.NoError(t, err)

	schemes := [][]int16{
		{10, 29, 70, 75},
		{29, 35, 80, 95},
		{1, 2, 3, 4},
	}
	storjSchemes := []storj.RedundancyScheme{}
	pbSchemes := []*pb.RedundancyScheme{}
	for _, scheme := range schemes {
		newStorj := storj.RedundancyScheme{
			RequiredShares: scheme[0],
			RepairShares:   scheme[1],
			OptimalShares:  scheme[2],
			TotalShares:    scheme[3],
		}
		storjSchemes = append(storjSchemes, newStorj)

		newPB := &pb.RedundancyScheme{
			MinReq:           int32(scheme[0]),
			RepairThreshold:  int32(scheme[1]),
			SuccessThreshold: int32(scheme[2]),
			Total:            int32(scheme[3]),
		}
		pbSchemes = append(pbSchemes, newPB)
	}

	ro := newOverrides
	require.EqualValues(t, 15, ro.GetOverrideValue(storjSchemes[0]))
	require.EqualValues(t, 15, ro.GetOverrideValuePB(pbSchemes[0]))

	require.EqualValues(t, 52, ro.GetOverrideValue(storjSchemes[1]))
	require.EqualValues(t, 52, ro.GetOverrideValuePB(pbSchemes[1]))

	require.EqualValues(t, 0, ro.GetOverrideValue(storjSchemes[2]))
	require.EqualValues(t, 0, ro.GetOverrideValuePB(pbSchemes[2]))
}
