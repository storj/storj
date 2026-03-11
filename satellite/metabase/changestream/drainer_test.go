// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testResult is a controllable PendingResult for testing.
type testResult struct {
	timestamp time.Time
	ready     chan struct{}
	err       error
}

func newTestResult(ts time.Time) *testResult {
	return &testResult{
		timestamp: ts,
		ready:     make(chan struct{}),
	}
}

func (r *testResult) resolve(err error) {
	r.err = err
	close(r.ready)
}

func (r *testResult) Timestamp() time.Time   { return r.timestamp }
func (r *testResult) Ready() <-chan struct{} { return r.ready }
func (r *testResult) Get(ctx context.Context) error {
	select {
	case <-r.ready:
		return r.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// testWatermarks records watermark updates for assertions.
type testWatermarks struct {
	mu         sync.Mutex
	watermarks []time.Time
}

func (w *testWatermarks) UpdatePartitionWatermark(_ string, t time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.watermarks = append(w.watermarks, t)
}

func (w *testWatermarks) last() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.watermarks) == 0 {
		return time.Time{}
	}
	return w.watermarks[len(w.watermarks)-1]
}

func newTestDrainer(wm *testWatermarks) *partitionDrainer {
	return &partitionDrainer{
		log:            zap.NewNop(),
		feedName:       "test-feed",
		partitionToken: "test-partition",
		watermarks:     wm,
	}
}

func TestDrainReady_DrainsOnlyReadyResults(t *testing.T) {
	ctx := context.Background()

	ts1 := time.Unix(1, 0)
	ts2 := time.Unix(2, 0)
	ts3 := time.Unix(3, 0)

	r1 := newTestResult(ts1)
	r2 := newTestResult(ts2)
	r3 := newTestResult(ts3)

	r1.resolve(nil)
	r2.resolve(nil)
	// r3 is not resolved — still in-flight

	wm := &testWatermarks{}
	d := newTestDrainer(wm)
	d.pending = []PendingResult{r1, r2, r3}

	require.NoError(t, d.drainReady(ctx))

	d.mu.Lock()
	remaining := len(d.pending)
	d.mu.Unlock()

	require.Equal(t, 1, remaining, "only r3 should remain")
	require.Equal(t, ts2, wm.last(), "watermark should advance to ts2")
}

func TestDrainReady_StopsAtFirstNotReady(t *testing.T) {
	ctx := context.Background()

	r1 := newTestResult(time.Unix(1, 0))
	r2 := newTestResult(time.Unix(2, 0))
	r3 := newTestResult(time.Unix(3, 0))

	// r1 not ready, r2 and r3 are — should stop at r1 without draining anything
	r2.resolve(nil)
	r3.resolve(nil)

	wm := &testWatermarks{}
	d := newTestDrainer(wm)
	d.pending = []PendingResult{r1, r2, r3}

	require.NoError(t, d.drainReady(ctx))

	d.mu.Lock()
	remaining := len(d.pending)
	d.mu.Unlock()

	require.Equal(t, 3, remaining, "nothing should be drained")
	require.True(t, wm.last().IsZero(), "watermark should not advance")
}

func TestDrainReady_PartialWatermarkOnError(t *testing.T) {
	ctx := context.Background()

	ts1 := time.Unix(1, 0)
	ts2 := time.Unix(2, 0)
	ts3 := time.Unix(3, 0)

	r1 := newTestResult(ts1)
	r2 := newTestResult(ts2)
	r3 := newTestResult(ts3)

	r1.resolve(nil)
	r2.resolve(nil)
	r3.resolve(errors.New("publish failed"))

	wm := &testWatermarks{}
	d := newTestDrainer(wm)
	d.pending = []PendingResult{r1, r2, r3}

	require.Error(t, d.drainReady(ctx))
	require.Equal(t, ts2, wm.last(), "watermark should advance to ts2 despite error on r3")

	d.mu.Lock()
	remaining := len(d.pending)
	d.mu.Unlock()
	require.Equal(t, 0, remaining, "pending should be empty — failed result removed to prevent background re-processing")
}

func TestDrainReady_PropagatesError(t *testing.T) {
	ctx := context.Background()

	r1 := newTestResult(time.Unix(1, 0))
	r1.resolve(errors.New("publish failed"))

	wm := &testWatermarks{}
	d := newTestDrainer(wm)
	d.pending = []PendingResult{r1}

	require.Error(t, d.drainReady(ctx))
}

func TestDrainAll_DrainsAllBlocking(t *testing.T) {
	ctx := context.Background()

	ts1 := time.Unix(1, 0)
	ts2 := time.Unix(2, 0)

	r1 := newTestResult(ts1)
	r2 := newTestResult(ts2)

	// Resolve asynchronously to exercise blocking behaviour.
	go func() {
		time.Sleep(10 * time.Millisecond)
		r1.resolve(nil)
		r2.resolve(nil)
	}()

	wm := &testWatermarks{}
	d := newTestDrainer(wm)
	d.pending = []PendingResult{r1, r2}

	require.NoError(t, d.drainAll(ctx))

	d.mu.Lock()
	remaining := len(d.pending)
	d.mu.Unlock()

	require.Equal(t, 0, remaining)
	require.Equal(t, ts2, wm.last())
}

func TestAdd_BackpressureTriggersDrainAll(t *testing.T) {
	ctx := context.Background()

	wm := &testWatermarks{}
	d := newTestDrainer(wm)

	// Fill up to pendingDrainSize-1 with already-resolved results.
	for i := range pendingDrainSize - 1 {
		r := newTestResult(time.Unix(int64(i), 0))
		r.resolve(nil)
		d.pending = append(d.pending, r)
	}

	// Adding one more should push len to pendingDrainSize and trigger drainAll.
	last := newTestResult(time.Unix(int64(pendingDrainSize), 0))
	last.resolve(nil)

	require.NoError(t, d.add(ctx, last))

	d.mu.Lock()
	remaining := len(d.pending)
	d.mu.Unlock()

	require.Equal(t, 0, remaining, "drainAll should have cleared all pending")
}

func TestDrainAll_PartialWatermarkOnError(t *testing.T) {
	ctx := context.Background()

	ts1 := time.Unix(1, 0)
	ts2 := time.Unix(2, 0)
	ts3 := time.Unix(3, 0)

	r1 := newTestResult(ts1)
	r2 := newTestResult(ts2)
	r3 := newTestResult(ts3)

	r1.resolve(nil)
	r2.resolve(nil)
	r3.resolve(errors.New("publish failed"))

	wm := &testWatermarks{}
	d := newTestDrainer(wm)
	d.pending = []PendingResult{r1, r2, r3}

	require.Error(t, d.drainAll(ctx))
	require.Equal(t, ts2, wm.last(), "watermark should advance to ts2 despite error on r3")

	d.mu.Lock()
	remaining := len(d.pending)
	d.mu.Unlock()
	require.Equal(t, 0, remaining, "pending should be empty — error causes process exit, no retry")
}

func TestBackground_DrainsWhileIdle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wm := &testWatermarks{}
	d := newTestDrainer(wm)

	ts := time.Unix(42, 0)
	r := newTestResult(ts)
	r.resolve(nil)

	d.mu.Lock()
	d.pending = append(d.pending, r)
	d.mu.Unlock()

	// Start a background ticker directly on our drainer.
	bgCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		ticker := time.NewTicker(drainReadyInterval)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				_ = d.drainReady(bgCtx)
			}
		}
	}()

	require.Eventually(t, func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return len(d.pending) == 0
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.Equal(t, ts, wm.last())
}
