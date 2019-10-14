// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/version"
)

func TestSemVer_Compare(t *testing.T) {
	version001, err := version.NewSemVer("v0.0.1")
	require.NoError(t, err)
	version002, err := version.NewSemVer("v0.0.2")
	require.NoError(t, err)
	version030, err := version.NewSemVer("v0.3.0")
	require.NoError(t, err)
	version040, err := version.NewSemVer("v0.4.0")
	require.NoError(t, err)
	version500, err := version.NewSemVer("v5.0.0")
	require.NoError(t, err)
	version600, err := version.NewSemVer("v6.0.0")
	require.NoError(t, err)

	// compare the same values
	require.True(t, version001.Compare(version001) == 0)
	require.True(t, version030.Compare(version030) == 0)
	require.True(t, version500.Compare(version500) == 0)

	require.True(t, version001.Compare(version002) < 0)
	require.True(t, version030.Compare(version040) < 0)
	require.True(t, version500.Compare(version600) < 0)
	require.True(t, version001.Compare(version030) < 0)
	require.True(t, version030.Compare(version500) < 0)

	require.True(t, version002.Compare(version001) > 0)
	require.True(t, version040.Compare(version030) > 0)
	require.True(t, version600.Compare(version500) > 0)
	require.True(t, version030.Compare(version002) > 0)
	require.True(t, version600.Compare(version040) > 0)
}

func TestRollout_MarshalJSON_UnmarshalJSON(t *testing.T) {
	var expectedRollout, actualRollout version.Rollout

	for i := 0; i < len(version.RolloutBytes{}); i++ {
		expectedRollout.Seed[i] = byte(i)
		expectedRollout.Cursor[i] = byte(i * 2)
	}

	_, err := json.Marshal(actualRollout.Seed)
	require.NoError(t, err)

	emptyJSONRollout, err := json.Marshal(actualRollout)
	require.NoError(t, err)

	jsonRollout, err := json.Marshal(expectedRollout)
	require.NoError(t, err)
	require.NotEqual(t, emptyJSONRollout, jsonRollout)

	err = json.Unmarshal(jsonRollout, &actualRollout)
	require.NoError(t, err)
	require.Equal(t, expectedRollout, actualRollout)
}
