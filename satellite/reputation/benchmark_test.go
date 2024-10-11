// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func BenchmarkReputation(b *testing.B) {
	satellitedbtest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db satellite.DB) {
		const (
			TotalNodeCount = 211
			OfflineCount   = 10
		)

		reputationdb := db.Reputation()

		var all []storj.NodeID
		for i := 0; i < TotalNodeCount; i++ {
			id := testrand.NodeID()
			all = append(all, id)
		}

		b.Run("UpdateStatsSuccess", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := reputationdb.Update(ctx, reputation.UpdateRequest{
					NodeID:       id,
					AuditOutcome: reputation.AuditSuccess,
					Config: reputation.Config{
						AuditHistory: testAuditHistoryConfig(),
					},
				}, time.Now())
				require.NoError(b, err)
			}
		})

		b.Run("UpdateStatsFailure", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := reputationdb.Update(ctx, reputation.UpdateRequest{
					NodeID:       id,
					AuditOutcome: reputation.AuditFailure,
					Config: reputation.Config{
						AuditHistory: testAuditHistoryConfig(),
					},
				}, time.Now())
				require.NoError(b, err)
			}
		})

		b.Run("UpdateStatsUnknown", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := reputationdb.Update(ctx, reputation.UpdateRequest{
					NodeID:       id,
					AuditOutcome: reputation.AuditUnknown,
					Config: reputation.Config{
						AuditHistory: testAuditHistoryConfig(),
					},
				}, time.Now())
				require.NoError(b, err)
			}
		})

		b.Run("UpdateStatsOffline", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := reputationdb.Update(ctx, reputation.UpdateRequest{
					NodeID:       id,
					AuditOutcome: reputation.AuditOffline,
					Config: reputation.Config{
						AuditHistory: testAuditHistoryConfig(),
					},
				}, time.Now())
				require.NoError(b, err)
			}
		})
	})
}
