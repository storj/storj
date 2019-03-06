// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"context"
	"testing"
	"time"

	"storj.io/storj/internal/sync2"
)

func TestSleep(t *testing.T) {
	t.Parallel()

	const sleepError = time.Second / 2 // should be larger than most system error with regards to sleep

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	if !sync2.Sleep(ctx, time.Second) {
		t.Error("expected true as result")
	}
	if time.Since(start) < time.Second-sleepError {
		t.Error("sleep took too little time")
	}
}

func TestSleep_Cancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if sync2.Sleep(ctx, 5*time.Second) {
		t.Error("expected false as result")
	}
	if time.Since(start) > time.Second {
		t.Error("sleep took too long")
	}
}
