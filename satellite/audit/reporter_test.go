// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/audit"
)

func TestReportPendingAudits(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		nodeID := planet.StorageNodes[0].ID()

		pending := audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           storj.NewPieceID(),
			StripeIndex:       1,
			ShareSize:         1 * memory.KiB.Int32(),
			ExpectedShareHash: pkcrypto.SHA256Hash([]byte("test")),
		}

		report := audit.Report{PendingAudits: []*audit.PendingAudit{&pending}}
		overlay := planet.Satellites[0].Overlay.Service
		containment := planet.Satellites[0].DB.Containment()
		log := planet.Satellites[0].Log.Named("reporter")

		reporter := audit.NewReporter(log, overlay, containment, 1, 3)
		failed, err := reporter.RecordAudits(ctx, &report)
		require.NoError(t, err)
		assert.Zero(t, failed)

		node, err := overlay.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.True(t, node.Contained)

		pa, err := containment.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, pending, *pa)
	})
}

func TestRecordAuditsAtLeastOnce(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Audit.Service.Close()
		require.NoError(t, err)

		nodeID := planet.StorageNodes[0].ID()

		report := audit.Report{Successes: []storj.NodeID{nodeID}}
		overlay := planet.Satellites[0].Overlay.Service
		containment := planet.Satellites[0].DB.Containment()
		log := planet.Satellites[0].Log.Named("reporter")

		// set maxRetries to 0
		reporter := audit.NewReporter(log, overlay, containment, 0, 3)

		// expect RecordAudits to try recording at least once
		failed, err := reporter.RecordAudits(ctx, &report)
		require.NoError(t, err)
		require.Zero(t, failed)

		node, err := overlay.Get(ctx, nodeID)
		require.NoError(t, err)
		require.EqualValues(t, 1, node.Reputation.AuditCount)
	})
}
