// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
)

func TestChoreAndWorkerIntegration(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable reputation write cache so changes are immediate
				config.Reputation.FlushInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()
		pauseQueueing(satellite)

		ul := planet.Uplinks[0]

		// Upload 2 remote files with 1 segment.
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		err := runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue

		uniqueSegments := make(map[audit.Segment]struct{})
		var segment audit.Segment
		var segmentCount int
		for {
			segment, err = queue.Next(ctx)
			if err != nil {
				break
			}
			segmentCount++
			_, ok := uniqueSegments[segment]
			require.False(t, ok, "expected unique segment in chore queue")

			uniqueSegments[segment] = struct{}{}
		}
		require.True(t, audit.ErrEmptyQueue.Has(err), "expected empty queue error, but got error %+v", err)
		require.Equal(t, 2, segmentCount)
		requireAuditQueueEmpty(ctx, t, audits.VerifyQueue)

		// Repopulate the queue for the worker.
		err = runQueueingOnce(ctx, satellite)
		require.NoError(t, err)

		// Make sure the worker processes the audit queue.
		audits.Worker.Loop.TriggerWait()
		requireAuditQueueEmpty(ctx, t, audits.VerifyQueue)
	})
}

func requireAuditQueueEmpty(ctx context.Context, t *testing.T, verifyQueue audit.VerifyQueue) {
	entry, err := verifyQueue.Next(ctx)
	require.NotNilf(t, err, "expected empty audit queue, but got entry %+v", entry)
	require.Truef(t, audit.ErrEmptyQueue.Has(err), "expected empty audit queue error, but unexpectedly got error %v", err)
	require.Empty(t, entry)
}
