// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

func TestConcurrentAudit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].RangedLoop.RangedLoop.Service.Loop.Stop()

		data := testrand.Bytes(10 * memory.MB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "testpath", data)
		require.NoError(t, err)
		var group errgroup.Group
		n := 5
		for i := 0; i < n; i++ {
			group.Go(func() error {
				err := planet.Satellites[0].Reputation.Service.ApplyAudit(ctx, planet.StorageNodes[0].ID(), overlay.ReputationStatus{}, reputation.AuditSuccess)
				return err
			})
		}
		err = group.Wait()
		require.NoError(t, err)

		node, err := planet.Satellites[0].Reputation.Service.Get(ctx, planet.StorageNodes[0].ID())
		require.NoError(t, err)
		require.Equal(t, int64(n), node.TotalAuditCount)
	})
}

func TestApplyAudit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditLambda = 0.123
				config.Reputation.AuditWeight = 0.456
				config.Reputation.AuditDQ = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		service := planet.Satellites[0].Reputation.Service
		config := planet.Satellites[0].Config.Reputation

		// node has not been audited yet
		node, err := service.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Zero(t, node.TotalAuditCount)

		status := overlay.ReputationStatus{
			Disqualified:          node.Disqualified,
			UnknownAuditSuspended: node.UnknownAuditSuspended,
			OfflineSuspended:      node.OfflineSuspended,
			VettedAt:              node.VettedAt,
		}
		err = service.ApplyAudit(ctx, nodeID, status, reputation.AuditSuccess)
		require.NoError(t, err)

		node, err = service.Get(ctx, nodeID)
		require.NoError(t, err)
		auditAlpha := node.AuditReputationAlpha
		auditBeta := node.AuditReputationBeta
		status = overlay.ReputationStatus{
			Disqualified:          node.Disqualified,
			UnknownAuditSuspended: node.UnknownAuditSuspended,
			OfflineSuspended:      node.OfflineSuspended,
			VettedAt:              node.VettedAt,
		}

		err = service.ApplyAudit(ctx, nodeID, status, reputation.AuditSuccess)
		require.NoError(t, err)

		stats, err := service.Get(ctx, nodeID)
		require.NoError(t, err)
		status = overlay.ReputationStatus{
			Disqualified:          stats.Disqualified,
			UnknownAuditSuspended: stats.UnknownAuditSuspended,
			OfflineSuspended:      stats.OfflineSuspended,
			VettedAt:              stats.VettedAt,
		}

		expectedAuditAlpha := config.AuditLambda*auditAlpha + config.AuditWeight
		expectedAuditBeta := config.AuditLambda * auditBeta
		require.InDelta(t, stats.AuditReputationAlpha, expectedAuditAlpha, 1e-8)
		require.InDelta(t, stats.AuditReputationBeta, expectedAuditBeta, 1e-8)

		auditAlpha = expectedAuditAlpha
		auditBeta = expectedAuditBeta

		err = service.ApplyAudit(ctx, nodeID, status, reputation.AuditFailure)
		require.NoError(t, err)

		stats, err = service.Get(ctx, nodeID)
		require.NoError(t, err)

		expectedAuditAlpha = config.AuditLambda * auditAlpha
		expectedAuditBeta = config.AuditLambda*auditBeta + config.AuditWeight
		require.InDelta(t, stats.AuditReputationAlpha, expectedAuditAlpha, 1e-8)
		require.InDelta(t, stats.AuditReputationBeta, expectedAuditBeta, 1e-8)

	})
}

func TestGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		service := planet.Satellites[0].Reputation.Service
		repConfig := planet.Satellites[0].Config.Reputation

		// existing node has not been audited yet should have default reputation
		// score
		node, err := service.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Zero(t, node.TotalAuditCount)
		require.InDelta(t, repConfig.InitialAlpha, node.AuditReputationAlpha, 1e-8)
		require.InDelta(t, repConfig.InitialBeta, node.AuditReputationBeta, 1e-8)
		require.InDelta(t, 1, node.UnknownAuditReputationAlpha, 1e-8)
		require.EqualValues(t, 1, node.OnlineScore)

		// if a node has no entry in reputation store, it should have default
		// reputation score
		newNode, err := service.Get(ctx, testrand.NodeID())
		require.NoError(t, err)
		require.Zero(t, newNode.TotalAuditCount)
		require.InDelta(t, repConfig.InitialAlpha, newNode.AuditReputationAlpha, 1e-8)
		require.InDelta(t, repConfig.InitialBeta, newNode.AuditReputationBeta, 1e-8)
		require.InDelta(t, 1, newNode.UnknownAuditReputationAlpha, 1e-8)
		require.EqualValues(t, 1, newNode.OnlineScore)
	})
}

func TestDisqualificationAuditFailure(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.4
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satel := planet.Satellites[0]
		nodeID := planet.StorageNodes[0].ID()

		nodeInfo, err := satel.Overlay.Service.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.Nil(t, nodeInfo.Disqualified)

		err = satel.Reputation.Service.ApplyAudit(ctx, nodeID, nodeInfo.Reputation.Status, reputation.AuditFailure)
		require.NoError(t, err)

		// node is not disqualified after failed audit if score is above threshold
		repInfo, err := satel.Reputation.Service.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.Nil(t, repInfo.Disqualified)
		nodeInfo, err = satel.Overlay.Service.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.Nil(t, nodeInfo.Disqualified)

		err = satel.Reputation.Service.ApplyAudit(ctx, nodeID, nodeInfo.Reputation.Status, reputation.AuditFailure)
		require.NoError(t, err)

		repInfo, err = satel.Reputation.Service.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.NotNil(t, repInfo.Disqualified)
		nodeInfo, err = satel.Overlay.Service.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.NotNil(t, nodeInfo.Disqualified)
	})
}

func TestExitedAndDQNodesGetNoAudit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 1
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.4
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satel := planet.Satellites[0]
		okNode := planet.StorageNodes[0].ID()
		dqNode := planet.StorageNodes[1].ID()
		exitNode := planet.StorageNodes[2].ID()

		// Ok node gets audit
		require.NoError(t, satel.Reputation.Service.ApplyAudit(ctx, okNode, overlay.ReputationStatus{}, reputation.AuditOffline))
		info, err := satel.Reputation.Service.Get(ctx, okNode)
		require.NoError(t, err)
		require.Equal(t, int64(1), info.TotalAuditCount)

		// DQ node
		require.NoError(t, satel.Overlay.Service.DisqualifyNode(ctx, dqNode, overlay.DisqualificationReasonAuditFailure))
		require.NoError(t, satel.Reputation.Service.ApplyAudit(ctx, dqNode, overlay.ReputationStatus{}, reputation.AuditOffline))
		info, err = satel.Reputation.Service.Get(ctx, dqNode)
		require.NoError(t, err)
		require.Zero(t, info.TotalAuditCount)

		// Exit node
		now := time.Now()
		_, err = satel.Overlay.DB.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              exitNode,
			ExitInitiatedAt:     now,
			ExitLoopCompletedAt: now,
			ExitFinishedAt:      now,
			ExitSuccess:         true,
		})
		require.NoError(t, err)
		require.NoError(t, satel.Reputation.Service.ApplyAudit(ctx, exitNode, overlay.ReputationStatus{}, reputation.AuditOffline))
		info, err = satel.Reputation.Service.Get(ctx, exitNode)
		require.NoError(t, err)
		require.Zero(t, info.TotalAuditCount)
	})
}
