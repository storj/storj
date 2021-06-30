// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/accounting"
)

// TestAuditOrderLimit tests that while auditing, order limits without
// specified bucket are counted correctly for storage node audit bandwidth
// usage and the storage nodes will be paid for that.
func TestAuditOrderLimit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit

		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		now := time.Now()

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		audits.Chore.Loop.TriggerWait()
		queue := audits.Queues.Fetch()
		queueSegment, err := queue.Next()
		require.NoError(t, err)
		require.False(t, queueSegment.StreamID.IsZero())

		_, err = audits.Verifier.Reverify(ctx, queueSegment)
		require.NoError(t, err)

		report, err := audits.Verifier.Verify(ctx, queueSegment, nil)
		require.NoError(t, err)

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.SendOrders(ctx, now.Add(24*time.Hour))
		}

		auditSettled := make(map[storj.NodeID]uint64)
		err = satellite.DB.StoragenodeAccounting().GetBandwidthSince(ctx, time.Time{}, func(c context.Context, sbr *accounting.StoragenodeBandwidthRollup) error {
			if sbr.Action == uint(pb.PieceAction_GET_AUDIT) {
				auditSettled[sbr.NodeID] += sbr.Settled
			}
			return nil
		})
		require.NoError(t, err)

		for _, nodeID := range report.Successes {
			require.NotZero(t, auditSettled[nodeID])
		}
	})
}
