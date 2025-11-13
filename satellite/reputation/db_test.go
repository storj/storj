// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/errs2"
	"storj.io/common/pb"
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
			NodeID:       node.ID(),
			AuditOutcome: reputation.AuditOffline,
			Config: reputation.Config{
				AuditCount:   planet.Satellites[0].Config.Reputation.AuditCount,
				AuditHistory: testAuditHistoryConfig(),
			},
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

func TestUpdateWithMinimumNodeAge(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditCount = 2
				config.Reputation.MinimumNodeAge = 24 * time.Hour // 1 day minimum age
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		db := planet.Satellites[0].DB.Reputation()

		// Test 1: Node meets audit count but is too young -> should not vet
		updateReq := reputation.UpdateRequest{
			NodeID:       node.ID(),
			AuditOutcome: reputation.AuditOffline,
			Config: reputation.Config{
				AuditCount:     planet.Satellites[0].Config.Reputation.AuditCount,
				MinimumNodeAge: planet.Satellites[0].Config.Reputation.MinimumNodeAge,
				AuditHistory:   testAuditHistoryConfig(),
			},
		}

		// First audit - this creates the reputation record
		now := time.Now()
		nodeStats, err := db.Update(ctx, updateReq, now)
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt, "node should not be vetted after 1 audit")

		// Get the actual CreatedAt from the database
		startTime := nodeStats.CreatedAt
		require.NotNil(t, startTime)

		// Second audit 1 hr (meets audit count but still too young, needs 24 hours)
		secondAuditTime := startTime.Add(time.Hour)
		nodeStats, err = db.Update(ctx, updateReq, secondAuditTime)
		require.NoError(t, err)
		assert.Nil(t, nodeStats.VettedAt, "node should not be vetted yet - too young despite meeting audit count")

		// Test 2: Node is now old enough and meets audit count -> should vet
		// Simulate time passing by doing an audit 25 hours after node creation
		oldEnoughTime := startTime.Add(25 * time.Hour)
		nodeStats, err = db.Update(ctx, updateReq, oldEnoughTime)
		require.NoError(t, err)
		assert.NotNil(t, nodeStats.VettedAt, "node should be vetted now - old enough and meets audit count")

		// Test 3: Verify vetted_at timestamp is not overwritten on subsequent audits
		nodeStats2, err := db.Update(ctx, updateReq, oldEnoughTime.Add(time.Hour))
		require.NoError(t, err)
		assert.NotNil(t, nodeStats2.VettedAt)
		assert.Equal(t, nodeStats.VettedAt, nodeStats2.VettedAt, "vetted_at should not change on subsequent audits")
	})
}

