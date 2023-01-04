// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
	"storj.io/storj/satellite/metabase/segmentloop"
)

func TestLoopCount(t *testing.T) {
	for _, parallelism := range []int{1, 2, 3} {
		for _, nSegments := range []int{0, 1, 2, 11} {
			for _, nObservers := range []int{0, 1, 2} {
				t.Run(
					fmt.Sprintf("par%d_seg%d_obs%d", parallelism, nSegments, nObservers),
					func(t *testing.T) {
						runCountTest(t, parallelism, nSegments, nObservers)
					},
				)
			}
		}
	}
}

func runCountTest(t *testing.T, parallelism int, nSegments int, nObservers int) {
	batchSize := 2
	ctx := testcontext.New(t)

	observers := []rangedloop.Observer{}
	for i := 0; i < nObservers; i++ {
		observers = append(observers, &rangedlooptest.CountObserver{})
	}

	loopService := rangedloop.NewService(
		zaptest.NewLogger(t),
		rangedloop.Config{
			BatchSize:   batchSize,
			Parallelism: parallelism,
		},
		&rangedlooptest.RangeSplitter{
			Segments: make([]segmentloop.Segment, nSegments),
		},
		observers,
	)

	observerDurations, err := loopService.RunOnce(ctx)
	require.NoError(t, err)
	require.Len(t, observerDurations, nObservers)

	for _, observer := range observers {
		countObserver := observer.(*rangedlooptest.CountObserver)
		require.Equal(t, nSegments, countObserver.NumSegments)
	}
}

func TestLoopDuration(t *testing.T) {
	t.Skip("Flaky test because it validates concurrency by measuring time")

	nSegments := 8
	nObservers := 2
	parallelism := 4
	batchSize := 2
	sleepIncrement := time.Millisecond * 10

	ctx := testcontext.New(t)

	observers := []rangedloop.Observer{}
	for i := 0; i < nObservers; i++ {
		observers = append(observers, &rangedlooptest.SleepObserver{
			Duration: sleepIncrement,
		})
	}

	segments := []segmentloop.Segment{}
	for i := 0; i < nSegments; i++ {
		streamId, err := uuid.FromBytes([]byte{byte(i), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		require.NoError(t, err)
		segments = append(segments, segmentloop.Segment{
			StreamID: streamId,
		})
	}

	loopService := rangedloop.NewService(
		zaptest.NewLogger(t),
		rangedloop.Config{
			BatchSize:   batchSize,
			Parallelism: parallelism,
		},
		&rangedlooptest.RangeSplitter{
			Segments: segments,
		},
		observers,
	)

	start := time.Now()
	observerDurations, err := loopService.RunOnce(ctx)
	require.NoError(t, err)

	duration := time.Since(start)
	expectedDuration := time.Duration(int64(nSegments) * int64(sleepIncrement) * int64(nObservers) / int64(parallelism))
	require.Equal(t, expectedDuration, duration.Truncate(sleepIncrement))

	require.Len(t, observerDurations, nObservers)
	for _, observerDuration := range observerDurations {
		expectedSleep := time.Duration(int64(nSegments) * int64(sleepIncrement))
		require.Equal(t, expectedSleep, observerDuration.Duration.Round(sleepIncrement))
	}
}
