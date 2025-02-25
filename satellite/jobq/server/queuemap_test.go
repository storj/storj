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
		return jobqueue.NewQueue(log.Named(fmt.Sprintf("queue-for-placement-%d", pc)), time.Hour, 100, 10)
	})
	defer qm.StopAll()
	err := qm.AddQueue(42)
	require.NoError(t, err)
	q := qm.GetQueue(42)
	require.NotNil(t, q)
	repairLen, retryLen := q.Len()
	require.Zero(t, repairLen)
	require.Zero(t, retryLen)

	q = qm.GetQueue(43)
	require.Nil(t, q)

	qs := qm.GetAllQueues()
	require.Len(t, qs, 1)
	require.NotNil(t, qs[42])
	clear(qs)
	q = qm.GetQueue(42)
	require.NotNil(t, q)

	var group errgroup.Group
	for i := 0; i < 30; i++ {
		i := i
		group.Go(func() error {
			err := qm.AddQueue(storj.PlacementConstraint(i))
			if err != nil {
				return errs.Wrap(err)
			}
			q := qm.GetQueue(storj.PlacementConstraint(i))
			if q == nil {
				return errs.New("queue for placement %d not found", i)
			}
			qs := qm.GetAllQueues()
			if len(qs) < 2 {
				return errs.New("returned queue map has too few queues (%d)", len(qs))
			}
			err = qm.DestroyQueue(storj.PlacementConstraint(i))
			if err != nil {
				return errs.Wrap(err)
			}
			q = qm.GetQueue(storj.PlacementConstraint(i))
			if q != nil {
				return errs.New("queue for placement %d not destroyed", i)
			}
			return nil
		})
	}
	err = group.Wait()
	require.NoError(t, err)

	qs = qm.GetAllQueues()
	require.Len(t, qs, 1)
	require.NotNil(t, qs[42])
}
