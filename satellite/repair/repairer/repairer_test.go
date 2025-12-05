// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/structs"

	"storj.io/common/storj"
)

func TestPlacementList(t *testing.T) {
	pl := Config{}

	decode := structs.Decode(map[string]string{
		"excluded-placements": "1,3,5,6",
	}, &pl)

	require.NoError(t, decode.Error)
	require.Len(t, decode.Broken, 0)
	require.Len(t, decode.Missing, 0)
	require.Len(t, decode.Used, 1)

	require.Equal(t, []storj.PlacementConstraint{1, 3, 5, 6}, pl.ExcludedPlacements.Placements)
}
