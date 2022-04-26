// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUpdate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditCount = 2
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		db := planet.Satellites[0].DB.Reputation()

		// 1 audit -> unvetted
		updateReq := reputation.UpdateRequest{
			NodeID:                   node.ID(),
			AuditOutcome:             reputation.AuditOffline,
			AuditsRequiredForVetting: planet.Satellites[0].Config.Reputation.AuditCount,
			AuditHistory:             testAuditHistoryConfig(),
		}
		nodeStats, err := db.Update(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt)

		// 2 audits -> vetted
		updateReq.NodeID = node.ID()
		updateReq.AuditOutcome = reputation.AuditOffline
		nodeStats, err = db.Update(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats.VettedAt)

		// Don't overwrite node's vetted_at timestamp
		updateReq.NodeID = node.ID()
		updateReq.AuditOutcome = reputation.AuditSuccess
		nodeStats2, err := db.Update(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats2.VettedAt)
		assert.Equal(t, nodeStats.VettedAt, nodeStats2.VettedAt)

	})
}

func TestDBDisqualifyNode(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reputationDB := db.Reputation()
		nodeID := testrand.NodeID()
		now := time.Now().Truncate(time.Second).UTC()

		err := reputationDB.DisqualifyNode(ctx, nodeID, now)
		require.NoError(t, err)

		info, err := reputationDB.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, info.Disqualified)
		require.Equal(t, now, info.Disqualified.UTC())
	})
}

func TestDBDisqualificationAuditFailure(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reputationDB := db.Reputation()
		nodeID := testrand.NodeID()
		now := time.Now()

		updateReq := reputation.UpdateRequest{
			NodeID:                   nodeID,
			AuditOutcome:             reputation.AuditFailure,
			AuditCount:               0,
			AuditLambda:              1,
			AuditWeight:              1,
			AuditDQ:                  0.99,
			SuspensionGracePeriod:    0,
			SuspensionDQEnabled:      false,
			AuditsRequiredForVetting: 0,
			AuditHistory:             reputation.AuditHistoryConfig{},
		}

		status, err := reputationDB.Update(ctx, updateReq, now)
		require.NoError(t, err)
		require.NotNil(t, status.Disqualified)
		assert.WithinDuration(t, now, *status.Disqualified, time.Microsecond)
		assert.Equal(t, overlay.DisqualificationReasonAuditFailure, status.DisqualificationReason)
	})
}

func TestDBDisqualificationSuspension(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reputationDB := db.Reputation()
		nodeID := testrand.NodeID()
		now := time.Now().Truncate(time.Second).UTC()

		updateReq := reputation.UpdateRequest{
			NodeID:                   nodeID,
			AuditOutcome:             reputation.AuditUnknown,
			AuditCount:               0,
			AuditLambda:              1,
			AuditWeight:              1,
			AuditDQ:                  0.99,
			SuspensionGracePeriod:    0,
			SuspensionDQEnabled:      true,
			AuditsRequiredForVetting: 0,
			AuditHistory:             reputation.AuditHistoryConfig{},
		}

		// suspend node due to failed unknown audit
		err := reputationDB.SuspendNodeUnknownAudit(ctx, nodeID, now.Add(-time.Second))
		require.NoError(t, err)

		// disqualify node after failed unknown audit when node is suspended
		status, err := reputationDB.Update(ctx, updateReq, now)
		require.NoError(t, err)
		require.NotNil(t, status.Disqualified)
		assert.Nil(t, status.UnknownAuditSuspended)
		assert.Equal(t, now, status.Disqualified.UTC())
		assert.Equal(t, overlay.DisqualificationReasonSuspension, status.DisqualificationReason)
	})
}

func TestDBDisqualificationNodeOffline(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reputationDB := db.Reputation()
		nodeID := testrand.NodeID()
		now := time.Now().Truncate(time.Second).UTC()

		updateReq := reputation.UpdateRequest{
			NodeID:                   nodeID,
			AuditOutcome:             reputation.AuditOffline,
			AuditCount:               0,
			AuditLambda:              0,
			AuditWeight:              0,
			AuditDQ:                  0,
			SuspensionGracePeriod:    0,
			SuspensionDQEnabled:      false,
			AuditsRequiredForVetting: 0,
			AuditHistory: reputation.AuditHistoryConfig{
				WindowSize:               0,
				TrackingPeriod:           1 * time.Second,
				GracePeriod:              0,
				OfflineThreshold:         1,
				OfflineDQEnabled:         true,
				OfflineSuspensionEnabled: true,
			},
		}

		// first window always returns perfect score
		_, err := reputationDB.Update(ctx, updateReq, now)
		require.NoError(t, err)

		// put node to offline suspension
		suspendedAt := now.Add(time.Second)
		status, err := reputationDB.Update(ctx, updateReq, suspendedAt)
		require.NoError(t, err)
		require.Equal(t, suspendedAt, status.OfflineSuspended.UTC())

		// should have at least 2 windows in audit history after earliest window is removed
		_, err = reputationDB.Update(ctx, updateReq, now.Add(2*time.Second))
		require.NoError(t, err)

		// disqualify node
		disqualifiedAt := now.Add(3 * time.Second)
		status, err = reputationDB.Update(ctx, updateReq, disqualifiedAt)
		require.NoError(t, err)
		require.NotNil(t, status.Disqualified)
		assert.Equal(t, disqualifiedAt, status.Disqualified.UTC())
		assert.Equal(t, overlay.DisqualificationReasonNodeOffline, status.DisqualificationReason)
	})
}

func testAuditHistoryConfig() reputation.AuditHistoryConfig {
	return reputation.AuditHistoryConfig{
		WindowSize:       time.Hour,
		TrackingPeriod:   time.Hour,
		GracePeriod:      time.Hour,
		OfflineThreshold: 0,
	}
}
