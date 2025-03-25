// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase"
)

func TestBucketTallyCombine(t *testing.T) {
	tally1 := &accounting.BucketTally{
		BucketLocation:     metabase.BucketLocation{BucketName: "bucket1"},
		ObjectCount:        10,
		PendingObjectCount: 5,
		TotalSegments:      20,
		TotalBytes:         1000,
		MetadataSize:       50,
	}

	tally2 := &accounting.BucketTally{
		BucketLocation:     metabase.BucketLocation{BucketName: "bucket2"},
		ObjectCount:        15,
		PendingObjectCount: 10,
		TotalSegments:      25,
		TotalBytes:         2000,
		MetadataSize:       100,
	}

	tally1.Combine(tally2)

	// Verify the combined values
	require.Equal(t, int64(25), tally1.ObjectCount)
	require.Equal(t, int64(15), tally1.PendingObjectCount)
	require.Equal(t, int64(45), tally1.TotalSegments)
	require.Equal(t, int64(3000), tally1.TotalBytes)
	require.Equal(t, int64(150), tally1.MetadataSize)
}
