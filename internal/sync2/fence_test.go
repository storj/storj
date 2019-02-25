// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
)

func TestFence(t *testing.T) {
	t.Parallel()

	var group errgroup.Group
	var fence sync2.Fence
	var done int32

	for i := 0; i < 10; i++ {
		group.Go(func() error {
			fence.Wait()
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
