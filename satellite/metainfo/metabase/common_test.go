// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

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
