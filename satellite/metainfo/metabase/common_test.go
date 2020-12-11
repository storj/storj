// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
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
