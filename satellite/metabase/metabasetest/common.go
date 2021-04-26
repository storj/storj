// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"bytes"
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
		cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

func sortObjects(objects []metabase.Object) {
	sort.Slice(objects, func(i, j int) bool {
		return bytes.Compare(objects[i].StreamID[:], objects[j].StreamID[:]) < 0
	})
}

func sortRawObjects(objects []metabase.RawObject) {
	sort.Slice(objects, func(i, j int) bool {
		return bytes.Compare(objects[i].StreamID[:], objects[j].StreamID[:]) < 0
	})
}

func sortRawSegments(segments []metabase.RawSegment) {
	sort.Slice(segments, func(i, j int) bool {
		return bytes.Compare(segments[i].StreamID[:], segments[j].StreamID[:]) < 0
	})
}

func checkError(t testing.TB, err error, errClass *errs.Class, errText string) {
	if errClass != nil {
		require.True(t, errClass.Has(err), "expected an error %v got %v", *errClass, err)
	}
	if errText != "" {
		require.EqualError(t, err, errClass.New(errText).Error())
	}
	if errClass == nil && errText == "" {
		require.NoError(t, err)
	}
}
