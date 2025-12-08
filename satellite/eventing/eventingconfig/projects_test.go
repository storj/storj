// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.
package eventingconfig_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/satellite/eventing/eventingconfig"
)

func TestProjectSet(t *testing.T) {
	t.Run("invalid inputs", func(t *testing.T) {
		var s eventingconfig.ProjectSet

		// Invalid UUID
		err := s.Set("notauuid")
		require.Error(t, err)

		// Multiple UUIDs with one invalid
		validID := testrand.UUID()
		err = s.Set(validID.String() + ",notauuid")
		require.Error(t, err)
	})

	t.Run("set and string", func(t *testing.T) {
		// Generate test UUIDs
		idA := testrand.UUID()
		idB := testrand.UUID()

		// Valid input with two UUIDs
		validStr := idA.String() + "," + idB.String()
		var s eventingconfig.ProjectSet
		err := s.Set(validStr)
		require.NoError(t, err)
		require.Len(t, s, 2)

		// Check both UUIDs are in the set
		require.True(t, s.Enabled(idA))
		require.True(t, s.Enabled(idB))

		// String output should contain both UUIDs
		str := s.String()
		require.Contains(t, str, idA.String())
		require.Contains(t, str, idB.String())

		// Single UUID
		idC := testrand.UUID()
		err = s.Set(idC.String())
		require.NoError(t, err)
		require.Len(t, s, 1)
		require.True(t, s.Enabled(idC))
		require.False(t, s.Enabled(idA))

		// Multiple UUIDs with whitespace
		err = s.Set(idA.String() + " , " + idB.String())
		require.NoError(t, err)
		require.Len(t, s, 2)
		require.True(t, s.Enabled(idA))
		require.True(t, s.Enabled(idB))

		// Empty input
		err = s.Set("")
		require.NoError(t, err)
		require.Len(t, s, 0)
		require.Equal(t, "", s.String())

		// Input with empty parts (extra commas)
		err = s.Set(idA.String() + ",,," + idB.String())
		require.NoError(t, err)
		require.Len(t, s, 2)
		require.True(t, s.Enabled(idA))
		require.True(t, s.Enabled(idB))
	})

	t.Run("enabled", func(t *testing.T) {
		projectID1 := testrand.UUID()
		projectID2 := testrand.UUID()
		projectID3 := testrand.UUID()

		s := eventingconfig.ProjectSet{
			projectID1: {},
			projectID2: {},
		}

		require.True(t, s.Enabled(projectID1))
		require.True(t, s.Enabled(projectID2))
		require.False(t, s.Enabled(projectID3))
		require.False(t, s.Enabled(testrand.UUID()))
	})
}
