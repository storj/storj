// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
)

// waitFor starts acquire in the background, waits until it is blocked, and
// returns a channel with its error. On success it appends name to served and
// releases the slot.
func waitFor(acquire func() (func(), error), name string, served *[]string) <-chan error {
	errc := make(chan error, 1)
	go func() {
		release, err := acquire()
		if err == nil {
			*served = append(*served, name) // serialized by the queue slot
			release()
		}
		errc <- err
	}()
	synctest.Wait()
	return errc
}

// waitForQueue is waitFor for q.Acquire.
func waitForQueue(ctx context.Context, q *readQueue, priority readPriority, name string, served *[]string) <-chan error {
	return waitFor(func() (func(), error) { return q.Acquire(ctx, priority) }, name, served)
}

func TestReadQueue_Order(t *testing.T) {
	type waiter struct {
		priority readPriority
		name     string
	}
	tests := []struct {
		name    string
		waiters []waiter
		want    []string
	}{
		{
			name: "customer before repair, newest first",
			waiters: []waiter{
				{readPriorityRepair, "repair-old"},
				{readPriorityDefault, "get-old"},
				{readPriorityRepair, "repair-new"},
				{readPriorityDefault, "get-new"},
			},
			want: []string{"get-new", "get-old", "repair-new", "repair-old"},
		},
		{
			name: "started downloads first, oldest first",
			waiters: []waiter{
				{readPriorityRepair, "repair-new"},
				{readPriorityContinue, "continue-old"},
				{readPriorityDefault, "get-new"},
				{readPriorityContinue, "continue-new"},
			},
			want: []string{"continue-old", "continue-new", "get-new", "repair-new"},
		},
		{
			name: "audits after started downloads, oldest first",
			waiters: []waiter{
				{readPriorityAudit, "audit-old"},
				{readPriorityDefault, "get-1"},
				{readPriorityContinue, "continue"},
				{readPriorityAudit, "audit-new"},
				{readPriorityDefault, "get-2"},
				{readPriorityDefault, "get-3"},
			},
			want: []string{"continue", "audit-old", "audit-new", "get-3", "get-2", "get-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				q := newReadQueue(1, 0)
				hold, err := q.Acquire(t.Context(), readPriorityDefault)
				require.NoError(t, err)

				var served []string
				var errcs []<-chan error
				for _, w := range tt.waiters {
					errcs = append(errcs, waitForQueue(t.Context(), q, w.priority, w.name, &served))
				}
				require.Empty(t, served, "reads wait while the slot is taken")

				hold()
				for _, errc := range errcs {
					require.NoError(t, <-errc)
				}
				require.Equal(t, tt.want, served)
				require.Zero(t, q.active)
			})
		})
	}
}

func TestReadQueue_CanceledWaiterLeaves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := newReadQueue(1, 0)
		hold, err := q.Acquire(t.Context(), readPriorityDefault)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		var served []string
		errc := waitForQueue(ctx, q, readPriorityRepair, "canceled", &served)

		cancel()
		require.ErrorIs(t, <-errc, context.Canceled)

		hold()
		require.Empty(t, served)
		require.Zero(t, q.active)
	})
}

func TestReadQueue_OnlyRepairTimesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := newReadQueue(1, time.Second)
		hold, err := q.Acquire(t.Context(), readPriorityDefault)
		require.NoError(t, err)

		var served []string
		start := time.Now()
		repair := waitForQueue(t.Context(), q, readPriorityRepair, "repair", &served)
		get := waitForQueue(t.Context(), q, readPriorityDefault, "get", &served)
		continued := waitForQueue(t.Context(), q, readPriorityContinue, "continue", &served)

		require.True(t, errReadQueueTimeout.Has(<-repair))
		require.Equal(t, time.Second, time.Since(start))

		time.Sleep(time.Minute)
		synctest.Wait()
		require.Empty(t, served, "customer and started repair reads keep waiting")

		hold()
		require.NoError(t, <-get)
		require.NoError(t, <-continued)
		require.Equal(t, []string{"continue", "get"}, served)
		require.Zero(t, q.active)
	})
}

func TestReadQueue_Stats(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := newReadQueue(1, 0)
		hold, err := q.Acquire(t.Context(), readPriorityDefault)
		require.NoError(t, err)

		var served []string
		errcs := []<-chan error{
			waitForQueue(t.Context(), q, readPriorityRepair, "repair-1", &served),
			waitForQueue(t.Context(), q, readPriorityRepair, "repair-2", &served),
			waitForQueue(t.Context(), q, readPriorityAudit, "audit", &served),
		}

		stats := map[string]float64{}
		q.Stats(func(key monkit.SeriesKey, field string, val float64) {
			stats[key.WithField(field)] = val
		})
		require.Equal(t, map[string]float64{
			"piece_read_queue limit":                     1,
			"piece_read_queue active":                    1,
			"piece_read_queue,priority=repair waiting":   2,
			"piece_read_queue,priority=default waiting":  0,
			"piece_read_queue,priority=audit waiting":    1,
			"piece_read_queue,priority=continue waiting": 0,
		}, stats)

		hold()
		for _, errc := range errcs {
			require.NoError(t, <-errc)
		}
	})
}

func TestReadPriorityFor(t *testing.T) {
	require.Equal(t, readPriorityDefault, readPriorityFor(pb.PieceAction_GET))
	require.Equal(t, readPriorityAudit, readPriorityFor(pb.PieceAction_GET_AUDIT))
	require.Equal(t, readPriorityRepair, readPriorityFor(pb.PieceAction_GET_REPAIR))
}

func TestReadQueueLimit(t *testing.T) {
	require.Equal(t, 0, readQueueLimit(0), "zero stays unlimited")
	require.Equal(t, 0, readQueueLimit(-1))
	require.Equal(t, minConcurrentPieceReads, readQueueLimit(1))
	require.Equal(t, minConcurrentPieceReads, readQueueLimit(minConcurrentPieceReads))
	require.Equal(t, 100, readQueueLimit(100))
}

func TestBeginRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		endpoint := &Endpoint{readQueue: newReadQueue(1, time.Second)}

		hold, err := endpoint.beginRead(t.Context(), readPriorityDefault)
		require.NoError(t, err)

		// a repair read that times out is Unavailable, so the repairer uses another piece.
		_, err = endpoint.beginRead(t.Context(), readPriorityRepair)
		require.Equal(t, rpcstatus.Unavailable, rpcstatus.Code(err))

		// a canceled client never waits for a slot.
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = endpoint.beginRead(canceled, readPriorityDefault)
		require.ErrorIs(t, err, context.Canceled)

		hold()
	})
}
