// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package server_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/storj/satellite/jobq/jobqueue"
	"storj.io/storj/satellite/jobq/server"
)

func TestQueueMap(t *testing.T) {
	log := zaptest.NewLogger(t)
	qm := server.NewQueueMap(log, func(pc storj.PlacementConstraint) (*jobqueue.Queue, error) {
		return jobqueue.NewQueue(log.Named(fmt.Sprintf("queue-for-placement-%d", pc)), time.Hour, 100, 0, 10)
	})
	defer qm.StopAll()
	q, err := qm.GetQueue(42)
	require.NoError(t, err)
	repairLen, retryLen := q.Len()
	require.Zero(t, repairLen)
	require.Zero(t, retryLen)

	q, err = qm.GetQueue(43)
	require.NoError(t, err)
	require.NotNil(t, q)

	qs := qm.GetAllQueues()
	require.Len(t, qs, 2)
	require.NotNil(t, qs[42])
	require.NotNil(t, qs[43])
	clear(qs)
	q, err = qm.GetQueue(42)
	require.NoError(t, err)
	require.NotNil(t, q)

	var group errgroup.Group
	for i := 0; i < 30; i++ {
		i := i
		group.Go(func() error {
			q, err := qm.GetQueue(storj.PlacementConstraint(i))
			if err != nil {
				return err
			}
			if q == nil {
				return errs.New("returned queue is nil")
			}
			qs := qm.GetAllQueues()
			if len(qs) < 2 {
				return errs.New("returned queue map has too few queues (%d)", len(qs))
			}
			return nil
		})
	}
	err = group.Wait()
	require.NoError(t, err)

	qs = qm.GetAllQueues()
	require.Len(t, qs, 30+2)
	require.NotNil(t, qs[42])
}
