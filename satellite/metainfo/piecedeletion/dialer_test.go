// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo/piecedeletion"
)

type CountedPromise struct {
	SuccessCount int64
	FailureCount int64
}

func (p *CountedPromise) Success() { atomic.AddInt64(&p.SuccessCount, 1) }
func (p *CountedPromise) Failure() { atomic.AddInt64(&p.FailureCount, 1) }

func TestDialer(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		log := zaptest.NewLogger(t)

		dialer := piecedeletion.NewDialer(log, planet.Satellites[0].Dialer, 5*time.Second, 5*time.Second, 100)
		require.NotNil(t, dialer)

		storageNode := planet.StorageNodes[0].NodeURL()

		promise, jobs := makeJobsQueue(t, 2)
		dialer.Handle(ctx, storageNode, jobs)

		require.Equal(t, int64(2), promise.SuccessCount)
		require.Equal(t, int64(0), promise.FailureCount)
	})
}

func TestDialer_DialTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		log := zaptest.NewLogger(t)

		const dialTimeout = 5 * time.Second

		rpcdial := planet.Satellites[0].Dialer
		rpcdial.DialTimeout = dialTimeout

		dialer := piecedeletion.NewDialer(log, rpcdial, 5*time.Second, 1*time.Minute, 100)
		require.NotNil(t, dialer)

		require.NoError(t, planet.StopPeer(planet.StorageNodes[0]))

		storageNode := planet.StorageNodes[0].NodeURL()

		{
			promise, jobs := makeJobsQueue(t, 1)
			// we should fail to dial in the time allocated
			start := time.Now()
			dialer.Handle(ctx, storageNode, jobs)
			failingToDial := time.Since(start)

			require.Less(t, failingToDial.Seconds(), (2 * dialTimeout).Seconds())
			require.Equal(t, int64(0), promise.SuccessCount)
			require.Equal(t, int64(1), promise.FailureCount)
		}

		{
			promise, jobs := makeJobsQueue(t, 1)

			// we should immediately return when we try to redial within 1 minute
			start := time.Now()
			dialer.Handle(ctx, storageNode, jobs)
			failingToRedial := time.Since(start)

			require.Less(t, failingToRedial.Seconds(), time.Second.Seconds())
			require.Equal(t, int64(0), promise.SuccessCount)
			require.Equal(t, int64(1), promise.FailureCount)
		}
	})
}

// we can use a random piece id, since deletion requests for already deleted pieces is expected.
func makeJobsQueue(t *testing.T, n int) (*CountedPromise, piecedeletion.Queue) {
	promise := &CountedPromise{}

	jobs := piecedeletion.NewLimitedJobs(-1)
	for i := 0; i < n; i++ {
		require.True(t, jobs.TryPush(piecedeletion.Job{
			Pieces:  []storj.PieceID{testrand.PieceID(), testrand.PieceID()},
			Resolve: promise,
		}))
	}

	return promise, jobs
}
