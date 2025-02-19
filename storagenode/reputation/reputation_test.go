// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestReputationDBGetInsert(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		timestamp := time.Now()
		reputationDB := db.Reputation()

		stats := reputation.Stats{
			SatelliteID: testrand.NodeID(),
			Audit: reputation.Metric{
				TotalCount:   6,
				SuccessCount: 7,
				Alpha:        8,
				Beta:         9,
				Score:        10,
				UnknownAlpha: 11,
				UnknownBeta:  12,
				UnknownScore: 13,
			},
			OnlineScore:          14,
			OfflineUnderReviewAt: &timestamp,
			OfflineSuspendedAt:   &timestamp,
			DisqualifiedAt:       &timestamp,
			SuspendedAt:          &timestamp,
			UpdatedAt:            timestamp,
			JoinedAt:             timestamp,
		}

		t.Run("insert", func(t *testing.T) {
			err := reputationDB.Store(ctx, stats)
			require.NoError(t, err)
		})

		t.Run("get", func(t *testing.T) {
			res, err := reputationDB.Get(ctx, stats.SatelliteID)
			require.NoError(t, err)

			require.Equal(t, res.SatelliteID, stats.SatelliteID)
			require.True(t, res.DisqualifiedAt.Equal(*stats.DisqualifiedAt))
			require.True(t, res.SuspendedAt.Equal(*stats.SuspendedAt))
			require.True(t, res.UpdatedAt.Equal(stats.UpdatedAt))
			require.True(t, res.JoinedAt.Equal(stats.JoinedAt))
			require.True(t, res.OfflineSuspendedAt.Equal(*stats.OfflineSuspendedAt))
			require.True(t, res.OfflineUnderReviewAt.Equal(*stats.OfflineUnderReviewAt))
			require.Equal(t, res.OnlineScore, stats.OnlineScore)
			require.Nil(t, res.AuditHistory)

			compareReputationMetric(t, &res.Audit, &stats.Audit)
		})
	})
}

func TestReputationDBGetAll(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		reputationDB := db.Reputation()

		var stats []reputation.Stats
		for i := 0; i < 10; i++ {
			// we use UTC here to make struct equality testing easier
			timestamp := time.Now().UTC().Add(time.Hour * time.Duration(i))

			rep := reputation.Stats{
				SatelliteID: testrand.NodeID(),
				Audit: reputation.Metric{
					TotalCount:   int64(i + 6),
					SuccessCount: int64(i + 7),
					Alpha:        float64(i + 8),
					Beta:         float64(i + 9),
					Score:        float64(i + 10),
					UnknownAlpha: float64(i + 11),
					UnknownBeta:  float64(i + 12),
					UnknownScore: float64(i + 13),
				},
				OnlineScore:          float64(i + 14),
				OfflineUnderReviewAt: &timestamp,
				OfflineSuspendedAt:   &timestamp,
				DisqualifiedAt:       &timestamp,
				SuspendedAt:          &timestamp,
				UpdatedAt:            timestamp,
				JoinedAt:             timestamp,
			}

			err := reputationDB.Store(ctx, rep)
			require.NoError(t, err)

			stats = append(stats, rep)
		}

		res, err := reputationDB.All(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, len(stats), len(res))

		for _, rep := range res {
			require.Contains(t, stats, rep)

			if rep.SatelliteID == stats[0].SatelliteID {
				require.Equal(t, rep.DisqualifiedAt, stats[0].DisqualifiedAt)
				require.Equal(t, rep.SuspendedAt, stats[0].SuspendedAt)
				require.Equal(t, rep.UpdatedAt, stats[0].UpdatedAt)
				require.Equal(t, rep.JoinedAt, stats[0].JoinedAt)
				require.Equal(t, rep.OfflineSuspendedAt, stats[0].OfflineSuspendedAt)
				require.Equal(t, rep.OfflineUnderReviewAt, stats[0].OfflineUnderReviewAt)
				require.Equal(t, rep.OnlineScore, stats[0].OnlineScore)
				require.Nil(t, rep.AuditHistory)

				compareReputationMetric(t, &rep.Audit, &stats[0].Audit)
			}
		}
	})
}

// compareReputationMetric compares two reputation metrics and asserts that they are equal.
func compareReputationMetric(t *testing.T, a, b *reputation.Metric) {
	require.Equal(t, a.SuccessCount, b.SuccessCount)
	require.Equal(t, a.TotalCount, b.TotalCount)
	require.Equal(t, a.Alpha, b.Alpha)
	require.Equal(t, a.Beta, b.Beta)
	require.Equal(t, a.Score, b.Score)
}

