// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package usedserials_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/piecestore/usedserials"
)

// TestChore verifies that the chore removes expired serial numbers while
// leaving active ones intact.
func TestChore(t *testing.T) {
	ctx := testcontext.New(t)

	table := usedserials.NewTable(memory.MiB)
	chore := usedserials.NewChore(zaptest.NewLogger(t), table, usedserials.Config{
		Interval:              time.Hour,
		ExpirationGracePeriod: 30 * time.Minute,
	})

	// Run must be started before Pause, since Pause blocks until the cycle is running.
	ctx.Go(func() error {
		return chore.Run(ctx)
	})
	chore.Loop.Pause()
	defer ctx.Check(chore.Close)

	satID := testrand.NodeID()
	now := time.Now()

	// add a serial that is already expired (2h in the past so the bucket is
	// always older than the 30-minute grace period, regardless of clock position)
	expiredSerial := testrand.SerialNumber()
	require.NoError(t, table.Add(ctx, satID, expiredSerial, now.Add(-2*time.Hour)))

	// add a serial that is not yet expired
	activeSerial := testrand.SerialNumber()
	require.NoError(t, table.Add(ctx, satID, activeSerial, now.Add(time.Hour)))

	require.Equal(t, 2, table.Count())

	// trigger one cycle and wait for it to complete
	chore.Loop.TriggerWait()

	// expired serial should be gone, active serial should remain
	require.Equal(t, 1, table.Count())
	require.True(t, table.Exists(satID, activeSerial, now.Add(time.Hour)))
	require.False(t, table.Exists(satID, expiredSerial, now.Add(-2*time.Hour)))
}

// TestChoreMaxSize verifies that after the chore cleans up expired entries,
// the table no longer relies on random eviction to stay within its size limit.
func TestChoreMaxSize(t *testing.T) {
	ctx := testcontext.New(t)

	// cap at 48 bytes — exactly 3 full serials at 16 bytes each.
	const maxBytes = 3 * usedserials.FullSize
	table := usedserials.NewTable(maxBytes)
	chore := usedserials.NewChore(zaptest.NewLogger(t), table, usedserials.Config{
		Interval:              time.Hour,
		ExpirationGracePeriod: 30 * time.Minute,
	})

	ctx.Go(func() error {
		return chore.Run(ctx)
	})
	chore.Loop.Pause()
	defer ctx.Check(chore.Close)

	satID := testrand.NodeID()
	now := time.Now()

	// add more entries than the cap allows, all already expired.
	// random eviction keeps the in-memory footprint within maxBytes.
	for i := 0; i < 10; i++ {
		require.NoError(t, table.Add(ctx, satID, testrand.SerialNumber(), now.Add(-2*time.Hour)))
	}
	require.LessOrEqual(t, table.Count(), int(maxBytes/usedserials.FullSize))

	// after the chore runs, expired entries are removed and tracked memory drops to zero.
	chore.Loop.TriggerWait()
	require.Equal(t, 0, table.Count())

	// the table can now accept up to maxBytes worth of fresh entries
	// without resorting to random eviction.
	for i := 0; i < int(maxBytes/usedserials.FullSize); i++ {
		require.NoError(t, table.Add(ctx, satID, testrand.SerialNumber(), now.Add(time.Hour)))
		require.Equal(t, i+1, table.Count())
	}
}
