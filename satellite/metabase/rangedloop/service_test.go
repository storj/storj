// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
	"storj.io/storj/satellite/metabase/segmentloop"
)

func TestLoop(t *testing.T) {
	for _, parallelism := range []int{1, 2, 3} {
		for _, nSegments := range []int{0, 1, 2, 11} {
			for _, nObservers := range []int{0, 1, 2} {
				t.Run(
					fmt.Sprintf("par%d_seg%d_obs%d", parallelism, nSegments, nObservers),
					func(t *testing.T) {
						RunTest(t, parallelism, nSegments, nObservers)
					},
				)
			}
		}
	}
}

func RunTest(t *testing.T, parallelism int, nSegments int, nObservers int) {
	batchSize := 2
	ctx := context.Background()

	observers := []rangedloop.Observer{}
	for i := 0; i < nObservers; i++ {
		observers = append(observers, &rangedlooptest.CountObserver{})
	}

	loopService := rangedloop.NewService(
		zaptest.NewLogger(t),
		rangedloop.Config{
			BatchSize:          batchSize,
			Parellelism:        parallelism,
			AsOfSystemInterval: 0,
		},
		&rangedlooptest.MockProvider{
			Segments: make([]segmentloop.Segment, nSegments),
		},
		observers,
	)

	err := loopService.RunOnce(ctx)
	require.NoError(t, err)

	for _, observer := range observers {
		countObserver := observer.(*rangedlooptest.CountObserver)
		require.Equal(t, nSegments, countObserver.NumSegments)
	}
}
