// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
)

func TestMakeUuids(t *testing.T) {
	inouts := []struct {
		inTopBits uint32
		outUuid   string
	}{
		{
			0,
			"00000000-0000-0000-0000-000000000000",
		}, {
			0xffffffff,
			"ffffffff-0000-0000-0000-000000000000",
		}, {
			0x12345678,
			"12345678-0000-0000-0000-000000000000",
		},
	}

	for _, inout := range inouts {
		createdUuid, err := rangedloop.MakeUUIDWithTopBits(inout.inTopBits)
		require.NoError(t, err)
		require.Equal(t, inout.outUuid, createdUuid.String())
	}
}

func TestCreateUuidBoundaries(t *testing.T) {
	inouts := []struct {
		inNumRanges uint32
		outUuids    []string
	}{
		{
			0,
			[]string{},
		}, {
			1,
			[]string{},
		}, {
			2,
			[]string{
				"80000000-0000-0000-0000-000000000000",
			},
		}, {
			4,
			[]string{
				"40000000-0000-0000-0000-000000000000",
				"80000000-0000-0000-0000-000000000000",
				"c0000000-0000-0000-0000-000000000000",
			},
		},
	}

	for _, inout := range inouts {
		expectedUuids := []uuid.UUID{}
		for _, outUuidString := range inout.outUuids {
			outUuid, err := uuid.FromString(outUuidString)
			require.NoError(t, err)
			expectedUuids = append(expectedUuids, outUuid)
		}

		createdRange, err := rangedloop.CreateUUIDBoundaries(inout.inNumRanges)
		require.NoError(t, err)
		require.Equal(t, expectedUuids, createdRange)
	}
}

func TestCreateUUIDBoundariesFor8191Ranges(t *testing.T) {
	// 0x1fff = Mersenne prime 8191
	boundaries, err := rangedloop.CreateUUIDBoundaries(0x1fff)
	require.NoError(t, err)
	require.Len(t, boundaries, 0x1ffe)

	// floor(2 ^ 32 / 0x1fff) = 0x80040
	secondUuid, err := uuid.FromString("00080040-0000-0000-0000-000000000000")
	require.NoError(t, err)
	require.Equal(t, secondUuid, boundaries[0])

	// floor(2 ^ 32 / 0x1fff) * 0x1ffe = 0xfff7ff80
	lastUuid, err := uuid.FromString("fff7ff80-0000-0000-0000-000000000000")
	require.NoError(t, err)
	require.Equal(t, lastUuid, boundaries[0x1ffd])
}
