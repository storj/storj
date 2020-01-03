// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestReputationDBGetInsert(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		timestamp := time.Now().UTC()
		reputationDB := db.Reputation()

		stats := reputation.Stats{
			SatelliteID: testrand.NodeID(),
			Uptime: reputation.Metric{
				TotalCount:   1,
				SuccessCount: 2,
				Alpha:        3,
				Beta:         4,
				Score:        5,
			},
			Audit: reputation.Metric{
				TotalCount:   6,
				SuccessCount: 7,
				Alpha:        8,
				Beta:         9,
				Score:        10,
			},
			Disqualified: &timestamp,
			UpdatedAt:    timestamp,
		}

		t.Run("insert", func(t *testing.T) {
			err := reputationDB.Store(ctx, stats)
			assert.NoError(t, err)
		})

		t.Run("get", func(t *testing.T) {
			res, err := reputationDB.Get(ctx, stats.SatelliteID)
			assert.NoError(t, err)

			assert.Equal(t, res.SatelliteID, stats.SatelliteID)
			assert.Equal(t, res.Disqualified, stats.Disqualified)
			assert.Equal(t, res.UpdatedAt, stats.UpdatedAt)

			compareReputationMetric(t, &res.Uptime, &stats.Uptime)
			compareReputationMetric(t, &res.Audit, &stats.Audit)
		})
	})
}

func TestReputationDBGetAll(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		reputationDB := db.Reputation()

		var stats []reputation.Stats
		for i := 0; i < 10; i++ {
			timestamp := time.Now().UTC().Add(time.Hour * time.Duration(i))

			rep := reputation.Stats{
				SatelliteID: testrand.NodeID(),
				Uptime: reputation.Metric{
					TotalCount:   int64(i + 1),
					SuccessCount: int64(i + 2),
					Alpha:        float64(i + 3),
					Beta:         float64(i + 4),
					Score:        float64(i + 5),
				},
				Audit: reputation.Metric{
					TotalCount:   int64(i + 6),
					SuccessCount: int64(i + 7),
					Alpha:        float64(i + 8),
					Beta:         float64(i + 9),
					Score:        float64(i + 10),
				},
				Disqualified: &timestamp,
				UpdatedAt:    timestamp,
			}

			err := reputationDB.Store(ctx, rep)
			require.NoError(t, err)

			stats = append(stats, rep)
		}

		res, err := reputationDB.All(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, len(stats), len(res))

		for _, rep := range res {
			assert.Contains(t, stats, rep)

			if rep.SatelliteID == stats[0].SatelliteID {
				assert.Equal(t, rep.Disqualified, stats[0].Disqualified)
				assert.Equal(t, rep.UpdatedAt, stats[0].UpdatedAt)

				compareReputationMetric(t, &rep.Uptime, &stats[0].Uptime)
				compareReputationMetric(t, &rep.Audit, &stats[0].Audit)
			}
		}
	})
}

// compareReputationMetric compares two reputation metrics and asserts that they are equal
func compareReputationMetric(t *testing.T, a, b *reputation.Metric) {
	assert.Equal(t, a.SuccessCount, b.SuccessCount)
	assert.Equal(t, a.TotalCount, b.TotalCount)
	assert.Equal(t, a.Alpha, b.Alpha)
	assert.Equal(t, a.Beta, b.Beta)
	assert.Equal(t, a.Score, b.Score)
}
