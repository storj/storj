// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
)

func TestQueue(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	q := &Queue{}

	_, err := q.Next()
	require.True(t, ErrEmptyQueue.Has(err), "required ErrEmptyQueue error")

	testQueue1 := []storj.Path{"a", "b", "c"}
	err = q.WaitForSwap(ctx, testQueue1)
	require.NoError(t, err)

	for _, expected := range testQueue1 {
		actual, err := q.Next()
		require.NoError(t, err)
		require.EqualValues(t, expected, actual)
	}

	require.Zero(t, q.Size())
}

func TestQueueWaitForSwap(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	q := &Queue{}
	// when queue is empty, WaitForSwap should return immediately
	testQueue1 := []storj.Path{"a", "b", "c"}
	err := q.WaitForSwap(ctx, testQueue1)
	require.NoError(t, err)

	testQueue2 := []storj.Path{"d", "e"}
	var group errgroup.Group
	group.Go(func() error {
		return q.WaitForSwap(ctx, testQueue2)
	})

	// wait for WaitForSwap to set onEmpty callback so we can test that consuming the queue frees it.
	ticker := time.NewTicker(100 * time.Millisecond)
	for range ticker.C {
		q.mu.Lock()
		if q.onEmpty != nil {
			q.mu.Unlock()
			break
		}
		q.mu.Unlock()
	}
	ticker.Stop()

	for _, expected := range testQueue1 {
		actual, err := q.Next()
		require.NoError(t, err)
		require.EqualValues(t, expected, actual)
	}

	// next call to Next() should swap queues and free WaitForSwap
	item, err := q.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[0], item)
	require.Equal(t, len(testQueue2)-1, q.Size())

	err = group.Wait()
	require.NoError(t, err)
}

func TestQueueWaitForSwapCancel(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	q := &Queue{}
	// when queue is empty, WaitForSwap should return immediately
	testQueue1 := []storj.Path{"a", "b", "c"}
	err := q.WaitForSwap(ctx, testQueue1)
	require.NoError(t, err)

	ctxWithCancel, cancel := context.WithCancel(ctx)
	testQueue2 := []storj.Path{"d", "e"}
	var group errgroup.Group
	group.Go(func() error {
		err = q.WaitForSwap(ctxWithCancel, testQueue2)
		require.True(t, errs2.IsCanceled(err))
		return nil
	})

	// wait for WaitForSwap to set onEmpty callback so we can test that canceling the context frees it.
	ticker := time.NewTicker(100 * time.Millisecond)
	for range ticker.C {
		q.mu.Lock()
		if q.onEmpty != nil {
			q.mu.Unlock()
			break
		}
		q.mu.Unlock()
	}
	ticker.Stop()

	cancel()

	err = group.Wait()
	require.NoError(t, err)
}
