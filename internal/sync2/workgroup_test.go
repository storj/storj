// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/sync2"
)

func TestWaitGroup(t *testing.T) {
	const Wait = 2 * time.Second
	const TimeError = time.Second / 2

	var group sync2.WorkGroup

	require.True(t, group.Start())
	go func() {
		defer group.Done()
		time.Sleep(Wait)
	}()

	require.True(t, group.Go(func() {
		time.Sleep(Wait)
	}))

	start := time.Now()
	group.Wait()
	duration := time.Since(start)

	if duration < Wait-TimeError || duration > Wait+TimeError {
		t.Fatalf("waited %s instead of %s", duration, Wait)
	}
}

func TestWaitGroupClose(t *testing.T) {
	const Wait = 2 * time.Second
	const LongWait = 10 * time.Second
	const TimeError = time.Second / 2

	var group sync2.WorkGroup

	require.True(t, group.Go(func() {
		time.Sleep(Wait)
	}))

	group.Close()

	require.False(t, group.Go(func() {
		time.Sleep(LongWait)
	}))

	start := time.Now()
	group.Wait()
	duration := time.Since(start)

	if duration < Wait-TimeError || duration > LongWait-TimeError {
		t.Fatalf("waited %s instead of %s", duration, Wait)
	}
}
