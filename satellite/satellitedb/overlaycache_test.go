// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
)

func TestUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeA := planet.StorageNodes[0]
		nodeB := planet.StorageNodes[1]
		nodeA.Contact.Chore.Pause(ctx)
		nodeB.Contact.Chore.Pause(ctx)

		cache := planet.Satellites[0].DB.OverlayCache()
		numAudits := int64(2)
		numUptimes := int64(3)

		// nodes automatically start with 2 uptimes from testplanet startup
		// nodeA: 1 audit, 2 uptime -> unvetted
		updateReq := &overlay.UpdateRequest{
			NodeID:                    nodeA.ID(),
			AuditOutcome:              overlay.AuditOffline,
			AuditsRequiredForVetting:  numAudits,
			UptimesRequiredForVetting: numUptimes,
			AuditHistory:              testAuditHistoryConfig(),
		}
		nodeStats, err := cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt)
		assert.EqualValues(t, 1, nodeStats.AuditCount)
		assert.EqualValues(t, 2, nodeStats.UptimeCount)

		// nodeA: 2 audits, 2 uptimes -> unvetted
		updateReq.NodeID = nodeA.ID()
		updateReq.AuditOutcome = overlay.AuditOffline
		nodeStats, err = cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt)
		assert.EqualValues(t, 2, nodeStats.AuditCount)
		assert.EqualValues(t, 2, nodeStats.UptimeCount)

		// nodeA: 3 audits, 3 uptimes -> vetted
		updateReq.NodeID = nodeA.ID()
		updateReq.AuditOutcome = overlay.AuditSuccess
		nodeStats, err = cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats.VettedAt)
		assert.EqualValues(t, 3, nodeStats.AuditCount)
		assert.EqualValues(t, 3, nodeStats.UptimeCount)

		// nodeB: 1 audit, 3 uptimes -> unvetted
		updateReq.NodeID = nodeB.ID()
		updateReq.AuditOutcome = overlay.AuditSuccess
		nodeStats, err = cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt)
		assert.EqualValues(t, 1, nodeStats.AuditCount)
		assert.EqualValues(t, 3, nodeStats.UptimeCount)

		// nodeB: 2 audits, 3 uptimes -> vetted
		updateReq.NodeID = nodeB.ID()
		updateReq.AuditOutcome = overlay.AuditOffline
		nodeStats, err = cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats.VettedAt)
		assert.EqualValues(t, 2, nodeStats.AuditCount)
		assert.EqualValues(t, 3, nodeStats.UptimeCount)

		// Don't overwrite node b's vetted_at timestamp
		updateReq.NodeID = nodeB.ID()
		updateReq.AuditOutcome = overlay.AuditSuccess
		nodeStats2, err := cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats2.VettedAt)
		assert.Equal(t, nodeStats.VettedAt, nodeStats2.VettedAt)
		assert.EqualValues(t, 3, nodeStats2.AuditCount)
		assert.EqualValues(t, 4, nodeStats2.UptimeCount)
	})
}

func TestBatchUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeA := planet.StorageNodes[0]
		nodeB := planet.StorageNodes[1]
		nodeA.Contact.Chore.Pause(ctx)
		nodeB.Contact.Chore.Pause(ctx)

		cache := planet.Satellites[0].DB.OverlayCache()
		numAudits := int64(2)
		numUptimes := int64(3)
		batchSize := 2

		// nodes automatically start with 2 uptimes from testplanet startup
		// both nodeA and nodeB unvetted
		updateReqA := &overlay.UpdateRequest{NodeID: nodeA.ID(), AuditOutcome: overlay.AuditOffline, AuditsRequiredForVetting: numAudits, UptimesRequiredForVetting: numUptimes, AuditHistory: testAuditHistoryConfig()}
		updateReqB := &overlay.UpdateRequest{NodeID: nodeB.ID(), AuditOutcome: overlay.AuditSuccess, AuditsRequiredForVetting: numAudits, UptimesRequiredForVetting: numUptimes, AuditHistory: testAuditHistoryConfig()}
		updateReqs := []*overlay.UpdateRequest{updateReqA, updateReqB}
		failed, err := cache.BatchUpdateStats(ctx, updateReqs, batchSize, time.Now())
		require.NoError(t, err)
		assert.Len(t, failed, 0)

		nA, err := cache.Get(ctx, nodeA.ID())
		require.NoError(t, err)
		assert.Nil(t, nA.Reputation.VettedAt)
		assert.EqualValues(t, 1, nA.Reputation.AuditCount)
		assert.EqualValues(t, 2, nA.Reputation.UptimeCount)

		nB, err := cache.Get(ctx, nodeB.ID())
		require.NoError(t, err)
		assert.Nil(t, nB.Reputation.VettedAt)
		assert.EqualValues(t, 1, nB.Reputation.AuditCount)
		assert.EqualValues(t, 3, nB.Reputation.UptimeCount)

		// nodeA unvetted, nodeB vetted
		updateReqA = &overlay.UpdateRequest{NodeID: nodeA.ID(), AuditOutcome: overlay.AuditOffline, AuditsRequiredForVetting: numAudits, UptimesRequiredForVetting: numUptimes, AuditHistory: testAuditHistoryConfig()}
		updateReqB = &overlay.UpdateRequest{NodeID: nodeB.ID(), AuditOutcome: overlay.AuditFailure, AuditsRequiredForVetting: numAudits, UptimesRequiredForVetting: numUptimes, AuditHistory: testAuditHistoryConfig()}
		updateReqs = []*overlay.UpdateRequest{updateReqA, updateReqB}
		failed, err = cache.BatchUpdateStats(ctx, updateReqs, batchSize, time.Now())
		require.NoError(t, err)
		assert.Len(t, failed, 0)

		nA, err = cache.Get(ctx, nodeA.ID())
		require.NoError(t, err)
		assert.Nil(t, nA.Reputation.VettedAt)
		assert.EqualValues(t, 2, nA.Reputation.AuditCount)
		assert.EqualValues(t, 2, nA.Reputation.UptimeCount)

		nB, err = cache.Get(ctx, nodeB.ID())
		require.NoError(t, err)
		assert.NotNil(t, nB.Reputation.VettedAt)
		assert.EqualValues(t, 2, nB.Reputation.AuditCount)
		assert.EqualValues(t, 4, nB.Reputation.UptimeCount)

		// both nodeA and nodeB vetted (don't overwrite timestamp)
		updateReqA = &overlay.UpdateRequest{NodeID: nodeA.ID(), AuditOutcome: overlay.AuditSuccess, AuditsRequiredForVetting: numAudits, UptimesRequiredForVetting: numUptimes, AuditHistory: testAuditHistoryConfig()}
		updateReqB = &overlay.UpdateRequest{NodeID: nodeB.ID(), AuditOutcome: overlay.AuditSuccess, AuditsRequiredForVetting: numAudits, UptimesRequiredForVetting: numUptimes, AuditHistory: testAuditHistoryConfig()}
		updateReqs = []*overlay.UpdateRequest{updateReqA, updateReqB}
		failed, err = cache.BatchUpdateStats(ctx, updateReqs, batchSize, time.Now())
		require.NoError(t, err)
		assert.Len(t, failed, 0)

		nA, err = cache.Get(ctx, nodeA.ID())
		require.NoError(t, err)
		assert.NotNil(t, nA.Reputation.VettedAt)
		assert.EqualValues(t, 3, nA.Reputation.AuditCount)
		assert.EqualValues(t, 3, nA.Reputation.UptimeCount)

		nB2, err := cache.Get(ctx, nodeB.ID())
		require.NoError(t, err)
		assert.NotNil(t, nB2.Reputation.VettedAt)
		assert.Equal(t, nB.Reputation.VettedAt, nB2.Reputation.VettedAt)
		assert.EqualValues(t, 3, nB2.Reputation.AuditCount)
		assert.EqualValues(t, 5, nB2.Reputation.UptimeCount)
	})
}

// returns an AuditHistoryConfig with sensible test values.
func testAuditHistoryConfig() overlay.AuditHistoryConfig {
	return overlay.AuditHistoryConfig{
		WindowSize:       time.Hour,
		TrackingPeriod:   time.Hour,
		GracePeriod:      time.Hour,
		OfflineThreshold: 0,
	}
}
