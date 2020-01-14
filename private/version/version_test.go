// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version_test

import (
	"encoding/json"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/private/version"
)

func TestInfo_IsZero(t *testing.T) {
	zeroInfo := version.Info{}
	require.True(t, zeroInfo.IsZero())

	ver, err := version.NewSemVer("1.2.3")
	require.NoError(t, err)

	info := version.Info{
		Version: ver,
	}
	require.False(t, info.IsZero())
}

func TestSemVer_IsZero(t *testing.T) {
	zeroVer := version.SemVer{}
	require.True(t, zeroVer.IsZero())

	ver, err := version.NewSemVer("1.2.3")
	require.NoError(t, err)
	require.False(t, ver.IsZero())
}

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
	var arbitraryRollout version.Rollout
	for i := 0; i < len(version.RolloutBytes{}); i++ {
		arbitraryRollout.Seed[i] = byte(i)
		arbitraryRollout.Cursor[i] = byte(i * 2)
	}

	scenarios := []struct {
		name    string
		rollout version.Rollout
	}{
		{
			"arbitrary rollout",
			arbitraryRollout,
		},
		{
			"empty rollout",
			version.Rollout{},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			var actualRollout version.Rollout

			_, err := json.Marshal(actualRollout.Seed)
			require.NoError(t, err)

			jsonRollout, err := json.Marshal(scenario.rollout)
			require.NoError(t, err)

			err = json.Unmarshal(jsonRollout, &actualRollout)
			require.NoError(t, err)
			require.Equal(t, scenario.rollout, actualRollout)
		})
	}
}

func TestShouldUpdate(t *testing.T) {
	// NB: total and acceptable tolerance are negatively correlated.
	total := 10000
	tolerance := total * 2 / 100 // 2%

	for p := 10; p < 100; p += 10 {
		var rollouts int
		percentage := p
		cursor := version.PercentageToCursor(percentage)

		rollout := version.Rollout{
			Seed:   version.RolloutBytes{},
			Cursor: cursor,
		}
		rand.Read(rollout.Seed[:])

		for i := 0; i < total; i++ {
			var nodeID storj.NodeID
			_, err := rand.Read(nodeID[:])
			require.NoError(t, err)

			if version.ShouldUpdate(rollout, nodeID) {
				rollouts++
			}
		}

		assert.Condition(t, func() bool {
			diff := rollouts - (total * percentage / 100)
			return int(math.Abs(float64(diff))) < tolerance
		})
	}
}
