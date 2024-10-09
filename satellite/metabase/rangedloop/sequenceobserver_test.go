// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
)

func TestSequenceObserver(t *testing.T) {
	ctx := testcontext.New(t)

	observers := make([]rangedloop.Observer, 4)
	for index := range observers {
		observers[index] = &rangedlooptest.CountObserver{}
	}

	rangeSplitter := &rangedlooptest.RangeSplitter{
		Segments: make([]rangedloop.Segment, 10),
	}

	service := rangedloop.NewService(zaptest.NewLogger(t), rangedloop.Config{
		Parallelism: 1,
		BatchSize:   1,
	}, rangeSplitter, []rangedloop.Observer{rangedloop.NewSequenceObserver(observers...)})

	for iteration := 0; iteration < 4; iteration++ {
		_, err := service.RunOnce(ctx)
		require.NoError(t, err)

		for index := range observers {
			countobserver := observers[index].(*rangedlooptest.CountObserver)
			// with each loop iteration more observers should be populated
			if iteration >= index {
				require.Equal(t, 10, countobserver.NumSegments)
			} else {
				require.Zero(t, countobserver.NumSegments)
			}
		}
	}

	rangeSplitter.Segments = make([]rangedloop.Segment, 20)

	for iteration := 0; iteration < 4; iteration++ {
		_, err := service.RunOnce(ctx)
		require.NoError(t, err)

		for index := range observers {
			countobserver := observers[index].(*rangedlooptest.CountObserver)
			// with each loop iteration more observers should be populated with latest value
			if iteration >= index {
				require.Equal(t, 20, countobserver.NumSegments)
			} else {
				require.Equal(t, 10, countobserver.NumSegments)
			}
		}
	}
}
