// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext_test

import (
	"testing"
	"time"

	"storj.io/storj/internal/testcontext"
)

func TestBasic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ctx.Go(func() error {
		time.Sleep(time.Millisecond)
		return nil
	})

	t.Log(ctx.Dir("a", "b", "c"))
	t.Log(ctx.File("a", "w", "c.txt"))
}

func TestFailure(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 1*time.Second)
	defer ctx.Cleanup()

	ctx.Go(func() error {
		time.Sleep(10 * time.Second)
		return nil
	})
}
