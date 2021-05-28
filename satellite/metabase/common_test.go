// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

func TestParseBucketPrefixInvalid(t *testing.T) {
	var testCases = []struct {
		name   string
		prefix metabase.BucketPrefix
	}{
		{"invalid, not valid UUID", "not UUID string/bucket1"},
		{"invalid, not valid UUID, no bucket", "not UUID string"},
		{"invalid, no project, no bucket", ""},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := metabase.ParseBucketPrefix(tt.prefix)
			require.NotNil(t, err)
			require.Error(t, err)
		})
	}
}

func TestParseBucketPrefixValid(t *testing.T) {
	var testCases = []struct {
		name               string
		project            string
		bucketName         string
		expectedBucketName string
	}{
		{"valid, no bucket, no objects", "bb6218e3-4b4a-4819-abbb-fa68538e33c0", "", ""},
		{"valid, with bucket", "bb6218e3-4b4a-4819-abbb-fa68538e33c0", "testbucket", "testbucket"},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			expectedProjectID, err := uuid.FromString(tt.project)
			require.NoError(t, err)
			bucketID := expectedProjectID.String() + "/" + tt.bucketName

			bucketLocation, err := metabase.ParseBucketPrefix(metabase.BucketPrefix(bucketID))
			require.NoError(t, err)
			require.Equal(t, expectedProjectID, bucketLocation.ProjectID)
			require.Equal(t, tt.expectedBucketName, bucketLocation.BucketName)
		})
	}
}

func TestParseSegmentKeyInvalid(t *testing.T) {
	var testCases = []struct {
		name       string
		segmentKey string
	}{
		{
			name:       "invalid, project ID only",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0",
		},
		{
			name:       "invalid, project ID and segment index only",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s0",
		},
		{
			name:       "invalid, project ID, bucket, and segment index only",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s0/testbucket",
		},
		{
			name:       "invalid, project ID is not UUID",
			segmentKey: "not UUID string/s0/testbucket/test/object",
		},
		{
			name:       "invalid, last segment with segment number",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/l0/testbucket/test/object",
		},
		{
			name:       "invalid, missing segment number",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s/testbucket/test/object",
		},
		{
			name:       "invalid, missing segment prefix",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/1/testbucket/test/object",
		},
		{
			name:       "invalid, segment index overflows int64",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s18446744073709551616/testbucket/test/object",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := metabase.ParseSegmentKey(metabase.SegmentKey(tt.segmentKey))
			require.NotNil(t, err, tt.name)
			require.Error(t, err, tt.name)
		})
	}
}

func TestParseSegmentKeyValid(t *testing.T) {
	projectID := testrand.UUID()

	var testCases = []struct {
		name             string
		segmentKey       string
		expectedLocation metabase.SegmentLocation
	}{
		{
			name:       "valid, part 0, last segment",
			segmentKey: projectID.String() + "/l/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: metabase.LastSegmentIndex},
			},
		},
		{
			name:       "valid, part 0, last segment, trailing slash",
			segmentKey: projectID.String() + "/l/testbucket/test/object/",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object/",
				Position:   metabase.SegmentPosition{Part: 0, Index: metabase.LastSegmentIndex},
			},
		},
		{
			name:       "valid, part 0, index 0",
			segmentKey: projectID.String() + "/s0/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: 0},
			},
		},
		{
			name:       "valid, part 0, index 1",
			segmentKey: projectID.String() + "/s1/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: 1},
			},
		},
		{
			name:       "valid, part 0, index 315",
			segmentKey: projectID.String() + "/s315/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: 315},
			},
		},
		{
			name:       "valid, part 1, index 0",
			segmentKey: projectID.String() + "/s" + strconv.FormatInt(1<<32, 10) + "/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 1, Index: 0},
			},
		},
		{
			name:       "valid, part 1, index 1",
			segmentKey: projectID.String() + "/s" + strconv.FormatInt(1<<32+1, 10) + "/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 1, Index: 1},
			},
		},
		{
			name:       "valid, part 18, index 315",
			segmentKey: projectID.String() + "/s" + strconv.FormatInt(18<<32+315, 10) + "/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 18, Index: 315},
			},
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			segmentLocation, err := metabase.ParseSegmentKey(metabase.SegmentKey(tt.segmentKey))
			require.NoError(t, err, tt.name)
			require.Equal(t, tt.expectedLocation, segmentLocation)
		})
	}
}

func TestPiecesEqual(t *testing.T) {
	sn1 := testrand.NodeID()
	sn2 := testrand.NodeID()

	var testCases = []struct {
		source metabase.Pieces
		target metabase.Pieces
		equal  bool
	}{
		{metabase.Pieces{}, metabase.Pieces{}, true},
		{
			metabase.Pieces{
				{1, sn1},
			},
			metabase.Pieces{}, false,
		},
		{
			metabase.Pieces{},
			metabase.Pieces{
				{1, sn1},
			}, false,
		},
		{
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			},
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			}, true,
		},
		{
			metabase.Pieces{
				{2, sn2},
				{1, sn1},
			},
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			}, true,
		},
		{
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			},
			metabase.Pieces{
				{1, sn2},
				{2, sn1},
			}, false,
		},
		{
			metabase.Pieces{
				{1, sn1},
				{3, sn2},
				{2, sn2},
			},
			metabase.Pieces{
				{3, sn2},
				{1, sn1},
				{2, sn2},
			}, true,
		},
	}
	for _, tt := range testCases {
		require.Equal(t, tt.equal, tt.source.Equal(tt.target))
	}
}
