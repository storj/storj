// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
)

func TestSafeLoop(t *testing.T) {
	t.Skip("this test depends on time, and safe only on environments which is not overloaded, not safe for Jenkins")
	runningCount := atomic.Int64{}
	finishedCount := atomic.Int64{}

	log := zaptest.NewLogger(t)
	a := &mockAvailability{}
	cfg := SafeLoopConfig{
		CheckingPeriod: 100 * time.Millisecond,
		RunPeriod:      3 * time.Second,
	}
	l := NewSafeLoop(log, []Enablement{a}, cfg)
	ctx := testcontext.New(t)
	result := make(chan error)
	go func() {

		_ = l.RunSafe(ctx, func(ctx context.Context) (err error) {

			runningCount.Add(1)
			fmt.Println("result")
			select {
			case err = <-result:
				// finished manually from this test
			case <-ctx.Done():
				// context is cancelled due to availability check

			}

			fmt.Println("done")
			finishedCount.Add(1)
			return err
		})
	}()

	time.Sleep(100 * time.Millisecond)

	// nothing is result, as we didn't enable
	require.Equal(t, int64(0), runningCount.Load())
	time.Sleep(100 * time.Millisecond)

	// enable result the function
	a.available.Store(true)
	time.Sleep(100 * time.Millisecond)

	count := runningCount.Load()
	require.Greater(t, count, int64(0))

	time.Sleep(100 * time.Millisecond)
	// no new run
	require.Equal(t, count, runningCount.Load())
	// one instance is running
	require.Equal(t, runningCount.Load(), finishedCount.Load()+1)

	// finish the runner
	result <- nil

	time.Sleep(1000 * time.Millisecond)
	// no new run, as we should wait 3 seconds for the next run
	require.Equal(t, count, runningCount.Load())

	// wait until next iteration
	require.Eventually(t, func() bool {
		return count < runningCount.Load()
	}, 5*time.Second, 100*time.Millisecond)

	// it's running
	require.Equal(t, runningCount.Load(), finishedCount.Load()+1)

	// let's stop it
	a.available.Store(false)

	require.Eventually(t, func() bool {
		// should be stopped by now
		return runningCount.Load() == finishedCount.Load()
	}, 5*time.Second, 100*time.Millisecond)

}

type mockAvailability struct {
	available atomic.Bool
}

func (m *mockAvailability) Enabled() (bool, error) {
	return m.available.Load(), nil
}

var _ Enablement = &mockAvailability{}
