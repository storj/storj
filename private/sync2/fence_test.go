// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/private/sync2"
	"storj.io/storj/private/testcontext"
)

func TestFence(t *testing.T) {
	t.Parallel()

	ctx := testcontext.NewWithTimeout(t, 30*time.Second)
	defer ctx.Cleanup()

	var group errgroup.Group
	var fence sync2.Fence
	var done int32

	for i := 0; i < 10; i++ {
		group.Go(func() error {
			if !fence.Wait(ctx) {
				return errors.New("got false from Wait")
			}
			if atomic.LoadInt32(&done) == 0 {
				return errors.New("fence not yet released")
			}
			return nil
		})
	}

	// wait a bit for all goroutines to hit the fence
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		group.Go(func() error {
			atomic.StoreInt32(&done, 1)
			fence.Release()
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestFence_ContextCancel(t *testing.T) {
	t.Parallel()

	tctx := testcontext.NewWithTimeout(t, 30*time.Second)
	defer tctx.Cleanup()

	ctx, cancel := context.WithCancel(tctx)

	var group errgroup.Group
	var fence sync2.Fence

	for i := 0; i < 10; i++ {
		group.Go(func() error {
			if fence.Wait(ctx) {
				return errors.New("got true from Wait")
			}
			return nil
		})
	}

	// wait a bit for all goroutines to hit the fence
	time.Sleep(100 * time.Millisecond)

	cancel()

	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
}
