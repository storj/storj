// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"encoding/binary"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/overlay"
)

// TestManageDoesNotSpinWhenDisabled verifies that a CachingDB configured with a
// zero FlushInterval (which disables periodic flushing) does not busy loop.
// Without the guard in Manage, the sync timer is rearmed with a zero duration on
// every iteration, which burns a full CPU core.
func TestManageDoesNotSpinWhenDisabled(t *testing.T) {
	cdb := NewCachingDB(zaptest.NewLogger(t), nil, Config{FlushInterval: 0})

	var nowCalls atomic.Int64
	cdb.nowFunc = func() time.Time {
		nowCalls.Add(1)
		return time.Now()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	managed := make(chan error, 1)
	go func() { managed <- cdb.Manage(ctx) }()

	// Manage must keep servicing on-demand sync requests even with periodic
	// flushing disabled. RequestSync blocks until Manage picks it up, so this
	// also proves the loop is still alive.
	syncCtx, syncCancel := context.WithTimeout(ctx, 10*time.Second)
	defer syncCancel()
	require.NoError(t, cdb.RequestSync(syncCtx, testrand.NodeID()))

	// Each pass through the loop calls nowFunc, so a spinning Manage racks up a
	// huge count here. Only the RequestSync above should have needed one.
	require.LessOrEqual(t, nowCalls.Load(), int64(1), "Manage busy looped while periodic flushing was disabled")

	cancel()
	require.ErrorIs(t, <-managed, context.Canceled)
}

func TestNextTimeForSync(t *testing.T) {
	var zeroesID storj.NodeID
	binary.BigEndian.PutUint64(zeroesID[:8], 0) // unnecessary, but for clarity

	var halfwayID storj.NodeID
	binary.BigEndian.PutUint64(halfwayID[:8], 1<<63)

	var quarterwayID storj.NodeID
	binary.BigEndian.PutUint64(quarterwayID[:8], 1<<62)

	const (
		zeroOffset       = 0
		halfwayOffset    = 1 << 63
		quarterwayOffset = 1 << 62
	)

	startOfHour := time.Now().Truncate(time.Hour)
	now := startOfHour.Add(15 * time.Minute)

	nextTime := nextTimeForSync(zeroOffset, halfwayID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(30*time.Minute), nextTime, time.Second)

	nextTime = nextTimeForSync(halfwayOffset, zeroesID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(30*time.Minute), nextTime, time.Second)

	nextTime = nextTimeForSync(zeroOffset, zeroesID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(time.Hour), nextTime, time.Second)

	nextTime = nextTimeForSync(halfwayOffset, halfwayID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(time.Hour), nextTime, time.Second)

	nextTime = nextTimeForSync(quarterwayOffset, halfwayID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(45*time.Minute), nextTime, time.Second)
}

func TestSelectDB(t *testing.T) {
	log := zaptest.NewLogger(t)
	direct := &stubDirectDB{}

	t.Run("cache enabled", func(t *testing.T) {
		config := Config{FlushInterval: time.Hour}
		cachingDB := NewCachingDB(log, direct, config)
		require.Same(t, cachingDB, SelectDB(cachingDB, direct, config))
	})

	t.Run("cache disabled by zero flush interval", func(t *testing.T) {
		config := Config{FlushInterval: 0}
		cachingDB := NewCachingDB(log, direct, config)
		require.Same(t, direct, SelectDB(cachingDB, direct, config))
	})
}

// stubDirectDB is a do-nothing DirectDB, used to check which implementation
// SelectDB hands back.
type stubDirectDB struct{}

func (*stubDirectDB) Update(context.Context, UpdateRequest, time.Time) (*Info, error) {
	return nil, nil
}

func (*stubDirectDB) Get(context.Context, storj.NodeID) (*Info, error) { return nil, nil }

func (*stubDirectDB) ApplyUpdates(context.Context, storj.NodeID, Mutations, Config, time.Time) (*Info, error) {
	return nil, nil
}

func (*stubDirectDB) UnsuspendNodeUnknownAudit(context.Context, storj.NodeID) error { return nil }

func (*stubDirectDB) DisqualifyNode(context.Context, storj.NodeID, time.Time, overlay.DisqualificationReason) error {
	return nil
}

func (*stubDirectDB) SuspendNodeUnknownAudit(context.Context, storj.NodeID, time.Time) error {
	return nil
}

func requireInDeltaTime(t *testing.T, expected time.Time, actual time.Time, delta time.Duration) {
	if delta < 0 {
		delta = -delta
	}
	require.Falsef(t, actual.Before(expected.Add(-delta)), "%s is not within %s of %s", actual, delta, expected)
	require.Falsef(t, actual.After(expected.Add(delta)), "%s is not within %s of %s", actual, delta, expected)
}
