// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestReputationDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		reputationDB := db.Reputation()

		stats := reputation.Stats{
			SatelliteID: testrand.NodeID(),
			Uptime: reputation.Metric{
				TotalCount:   1,
				SuccessCount: 1,
				Alpha:        1,
				Beta:         1,
				Score:        1,
			},
			Audit: reputation.Metric{
				TotalCount:   2,
				SuccessCount: 2,
				Alpha:        2,
				Beta:         2,
				Score:        2,
			},
			Disqualified: nil,
			UpdatedAt:    time.Now().UTC(),
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

// compareReputationMetric compares two reputation metrics and asserts that they are equal
func compareReputationMetric(t *testing.T, a, b *reputation.Metric) {
	assert.Equal(t, a.SuccessCount, b.SuccessCount)
	assert.Equal(t, a.TotalCount, b.TotalCount)
	assert.Equal(t, a.Alpha, b.Alpha)
	assert.Equal(t, a.Beta, b.Beta)
	assert.Equal(t, a.Score, b.Score)
}
