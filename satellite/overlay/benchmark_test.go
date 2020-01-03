// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func BenchmarkOverlay(b *testing.B) {
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		const (
			TotalNodeCount = 211
			OnlineCount    = 90
			OfflineCount   = 10
		)

		overlaydb := db.OverlayCache()
		ctx := context.Background()

		var all []storj.NodeID
		var check []storj.NodeID
		for i := 0; i < TotalNodeCount; i++ {
			id := testrand.NodeID()
			all = append(all, id)
			if i < OnlineCount {
				check = append(check, id)
			}
		}

		for _, id := range all {
			err := overlaydb.UpdateAddress(ctx, &pb.Node{Id: id}, overlay.NodeSelectionConfig{})
			require.NoError(b, err)
		}

		// create random offline node ids to check
		for i := 0; i < OfflineCount; i++ {
			check = append(check, testrand.NodeID())
		}

		b.Run("KnownUnreliableOrOffline", func(b *testing.B) {
			criteria := &overlay.NodeCriteria{
				AuditCount:   0,
				OnlineWindow: 1000 * time.Hour,
				UptimeCount:  0,
			}
			for i := 0; i < b.N; i++ {
				badNodes, err := overlaydb.KnownUnreliableOrOffline(ctx, criteria, check)
				require.NoError(b, err)
				require.Len(b, badNodes, OfflineCount)
			}
		})

		b.Run("UpdateAddress", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				err := overlaydb.UpdateAddress(ctx, &pb.Node{Id: id}, overlay.NodeSelectionConfig{})
				require.NoError(b, err)
			}
		})

		b.Run("UpdateStats", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := overlaydb.UpdateStats(ctx, &overlay.UpdateRequest{
					NodeID:       id,
					AuditSuccess: i&1 == 0,
					IsUp:         i&2 == 0,
				})
				require.NoError(b, err)
			}
		})

		b.Run("BatchUpdateStats", func(b *testing.B) {
			var updateRequests []*overlay.UpdateRequest
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				updateRequests = append(updateRequests, &overlay.UpdateRequest{
					NodeID:       id,
					AuditSuccess: i&1 == 0,
					IsUp:         i&2 == 0,
				})

			}
			_, err := overlaydb.BatchUpdateStats(ctx, updateRequests, 100)
			require.NoError(b, err)
		})

		b.Run("UpdateNodeInfo", func(b *testing.B) {
			now := time.Now()
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := overlaydb.UpdateNodeInfo(ctx, id, &pb.InfoResponse{
					Type: pb.NodeType_STORAGE,
					Operator: &pb.NodeOperator{
						Wallet: "0x0123456789012345678901234567890123456789",
						Email:  "a@mail.test",
					},
					Capacity: &pb.NodeCapacity{
						FreeBandwidth: 1000,
						FreeDisk:      1000,
					},
					Version: &pb.NodeVersion{
						Version:    "1.0.0",
						CommitHash: "0",
						Timestamp:  now,
						Release:    false,
					},
				})
				require.NoError(b, err)
			}
		})

		b.Run("UpdateUptime", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := overlaydb.UpdateUptime(ctx, id, i&1 == 0, 1, 1, 0.5)
				require.NoError(b, err)
			}
		})

		b.Run("UpdateCheckIn", func(b *testing.B) {
			now := time.Now()
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				err := overlaydb.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
					NodeID: id,
					Address: &pb.NodeAddress{
						Address: "1.2.4.4",
					},
					IsUp: true,
					Capacity: &pb.NodeCapacity{
						FreeBandwidth: int64(i),
						FreeDisk:      int64(i),
					},
					Operator: &pb.NodeOperator{
						Email:  "a@mail.test",
						Wallet: "0x0123456789012345678901234567890123456789",
					},
					Version: &pb.NodeVersion{
						Version:    "1.0.0",
						CommitHash: "0",
						Timestamp:  now,
						Release:    false,
					},
				},
					now,
					overlay.NodeSelectionConfig{
						UptimeReputationLambda: 0.99,
						UptimeReputationWeight: 1.0,
						UptimeReputationDQ:     0,
					})
				require.NoError(b, err)
			}
		})
	})
}
