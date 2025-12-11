// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package avrometabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase/avrometabase"
)

func TestObjectsIterator(t *testing.T) {
	// this test is not perfect as uses export from QA environment but
	// still gives a chance to verify that parsing works as expected.

	ctx := testcontext.New(t)

	readerIterator := avrometabase.NewFileIterator("testdata/objects-test.avro")
	objectsIterator := avrometabase.NewObjectIterator(readerIterator)

	count := 0
	for object, err := range objectsIterator.Iterate(ctx) {
		require.NoError(t, err)

		require.False(t, object.ProjectID.IsZero())
		require.NotEmpty(t, object.BucketName)
		require.NotEmpty(t, object.ObjectKey)
		require.NotZero(t, object.Version)
		require.NotZero(t, object.CreatedAt)
		require.NotZero(t, object.Encryption)

		count++
	}
	require.Equal(t, 50, count)
}