// testApplyUpdatesEquivalentToMultipleUpdates checks that the ApplyUpdates call
// is equivalent to making multiple separate Update() calls (modulo some details
// like exact-time-of-disqualification).
func testApplyUpdatesEquivalentToMultipleUpdates(ctx context.Context, t *testing.T, reputationDB reputation.DB, config reputation.Config) {
	for _, testDef := range []struct {
		name      string
		failures  int
		successes int
		offlines  int
		unknowns  int
	}{
		{"4f-3s", 4, 3, 0, 0},
		{"3s-3o", 0, 3, 3, 0},
		{"4s-2u", 0, 4, 0, 2},
		{"1f-4s-1o-3u", 1, 4, 1, 3},
		{"4o", 4, 0, 0, 0},
		{"5s", 0, 5, 0, 0},
		{"6u", 0, 0, 0, 6},
	} {
		t.Run(testDef.name, func(t *testing.T) {
			node1 := testrand.NodeID()
			node2 := testrand.NodeID()
			startTime := time.Now()
			var (
				info1, info2 *reputation.Info
				err          error
			)

			// Do the Update() calls first, on node1

			updateReq := reputation.UpdateRequest{
				NodeID: node1,
				Config: config,
			}

			updateReq.AuditOutcome = reputation.AuditFailure
			for i := 0; i < testDef.failures; i++ {
				info1, err = reputationDB.Update(ctx, updateReq, startTime.Add(time.Duration(i)*time.Minute))
				require.NoError(t, err)
			}
			updateReq.AuditOutcome = reputation.AuditOffline
			for i := 0; i < testDef.offlines; i++ {
				info1, err = reputationDB.Update(ctx, updateReq, startTime.Add(time.Duration(10+i)*time.Minute))
				require.NoError(t, err)
			}
			updateReq.AuditOutcome = reputation.AuditUnknown
			for i := 0; i < testDef.unknowns; i++ {
				info1, err = reputationDB.Update(ctx, updateReq, startTime.Add(time.Duration(20+i)*time.Minute))
				require.NoError(t, err)
			}
			updateReq.AuditOutcome = reputation.AuditSuccess
			for i := 0; i < testDef.successes; i++ {
				info1, err = reputationDB.Update(ctx, updateReq, startTime.Add(time.Duration(30+i)*time.Minute))
				require.NoError(t, err)
			}

			// Now do the single ApplyUpdates call, on node2

			var hist pb.AuditHistory
			for i := 0; i < testDef.failures; i++ {
				err = reputation.AddAuditToHistory(&hist, true, startTime.Add(time.Duration(i)*time.Minute), config.AuditHistory)
				require.NoError(t, err)
			}
			for i := 0; i < testDef.offlines; i++ {
				err = reputation.AddAuditToHistory(&hist, false, startTime.Add(time.Duration(10+i)*time.Minute), config.AuditHistory)
				require.NoError(t, err)
			}
			for i := 0; i < testDef.unknowns; i++ {
				err = reputation.AddAuditToHistory(&hist, true, startTime.Add(time.Duration(20+i)*time.Minute), config.AuditHistory)
				require.NoError(t, err)
			}
			for i := 0; i < testDef.successes; i++ {
				err = reputation.AddAuditToHistory(&hist, true, startTime.Add(time.Duration(30+i)*time.Minute), config.AuditHistory)
				require.NoError(t, err)
			}
			mutations := reputation.Mutations{
				PositiveResults: testDef.successes,
				FailureResults:  testDef.failures,
				UnknownResults:  testDef.unknowns,
				OfflineResults:  testDef.offlines,
				OnlineHistory:   &hist,
			}
			info2, err = reputationDB.ApplyUpdates(ctx, node2, mutations, config, startTime.Add(40*time.Minute))
			require.NoError(t, err)

			require.NotNil(t, info1)
			require.NotNil(t, info2)
			require.Equalf(t, info1.VettedAt == nil, info2.VettedAt == nil,
				"info1.VettedAt (%v) and info2.VettedAt (%v) should both be nil or both have values", info1.VettedAt, info2.VettedAt)
			require.Equalf(t, info1.Disqualified == nil, info2.Disqualified == nil,
				"info1.Disqualified (%v) and info2.Disqualified (%v) should both be nil or both have values", info1.Disqualified, info2.Disqualified)
			require.InDelta(t, info1.AuditReputationAlpha, info2.AuditReputationAlpha, 1e-8)
			require.InDelta(t, info1.AuditReputationBeta, info2.AuditReputationBeta, 1e-8)
			require.InDelta(t, info1.UnknownAuditReputationAlpha, info2.UnknownAuditReputationAlpha, 1e-8)
			require.InDelta(t, info1.UnknownAuditReputationBeta, info2.UnknownAuditReputationBeta, 1e-8)
			require.InDelta(t, info1.OnlineScore, info2.OnlineScore, 1e-8)
			require.InDelta(t, info1.AuditHistory.Score, info2.AuditHistory.Score, 1e-8)
			require.NotNil(t, info1.AuditHistory)
			require.NotNil(t, info2.AuditHistory)
			require.Equal(t, info1.AuditHistory.Score, info2.AuditHistory.Score)
			require.Equal(t, len(info1.AuditHistory.Windows), len(info2.AuditHistory.Windows),
				"info1.AuditHistory.Windows (%v) and info2.AuditHistory.Windows (%v) should have the same length", info1.AuditHistory.Windows, info2.AuditHistory.Windows)
		})
	}
}

// TestApplyUpdatesEquivalentToMultipleUpdates checks that the ApplyUpdates call
// on db.Reputation() is equivalent to making multiple separate Update() calls
// (modulo some details like exact-time-of-disqualification).
func TestApplyUpdatesEquivalentToMultipleUpdates(t *testing.T) {
	config := reputation.Config{
		AuditLambda:           0.99,
		AuditWeight:           1,
		AuditDQ:               0.1,
		InitialAlpha:          1000,
		InitialBeta:           0,
		UnknownAuditDQ:        0.1,
		UnknownAuditLambda:    0.95,
		SuspensionGracePeriod: 20 * time.Minute,
		SuspensionDQEnabled:   true,
		AuditCount:            3,
		MinimumNodeAge:        0,
		AuditHistory: reputation.AuditHistoryConfig{
			WindowSize:               10 * time.Minute,
			TrackingPeriod:           1 * time.Hour,
			GracePeriod:              20 * time.Minute,
			OfflineThreshold:         0.5,
			OfflineDQEnabled:         false,
			OfflineSuspensionEnabled: true,
		},
	}

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		testApplyUpdatesEquivalentToMultipleUpdates(ctx, t, db.Reputation(), config)
	})
}

