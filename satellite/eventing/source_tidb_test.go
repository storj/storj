// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
)

var errTest = errors.New("test error")

func TestOutboxDrainer(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	makeResult := func() *manualPendingResult {
		return &manualPendingResult{ready: make(chan struct{})}
	}
	confirm := func(r *manualPendingResult) {
		close(r.ready)
	}

	t.Run("drainReady harvests only confirmed entries", func(t *testing.T) {
		d := newOutboxDrainer()
		r1 := makeResult()
		r2 := makeResult()
		r3 := makeResult()

		d.add(1, r1)
		d.add(2, r2)
		d.add(3, r3)

		confirm(r1)
		confirm(r3)

		ids := d.drainReady()
		require.ElementsMatch(t, []int64{1, 3}, ids)
		require.Len(t, d.pending, 1)
		require.Equal(t, int64(2), d.pending[0].id)
	})

	t.Run("drainReady returns nil when nothing confirmed", func(t *testing.T) {
		d := newOutboxDrainer()
		r1 := makeResult()
		d.add(1, r1)

		ids := d.drainReady()
		require.Nil(t, ids)
		require.Len(t, d.pending, 1)
	})

	t.Run("drainOldest blocks until oldest confirmed", func(t *testing.T) {
		d := newOutboxDrainer()
		r1 := makeResult()
		r2 := makeResult()
		d.add(1, r1)
		d.add(2, r2)

		ctx.Go(func() error {
			time.Sleep(10 * time.Millisecond)
			confirm(r1)
			return nil
		})

		id, err := d.drainOldest(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(1), id)
		require.Len(t, d.pending, 1)
		require.Equal(t, int64(2), d.pending[0].id)
	})

	t.Run("drainOldest returns error on failed result", func(t *testing.T) {
		d := newOutboxDrainer()
		r := &manualPendingResult{ready: make(chan struct{}), err: errTest}
		d.add(1, r)
		confirm(r)

		_, err := d.drainOldest(ctx)
		require.ErrorIs(t, err, errTest)
	})

	t.Run("drainOldest panics on empty drainer", func(t *testing.T) {
		d := newOutboxDrainer()
		require.Panics(t, func() {
			_, _ = d.drainOldest(ctx) //nolint:errcheck
		})
	})

	t.Run("drainOldest respects context cancellation", func(t *testing.T) {
		d := newOutboxDrainer()
		r := makeResult()
		d.add(1, r)

		cancelCtx, cancel := context.WithCancel(ctx)
		ctx.Go(func() error {
			time.Sleep(10 * time.Millisecond)
			cancel()
			return nil
		})

		_, err := d.drainOldest(cancelCtx)
		require.ErrorIs(t, err, context.Canceled)
	})
}
