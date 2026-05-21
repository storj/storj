// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/metabase/changestream"
)

func TestNewCombinedPendingResult_EmptyPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewCombinedPendingResult(nil)
	})
	assert.Panics(t, func() {
		NewCombinedPendingResult([]PendingResult{})
	})
}

func TestCombinedPendingResult_Single(t *testing.T) {
	ts := time.Now()
	r := NewCombinedPendingResult([]PendingResult{
		changestream.ImmediateResult(ts),
	})

	assert.Equal(t, ts, r.Timestamp())
	assertReady(t, r)
	require.NoError(t, r.Get(context.Background()))
}

func TestCombinedPendingResult_Timestamp(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	t3 := t2.Add(time.Second)

	r := NewCombinedPendingResult([]PendingResult{
		changestream.ImmediateResult(t1),
		changestream.ImmediateResult(t2),
		changestream.ImmediateResult(t3),
	})

	assert.Equal(t, t3, r.Timestamp())
}

func TestCombinedPendingResult_ReadyWaitsForAll(t *testing.T) {
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})

	r := NewCombinedPendingResult([]PendingResult{
		&manualPendingResult{ready: ch1},
		&manualPendingResult{ready: ch2},
	})

	select {
	case <-r.Ready():
		t.Fatal("Ready should not be closed yet")
	default:
	}

	close(ch1)

	select {
	case <-r.Ready():
		t.Fatal("Ready should not be closed after only first result")
	default:
	}

	close(ch2)
	assertReady(t, r)
}

func TestCombinedPendingResult_GetReturnsFirstError(t *testing.T) {
	sentinel := errors.New("publish failed")

	r := NewCombinedPendingResult([]PendingResult{
		changestream.ImmediateResult(time.Now()),
		&manualPendingResult{ready: closedForTest(), err: sentinel},
		changestream.ImmediateResult(time.Now()),
	})

	err := r.Get(context.Background())
	require.ErrorIs(t, err, sentinel)
}

func TestCombinedPendingResult_GetNoError(t *testing.T) {
	r := NewCombinedPendingResult([]PendingResult{
		changestream.ImmediateResult(time.Now()),
		changestream.ImmediateResult(time.Now()),
		changestream.ImmediateResult(time.Now()),
	})

	require.NoError(t, r.Get(context.Background()))
}

func assertReady(t *testing.T, r PendingResult) {
	t.Helper()
	select {
	case <-r.Ready():
	case <-time.After(time.Second):
		t.Fatal("Ready channel not closed within timeout")
	}
}

func closedForTest() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

type manualPendingResult struct {
	ready chan struct{}
	err   error
}

func (m *manualPendingResult) Timestamp() time.Time        { return time.Time{} }
func (m *manualPendingResult) Ready() <-chan struct{}      { return m.ready }
func (m *manualPendingResult) Get(_ context.Context) error { return m.err }
