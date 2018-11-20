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

func TestTimeout(realTest *testing.T) {
	ok := testing.RunTests(nil, []testing.InternalTest{{
		Name: "TimeoutFailure",
		F: func(t *testing.T) {
			ctx := testcontext.NewWithTimeout(t, 50*time.Millisecond)
			defer ctx.Cleanup()

			ctx.Go(func() error {
				time.Sleep(time.Second)
				return nil
			})
		},
	}})

	if ok {
		realTest.Error("test should have failed")
	}
}
