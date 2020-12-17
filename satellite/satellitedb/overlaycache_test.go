// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
)

func TestDQNodesLastSeenBefore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		cache := planet.Satellites[0].Overlay.DB

		info, err := cache.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Nil(t, info.Disqualified)

		require.NoError(t, cache.DQNodesLastSeenBefore(ctx, time.Now()))

		info, err = cache.Get(ctx, node.ID())
		require.NoError(t, err)
		require.NotNil(t, info.Disqualified)
	})
}

func TestUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.AuditCount = 2
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		cache := planet.Satellites[0].DB.OverlayCache()

		// 1 audit -> unvetted
		updateReq := &overlay.UpdateRequest{
			NodeID:                   node.ID(),
			AuditOutcome:             overlay.AuditOffline,
			AuditsRequiredForVetting: planet.Satellites[0].Config.Overlay.Node.AuditCount,
			AuditHistory:             testAuditHistoryConfig(),
		}
		nodeStats, err := cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt)
		assert.EqualValues(t, 1, nodeStats.AuditCount)

		// 2 audits -> vetted
		updateReq.NodeID = node.ID()
		updateReq.AuditOutcome = overlay.AuditOffline
		nodeStats, err = cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats.VettedAt)
		assert.EqualValues(t, 2, nodeStats.AuditCount)

		// Don't overwrite node's vetted_at timestamp
		updateReq.NodeID = node.ID()
		updateReq.AuditOutcome = overlay.AuditSuccess
		nodeStats2, err := cache.UpdateStats(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats2.VettedAt)
		assert.Equal(t, nodeStats.VettedAt, nodeStats2.VettedAt)
		assert.EqualValues(t, 3, nodeStats2.AuditCount)
	})
}

func TestBatchUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.AuditCount = 2
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeA := planet.StorageNodes[0]
		nodeB := planet.StorageNodes[1]
		nodeA.Contact.Chore.Pause(ctx)
		nodeB.Contact.Chore.Pause(ctx)

		cache := planet.Satellites[0].DB.OverlayCache()
		numAudits := planet.Satellites[0].Config.Overlay.Node.AuditCount
		batchSize := 2

		// unvetted
		updateReqA := &overlay.UpdateRequest{NodeID: nodeA.ID(), AuditOutcome: overlay.AuditOffline, AuditsRequiredForVetting: numAudits, AuditHistory: testAuditHistoryConfig()}
		updateReqB := &overlay.UpdateRequest{NodeID: nodeB.ID(), AuditOutcome: overlay.AuditSuccess, AuditsRequiredForVetting: numAudits, AuditHistory: testAuditHistoryConfig()}
		updateReqs := []*overlay.UpdateRequest{updateReqA, updateReqB}
		failed, err := cache.BatchUpdateStats(ctx, updateReqs, batchSize, time.Now())
		require.NoError(t, err)
		assert.Len(t, failed, 0)

		nA, err := cache.Get(ctx, nodeA.ID())
		require.NoError(t, err)
		assert.Nil(t, nA.Reputation.VettedAt)
		assert.EqualValues(t, 1, nA.Reputation.AuditCount)

		nB, err := cache.Get(ctx, nodeB.ID())
		require.NoError(t, err)
		assert.Nil(t, nB.Reputation.VettedAt)
		assert.EqualValues(t, 1, nB.Reputation.AuditCount)

		// vetted
		updateReqA = &overlay.UpdateRequest{NodeID: nodeA.ID(), AuditOutcome: overlay.AuditOffline, AuditsRequiredForVetting: numAudits, AuditHistory: testAuditHistoryConfig()}
		updateReqB = &overlay.UpdateRequest{NodeID: nodeB.ID(), AuditOutcome: overlay.AuditFailure, AuditsRequiredForVetting: numAudits, AuditHistory: testAuditHistoryConfig()}
		updateReqs = []*overlay.UpdateRequest{updateReqA, updateReqB}
		failed, err = cache.BatchUpdateStats(ctx, updateReqs, batchSize, time.Now())
		require.NoError(t, err)
		assert.Len(t, failed, 0)

		nA, err = cache.Get(ctx, nodeA.ID())
		require.NoError(t, err)
		assert.NotNil(t, nA.Reputation.VettedAt)
		assert.EqualValues(t, 2, nA.Reputation.AuditCount)

		nB, err = cache.Get(ctx, nodeB.ID())
		require.NoError(t, err)
		assert.NotNil(t, nB.Reputation.VettedAt)
		assert.EqualValues(t, 2, nB.Reputation.AuditCount)

		// don't overwrite timestamp
		updateReqA = &overlay.UpdateRequest{NodeID: nodeA.ID(), AuditOutcome: overlay.AuditSuccess, AuditsRequiredForVetting: numAudits, AuditHistory: testAuditHistoryConfig()}
		updateReqB = &overlay.UpdateRequest{NodeID: nodeB.ID(), AuditOutcome: overlay.AuditSuccess, AuditsRequiredForVetting: numAudits, AuditHistory: testAuditHistoryConfig()}
		updateReqs = []*overlay.UpdateRequest{updateReqA, updateReqB}
		failed, err = cache.BatchUpdateStats(ctx, updateReqs, batchSize, time.Now())
		require.NoError(t, err)
		assert.Len(t, failed, 0)

		nA2, err := cache.Get(ctx, nodeA.ID())
		require.NoError(t, err)
		assert.NotNil(t, nA2.Reputation.VettedAt)
		assert.Equal(t, nA.Reputation.VettedAt, nA2.Reputation.VettedAt)
		assert.EqualValues(t, 3, nA2.Reputation.AuditCount)

		nB2, err := cache.Get(ctx, nodeB.ID())
		require.NoError(t, err)
		assert.NotNil(t, nB2.Reputation.VettedAt)
		assert.Equal(t, nB.Reputation.VettedAt, nB2.Reputation.VettedAt)
		assert.EqualValues(t, 3, nB2.Reputation.AuditCount)
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