func TestReputationDBGetInsertAuditHistory(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		timestamp := time.Now()
		reputationDB := db.Reputation()

		stats := reputation.Stats{
			SatelliteID: testrand.NodeID(),
			Audit:       reputation.Metric{},
			AuditHistory: &pb.AuditHistory{
				Score: 0.5,
				Windows: []*pb.AuditWindow{
					{
						WindowStart: timestamp,
						OnlineCount: 5,
						TotalCount:  10,
					},
				},
			},
		}

		t.Run("insert", func(t *testing.T) {
			err := reputationDB.Store(ctx, stats)
			require.NoError(t, err)
		})

		t.Run("get", func(t *testing.T) {
			res, err := reputationDB.Get(ctx, stats.SatelliteID)
			require.NoError(t, err)

			require.Equal(t, res.AuditHistory.Score, stats.AuditHistory.Score)
			require.Equal(t, len(res.AuditHistory.Windows), len(stats.AuditHistory.Windows))
			resWindow := res.AuditHistory.Windows[0]
			statsWindow := stats.AuditHistory.Windows[0]
			require.True(t, resWindow.WindowStart.Equal(statsWindow.WindowStart))
			require.Equal(t, resWindow.TotalCount, statsWindow.TotalCount)
			require.Equal(t, resWindow.OnlineCount, statsWindow.OnlineCount)
		})
	})
}

func TestServiceStore(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		reputationDB := db.Reputation()
		notificationsDB := db.Notifications()
		log := zaptest.NewLogger(t)
		notificationService := notifications.NewService(log, notificationsDB)
		reputationService := reputation.NewService(log, reputationDB, rpc.Dialer{}, nil, storj.NodeID{}, notificationService)

		id := testrand.NodeID()
		now := time.Now().AddDate(0, 0, -2)
		later := time.Now().AddDate(0, 0, -1)

		stats := reputation.Stats{
			SatelliteID: id,
		}

		err := reputationDB.Store(ctx, stats)
		require.NoError(t, err)

		statsNew := reputation.Stats{
			SatelliteID:        id,
			OfflineSuspendedAt: &now,
		}

		err = reputationService.Store(ctx, statsNew, id)
		require.NoError(t, err)
		amount, err := notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 1)

		statsNew = reputation.Stats{
			SatelliteID:        id,
			OfflineSuspendedAt: &later,
		}

		err = reputationService.Store(ctx, statsNew, id)
		require.NoError(t, err)
		amount, err = notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 2)

		statsNew = reputation.Stats{
			SatelliteID:        id,
			OfflineSuspendedAt: &later,
			DisqualifiedAt:     &later,
		}

		err = reputationService.Store(ctx, statsNew, id)
		require.NoError(t, err)
		amount, err = notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 2)

		statsNew = reputation.Stats{
			SatelliteID:        id,
			OfflineSuspendedAt: &now,
			DisqualifiedAt:     &later,
		}

		err = reputationService.Store(ctx, statsNew, id)
		require.NoError(t, err)
		amount, err = notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 2)

		statsNew = reputation.Stats{
			SatelliteID:        id,
			OfflineSuspendedAt: &later,
			DisqualifiedAt:     nil,
		}

		err = reputationService.Store(ctx, statsNew, id)
		require.NoError(t, err)
		amount, err = notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 3)

		later = later.AddDate(0, 1, 0)

		statsNew = reputation.Stats{
			SatelliteID:        id,
			OfflineSuspendedAt: &later,
		}

		err = reputationService.Store(ctx, statsNew, id)
		require.NoError(t, err)
		amount, err = notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 4)

		statsNew = reputation.Stats{
			SatelliteID:        id,
			OfflineSuspendedAt: &later,
		}

		err = reputationService.Store(ctx, statsNew, id)
		require.NoError(t, err)
		amount, err = notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 4)

		id2 := testrand.NodeID()

		statsNew = reputation.Stats{
			SatelliteID:        id2,
			OfflineSuspendedAt: &later,
		}

		err = reputationService.Store(ctx, statsNew, id2)
		require.NoError(t, err)
		amount, err = notificationsDB.UnreadAmount(ctx)
		require.NoError(t, err)
		require.Equal(t, amount, 5)
	})
}
