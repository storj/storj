// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
)

func TestQueues(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	queues := NewQueues()
	q := queues.Fetch()

	_, err := q.Next()
	require.True(t, ErrEmptyQueue.Has(err), "required ErrEmptyQueue error")

	testQueue1 := []storj.Path{"a", "b", "c"}
	err = queues.Push(testQueue1)
	require.NoError(t, err)
	err = queues.WaitForSwap(ctx)
	require.NoError(t, err)

	q = queues.Fetch()
	for _, expected := range testQueue1 {
		actual, err := q.Next()
		require.NoError(t, err)
		require.EqualValues(t, expected, actual)
	}

	require.Zero(t, q.Size())
}

func TestQueuesPush(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	queues := NewQueues()
	// when next queue is empty, WaitForSwap should return immediately
	testQueue1 := []storj.Path{"a", "b", "c"}
	err := queues.Push(testQueue1)
	require.NoError(t, err)
	err = queues.WaitForSwap(ctx)
	require.NoError(t, err)

	// second call to WaitForSwap should block until Fetch is called the first time
	testQueue2 := []storj.Path{"d", "e"}
	err = queues.Push(testQueue2)
	require.NoError(t, err)
	var group errgroup.Group
	group.Go(func() error {
		return queues.WaitForSwap(ctx)
	})

	q := queues.Fetch()
	for _, expected := range testQueue1 {
		actual, err := q.Next()
		require.NoError(t, err)
		require.EqualValues(t, expected, actual)
	}

	// second call to Fetch should return testQueue2
	q = queues.Fetch()
	item, err := q.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[0], item)
	require.Equal(t, len(testQueue2)-1, q.Size())

	err = group.Wait()
	require.NoError(t, err)
}

func TestQueuesPushCancel(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	queues := NewQueues()
	// when queue is empty, WaitForSwap should return immediately
	testQueue1 := []storj.Path{"a", "b", "c"}
	err := queues.Push(testQueue1)
	require.NoError(t, err)
	err = queues.WaitForSwap(ctx)
	require.NoError(t, err)

	ctxWithCancel, cancel := context.WithCancel(ctx)
	testQueue2 := []storj.Path{"d", "e"}
	err = queues.Push(testQueue2)
	require.NoError(t, err)
	var group errgroup.Group
	group.Go(func() error {
		err = queues.WaitForSwap(ctxWithCancel)
		require.True(t, errs2.IsCanceled(err))
		return nil
	})

	// make sure a concurrent call to Push fails
	err = queues.Push(testQueue2)
	require.True(t, ErrPendingQueueInProgress.Has(err))

	cancel()

	err = group.Wait()
	require.NoError(t, err)
}
