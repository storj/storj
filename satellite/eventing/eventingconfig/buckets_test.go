// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.
package eventingconfig_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/metabase"
)

func TestBucketLocationTopicIDMap(t *testing.T) {
	t.Run("invalid inputs", func(t *testing.T) {
		var m eventingconfig.BucketLocationTopicIDMap

		// Not enough parts
		err := m.Set("invalidtuple")
		require.Error(t, err)

		// Invalid UUID
		err = m.Set("notauuid:bucket:value")
		require.Error(t, err)

		// Invalid bucket name (empty)
		id := testrand.UUID()
		err = m.Set(id.String() + "::value")
		require.Error(t, err)
	})

	t.Run("set and string", func(t *testing.T) {
		// Generate test UUIDs
		idA := testrand.UUID()
		idB := testrand.UUID()

		// Valid input
		validStr := idA.String() + ":bucketA:valueA," + idB.String() + ":bucketB:valueB"
		var m eventingconfig.BucketLocationTopicIDMap
		err := m.Set(validStr)
		require.NoError(t, err)
		require.Len(t, m, 2)

		// Check values
		foundA := false
		foundB := false
		for loc, val := range m {
			if loc.ProjectID == idA && loc.BucketName == "bucketA" {
				require.Equal(t, "valueA", val)
				foundA = true
			}
			if loc.ProjectID == idB && loc.BucketName == "bucketB" {
				require.Equal(t, "valueB", val)
				foundB = true
			}
		}
		require.True(t, foundA)
		require.True(t, foundB)

		// String output should contain both tuples
		str := m.String()
		require.Contains(t, str, idA.String()+":bucketA:valueA")
		require.Contains(t, str, idB.String()+":bucketB:valueB")

		err = m.Set("01000000-0000-0000-0000-000000000000:bucketA:valueA,01000000-0000-0000-0000-000000000000:bucketB:valueA,01000000-0000-0000-0000-000000000000:bucketC:valueA")
		require.NoError(t, err)
		require.Len(t, m, 3)

		for _, bucketName := range []string{"bucketA", "bucketB", "bucketC"} {
			require.Equal(t, "valueA", m[metabase.BucketLocation{
				ProjectID:  uuid.UUID{1},
				BucketName: metabase.BucketName(bucketName),
			}])
		}

		// Empty input
		err = m.Set("")
		require.NoError(t, err)
		require.Len(t, m, 0)
		require.Equal(t, "", m.String())
	})

	t.Run("enabled", func(t *testing.T) {
		projectID1 := testrand.UUID()
		projectID2 := testrand.UUID()
		bucketName1 := "bucket1"
		bucketName2 := "bucket2"
		topic1 := "topicA"
		topic2 := "topicB"

		m := eventingconfig.BucketLocationTopicIDMap{
			metabase.BucketLocation{
				ProjectID:  projectID1,
				BucketName: metabase.BucketName(bucketName1),
			}: topic1,
			metabase.BucketLocation{
				ProjectID:  projectID2,
				BucketName: metabase.BucketName(bucketName2),
			}: topic2,
		}

		require.True(t, m.Enabled(projectID1, bucketName1))
		require.True(t, m.Enabled(projectID2, bucketName2))
		require.False(t, m.Enabled(projectID1, bucketName2))
		require.False(t, m.Enabled(testrand.UUID(), "nonexistent"))
	})

	t.Run("get topic id", func(t *testing.T) {
		projectID1 := testrand.UUID()
		projectID2 := testrand.UUID()

		bucketMap := eventingconfig.BucketLocationTopicIDMap{
			metabase.BucketLocation{
				ProjectID:  projectID1,
				BucketName: "bucketA",
			}: "topic1",
			metabase.BucketLocation{
				ProjectID:  projectID2,
				BucketName: "bucketB",
			}: "topic2",
		}

		tests := []struct {
			name       string
			projectID  uuid.UUID
			bucketName string
			want       string
		}{
			{
				name:       "existing bucketA",
				projectID:  projectID1,
				bucketName: "bucketA",
				want:       "topic1",
			},
			{
				name:       "existing bucketB",
				projectID:  projectID2,
				bucketName: "bucketB",
				want:       "topic2",
			},
			{
				name:       "non-existing bucket",
				projectID:  testrand.UUID(),
				bucketName: "bucketC",
				want:       "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := bucketMap.GetTopicID(tt.projectID, tt.bucketName)
				require.Equal(t, tt.want, got)
			})
		}
	})
}
