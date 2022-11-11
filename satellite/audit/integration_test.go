// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/audit"
)

func TestChoreAndWorkerIntegration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]

		// Upload 2 remote files with 1 segment.
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		audits.Chore.Loop.TriggerWait()
		queue := audits.VerifyQueue

		uniqueSegments := make(map[audit.Segment]struct{})
		var err error
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
		audits.Chore.Loop.TriggerWait()

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
