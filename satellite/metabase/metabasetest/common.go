// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
)

// DeleteAll deletes all data from metabase.
type DeleteAll struct{}

// Check runs the test.
func (step DeleteAll) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.TestingDeleteAll(ctx)
	require.NoError(t, err)
}

// Verify verifies whether metabase state matches the content.
type Verify metabase.RawState

// Check runs the test.
func (step Verify) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	state, err := db.TestingGetState(ctx)
	require.NoError(t, err)

	sortRawObjects(state.Objects)
	sortRawObjects(step.Objects)
	sortRawSegments(state.Segments)
	sortRawSegments(step.Segments)

	diff := cmp.Diff(metabase.RawState(step), *state,
		DefaultTimeDiff(),
		cmpopts.EquateEmpty())
	require.Zero(t, diff)
}

func sortObjects(objects []metabase.Object) {
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].StreamID.Less(objects[j].StreamID)
	})
}

func sortBucketTallies(tallies []metabase.BucketTally) {
	sort.Slice(tallies, func(i, j int) bool {
		if tallies[i].ProjectID == tallies[j].ProjectID {
			return tallies[i].BucketName < tallies[j].BucketName
		}
		return tallies[i].ProjectID.Less(tallies[j].ProjectID)
	})
}

func sortRawObjects(objects []metabase.RawObject) {
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].ObjectStream.Less(objects[j].ObjectStream)
	})
}

func sortRawSegments(segments []metabase.RawSegment) {
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].StreamID == segments[j].StreamID {
			return segments[i].Position.Less(segments[j].Position)
		}
		return segments[i].StreamID.Less(segments[j].StreamID)
	})
}

func checkError(t require.TestingT, err error, errClass *errs.Class, errText string) {
	if errClass != nil {
		require.True(t, errClass.Has(err), "expected an error %q got %q", *errClass, err)
	}
	if errText != "" {
		require.EqualError(t, err, errClass.New("%v", errText).Error())
	}
	if errClass == nil && errText == "" {
		require.NoError(t, err)
	}
}

// DefaultTimeDiff is the central place to adjust test sql "timeout" (accepted diff between start and end of the test).
func DefaultTimeDiff() cmp.Option {
	return cmpopts.EquateApproxTime(20 * time.Second)
}
