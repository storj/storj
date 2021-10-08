// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/reputation"
)

func TestConcurrentAudit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Audit.Chore.Loop.Stop()
		data := testrand.Bytes(10 * memory.MB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "testpath", data)
		require.NoError(t, err)
		var group errgroup.Group
		n := 5
		for i := 0; i < n; i++ {
			group.Go(func() error {
				err := planet.Satellites[0].Reputation.Service.ApplyAudit(ctx, planet.StorageNodes[0].ID(), reputation.AuditSuccess)
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

		err = service.ApplyAudit(ctx, nodeID, reputation.AuditSuccess)
		require.NoError(t, err)

		node, err = service.Get(ctx, nodeID)
		require.NoError(t, err)
		auditAlpha := node.AuditReputationAlpha
		auditBeta := node.AuditReputationBeta

		err = service.ApplyAudit(ctx, nodeID, reputation.AuditSuccess)
		require.NoError(t, err)

		stats, err := service.Get(ctx, nodeID)
		require.NoError(t, err)

		expectedAuditAlpha := config.AuditLambda*auditAlpha + config.AuditWeight
		expectedAuditBeta := config.AuditLambda * auditBeta
		require.EqualValues(t, stats.AuditReputationAlpha, expectedAuditAlpha)
		require.EqualValues(t, stats.AuditReputationBeta, expectedAuditBeta)

		auditAlpha = expectedAuditAlpha
		auditBeta = expectedAuditBeta

		err = service.ApplyAudit(ctx, nodeID, reputation.AuditFailure)
		require.NoError(t, err)

		stats, err = service.Get(ctx, nodeID)
		require.NoError(t, err)

		expectedAuditAlpha = config.AuditLambda * auditAlpha
		expectedAuditBeta = config.AuditLambda*auditBeta + config.AuditWeight
		require.EqualValues(t, stats.AuditReputationAlpha, expectedAuditAlpha)
		require.EqualValues(t, stats.AuditReputationBeta, expectedAuditBeta)

	})
}

func TestGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		service := planet.Satellites[0].Reputation.Service

		// existing node has not been audited yet should have default reputation
		// score
		node, err := service.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Zero(t, node.TotalAuditCount)
		require.EqualValues(t, 1, node.AuditReputationAlpha)
		require.EqualValues(t, 1, node.UnknownAuditReputationAlpha)
		require.EqualValues(t, 1, node.OnlineScore)

		// if a node has no entry in reputation store, it should have default
		// reputation score
		newNode, err := service.Get(ctx, testrand.NodeID())
		require.NoError(t, err)
		require.Zero(t, newNode.TotalAuditCount)
		require.EqualValues(t, 1, newNode.AuditReputationAlpha)
		require.EqualValues(t, 1, newNode.UnknownAuditReputationAlpha)
		require.EqualValues(t, 1, newNode.OnlineScore)
	})
}
