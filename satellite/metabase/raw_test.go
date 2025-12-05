// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestTestingBatchInsertObjects_RoundTrip(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj1 := metabasetest.RandObjectStream()
		obj2 := metabasetest.RandObjectStream()

		// create some objects
		metabasetest.CreateObject(ctx, t, db, obj1, 1)
		metabasetest.CreateObjectVersioned(ctx, t, db, obj2, 1)

		// get some valid objects
		validObjects, err := db.TestingAllObjects(ctx)
		require.NoError(t, err)

		// wipe data
		err = db.TestingDeleteAll(ctx)
		require.NoError(t, err)

		var validRawObjects []metabase.RawObject
		for _, obj := range validObjects {
			validRawObjects = append(validRawObjects, metabase.RawObject(obj))
		}

		err = db.TestingBatchInsertObjects(ctx, validRawObjects)
		require.NoError(t, err)

		insertedObjects, err := db.TestingAllObjects(ctx)
		require.NoError(t, err)

		require.Equal(t, validObjects, insertedObjects)
	})
}

func TestTestingBatchInsertSegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		rawSegments := make([]metabase.RawSegment, 10)
		for i := range rawSegments {
			rawSegments[i] = metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{
				Part:  1,
				Index: uint32(i) + 1,
			})
		}

		err := db.TestingBatchInsertSegments(ctx, rawSegments)
		require.NoError(t, err)

		insertedSegments, err := db.TestingAllSegments(ctx)
		require.NoError(t, err)

		require.Zero(t, cmp.Diff(rawSegments, metabasetest.SegmentsToRaw(insertedSegments),
			cmpopts.EquateApproxTime(1*time.Second)))
	})
}
