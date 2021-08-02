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
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/reputation"
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
		nodeStats, changed, err := db.Update(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt)
		require.False(t, changed)

		// 2 audits -> vetted
		updateReq.NodeID = node.ID()
		updateReq.AuditOutcome = reputation.AuditOffline
		nodeStats, changed, err = db.Update(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats.VettedAt)
		require.True(t, changed)

		// Don't overwrite node's vetted_at timestamp
		updateReq.NodeID = node.ID()
		updateReq.AuditOutcome = reputation.AuditSuccess
		nodeStats2, changed, err := db.Update(ctx, updateReq, time.Now())
		require.NoError(t, err)
		assert.NotNil(t, nodeStats2.VettedAt)
		assert.Equal(t, nodeStats.VettedAt, nodeStats2.VettedAt)
		require.False(t, changed)

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