// TestApplyUpdatesEquivalentToMultipleUpdatesCached checks that the ApplyUpdates
// call on a CachingDB is equivalent to making multiple separate Update() calls
// (modulo some details like exact-time-of-disqualification).
func TestApplyUpdatesEquivalentToMultipleUpdatesCached(t *testing.T) {
	config := reputation.Config{
		AuditLambda:           0.99,
		AuditWeight:           1,
		AuditDQ:               0.1,
		InitialAlpha:          1000,
		InitialBeta:           0,
		UnknownAuditDQ:        0.1,
		UnknownAuditLambda:    0.95,
		SuspensionGracePeriod: 20 * time.Minute,
		SuspensionDQEnabled:   true,
		AuditCount:            3,
		MinimumNodeAge:        0,
		AuditHistory: reputation.AuditHistoryConfig{
			WindowSize:               10 * time.Minute,
			TrackingPeriod:           1 * time.Hour,
			GracePeriod:              20 * time.Minute,
			OfflineThreshold:         0.5,
			OfflineDQEnabled:         false,
			OfflineSuspensionEnabled: true,
		},
	}

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cachingDB := reputation.NewCachingDB(zaptest.NewLogger(t), db.Reputation(), config)
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		ctx.Go(func() error {
			err := cachingDB.Manage(cancelCtx)
			return errs2.IgnoreCanceled(err)
		})
		testApplyUpdatesEquivalentToMultipleUpdates(cancelCtx, t, cachingDB, config)

		cancel()
		ctx.Wait() // wait for the above cachingDB.Manage to return
	})
}

func TestDBDisqualifyNode(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reputationDB := db.Reputation()
		nodeID := testrand.NodeID()
		now := time.Now().Truncate(time.Second).UTC()

		err := reputationDB.DisqualifyNode(ctx, nodeID, now, overlay.DisqualificationReasonAuditFailure)
		require.NoError(t, err)

		info, err := reputationDB.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, info.Disqualified)
		require.Equal(t, now, info.Disqualified.UTC())
		require.Equal(t, overlay.DisqualificationReasonAuditFailure, info.DisqualificationReason)
	})
}

func TestDBDisqualificationAuditFailure(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reputationDB := db.Reputation()
		nodeID := testrand.NodeID()
		now := time.Now()

		updateReq := reputation.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: reputation.AuditFailure,
			Config: reputation.Config{
				AuditLambda:           1,
				UnknownAuditLambda:    1,
				AuditWeight:           1,
				AuditDQ:               0.99,
				UnknownAuditDQ:        0.99,
				SuspensionGracePeriod: 0,
				SuspensionDQEnabled:   false,
				AuditCount:            0,
				AuditHistory:          reputation.AuditHistoryConfig{},
				InitialAlpha:          1,
				InitialBeta:           0,
			},
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
			NodeID:       nodeID,
			AuditOutcome: reputation.AuditUnknown,
			Config: reputation.Config{
				AuditLambda:           1,
				UnknownAuditLambda:    1,
				AuditWeight:           1,
				AuditDQ:               0.99,
				UnknownAuditDQ:        0.99,
				InitialAlpha:          1000,
				InitialBeta:           0,
				SuspensionGracePeriod: 0,
				SuspensionDQEnabled:   true,
				AuditCount:            0,
				AuditHistory:          reputation.AuditHistoryConfig{},
			},
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
			NodeID:       nodeID,
			AuditOutcome: reputation.AuditOffline,
			Config: reputation.Config{
				AuditLambda:           0,
				UnknownAuditLambda:    0,
				AuditWeight:           0,
				AuditDQ:               0,
				UnknownAuditDQ:        0,
				InitialAlpha:          0,
				InitialBeta:           0,
				SuspensionGracePeriod: 0,
				SuspensionDQEnabled:   false,
				AuditCount:            0,
				AuditHistory: reputation.AuditHistoryConfig{
					WindowSize:               1 * time.Second,
					TrackingPeriod:           1 * time.Second,
					GracePeriod:              0,
					OfflineThreshold:         1,
					OfflineDQEnabled:         true,
					OfflineSuspensionEnabled: true,
				},
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
