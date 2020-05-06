// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

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

		for i, id := range all {
			addr := fmt.Sprintf("127.0.%d.0:8080", i)
			lastNet := fmt.Sprintf("127.0.%d", i)
			d := overlay.NodeCheckInInfo{
				NodeID:     id,
				Address:    &pb.NodeAddress{Address: addr},
				LastIPPort: addr,
				LastNet:    lastNet,
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
				IsUp:       true,
			}
			err := overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
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

		b.Run("UpdateCheckIn", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				addr := fmt.Sprintf("127.0.%d.0:8080", i)
				lastNet := fmt.Sprintf("127.0.%d", i)
				d := overlay.NodeCheckInInfo{
					NodeID:     id,
					Address:    &pb.NodeAddress{Address: addr},
					LastIPPort: addr,
					LastNet:    lastNet,
					Version:    &pb.NodeVersion{Version: "v1.0.0"},
				}
				err := overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
				require.NoError(b, err)
			}
		})

		b.Run("UpdateStats", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				outcome := overlay.AuditFailure
				if i&1 == 0 {
					outcome = overlay.AuditSuccess
				}
				_, err := overlaydb.UpdateStats(ctx, &overlay.UpdateRequest{
					NodeID:       id,
					AuditOutcome: outcome,
					IsUp:         i&2 == 0,
				})
				require.NoError(b, err)
			}
		})

		b.Run("BatchUpdateStats", func(b *testing.B) {
			var updateRequests []*overlay.UpdateRequest
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				outcome := overlay.AuditFailure
				if i&1 == 0 {
					outcome = overlay.AuditSuccess
				}
				updateRequests = append(updateRequests, &overlay.UpdateRequest{
					NodeID:       id,
					AuditOutcome: outcome,
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
						FreeDisk: 1000,
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
				_, err := overlaydb.UpdateUptime(ctx, id, i&1 == 0)
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
						FreeDisk: int64(i),
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
					overlay.NodeSelectionConfig{},
				)
				require.NoError(b, err)
			}
		})
	})
}

func BenchmarkNodeSelection(b *testing.B) {
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		const (
			Total       = 10000
			Offline     = 1000
			NodesPerNet = 2

			SelectCount   = 100
			ExcludedCount = 90

			newNodeFraction = 0.05
		)

		SelectNewCount := int(100 * newNodeFraction)

		now := time.Now()
		twoHoursAgo := now.Add(-2 * time.Hour)

		overlaydb := db.OverlayCache()
		ctx := context.Background()

		nodeSelectionConfig := overlay.NodeSelectionConfig{
			AuditCount:       1,
			NewNodeFraction:  newNodeFraction,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 0,
		}

		var excludedIDs []storj.NodeID
		var excludedNets []string

		for i := 0; i < Total/NodesPerNet; i++ {
			for k := 0; k < NodesPerNet; k++ {
				nodeID := testrand.NodeID()
				address := fmt.Sprintf("127.%d.%d.%d", byte(i>>8), byte(i), byte(k))
				lastNet := fmt.Sprintf("127.%d.%d.0", byte(i>>8), byte(i))

				if i < ExcludedCount && k == 0 {
					excludedIDs = append(excludedIDs, nodeID)
					excludedNets = append(excludedNets, lastNet)
				}

				addr := address + ":12121"
				d := overlay.NodeCheckInInfo{
					NodeID:     nodeID,
					Address:    &pb.NodeAddress{Address: addr},
					LastIPPort: addr,
					LastNet:    lastNet,
					Version:    &pb.NodeVersion{Version: "v1.0.0"},
					Capacity: &pb.NodeCapacity{
						FreeDisk: 1_000_000_000,
					},
				}
				err := overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
				require.NoError(b, err)

				_, err = overlaydb.UpdateNodeInfo(ctx, nodeID, &pb.InfoResponse{
					Type: pb.NodeType_STORAGE,
					Capacity: &pb.NodeCapacity{
						FreeDisk: 1_000_000_000,
					},
					Version: &pb.NodeVersion{
						Version:   "v1.0.0",
						Timestamp: now,
						Release:   true,
					},
				})
				require.NoError(b, err)

				if i%2 == 0 { // make half of nodes "new" and half "vetted"
					_, err = overlaydb.UpdateStats(ctx, &overlay.UpdateRequest{
						NodeID:       nodeID,
						IsUp:         true,
						AuditOutcome: overlay.AuditSuccess,
						AuditLambda:  1,
						AuditWeight:  1,
						AuditDQ:      0.5,
					})
					require.NoError(b, err)
				}

				if i > Total-Offline {
					switch i % 3 {
					case 0:
						err := overlaydb.SuspendNode(ctx, nodeID, now)
						require.NoError(b, err)
					case 1:
						err := overlaydb.DisqualifyNode(ctx, nodeID)
						require.NoError(b, err)
					case 2:
						err := overlaydb.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
							NodeID: nodeID,
							IsUp:   true,
							Address: &pb.NodeAddress{
								Address: address,
							},
							Operator: nil,
							Version:  nil,
						}, twoHoursAgo, nodeSelectionConfig)
						require.NoError(b, err)
					}
				}
			}
		}

		criteria := &overlay.NodeCriteria{
			FreeDisk:         0,
			AuditCount:       1,
			UptimeCount:      0,
			ExcludedIDs:      nil,
			ExcludedNetworks: nil,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     time.Hour,
			DistinctIP:       false,
		}
		excludedCriteria := &overlay.NodeCriteria{
			FreeDisk:         0,
			AuditCount:       1,
			UptimeCount:      0,
			ExcludedIDs:      excludedIDs,
			ExcludedNetworks: excludedNets,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     time.Hour,
			DistinctIP:       false,
		}

		b.Run("SelectStorageNodes", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := overlaydb.SelectStorageNodes(ctx, SelectCount, 0, criteria)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("SelectNewStorageNodes", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := overlaydb.SelectStorageNodes(ctx, SelectCount, SelectCount, criteria)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("SelectStorageNodesExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := overlaydb.SelectStorageNodes(ctx, SelectCount, 0, excludedCriteria)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("SelectNewStorageNodesExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := overlaydb.SelectStorageNodes(ctx, SelectCount, SelectCount, excludedCriteria)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("SelectStorageNodesBoth", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := overlaydb.SelectStorageNodes(ctx, SelectCount, SelectNewCount, criteria)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("SelectStorageNodesBothExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := overlaydb.SelectStorageNodes(ctx, SelectCount, SelectNewCount, excludedCriteria)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("GetNodesNetwork", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				excludedNetworks, err := overlaydb.GetNodesNetwork(ctx, excludedIDs)
				require.NoError(b, err)
				require.NotEmpty(b, excludedNetworks)
			}
		})

		service := overlay.NewService(zap.NewNop(), overlaydb, overlay.Config{
			Node: nodeSelectionConfig,
			NodeSelectionCache: overlay.CacheConfig{
				Staleness: time.Hour,
			},
		})

		b.Run("FindStorageNodesWithPreference", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{
					MinimumRequiredNodes: SelectCount,
					RequestedCount:       SelectCount,
					ExcludedIDs:          nil,
					MinimumVersion:       "v1.0.0",
				}, &nodeSelectionConfig)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("FindStorageNodesWithPreferenceExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.FindStorageNodesWithPreferences(ctx, overlay.FindStorageNodesRequest{
					MinimumRequiredNodes: SelectCount,
					RequestedCount:       SelectCount,
					ExcludedIDs:          excludedIDs,
					MinimumVersion:       "v1.0.0",
				}, &nodeSelectionConfig)
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("FindStorageNodes", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
					MinimumRequiredNodes: SelectCount,
					RequestedCount:       SelectCount,
					ExcludedIDs:          nil,
					MinimumVersion:       "v1.0.0",
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("FindStorageNodesExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
					MinimumRequiredNodes: SelectCount,
					RequestedCount:       SelectCount,
					ExcludedIDs:          excludedIDs,
					MinimumVersion:       "v1.0.0",
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("NodeSelectionCacheGetNodes", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.SelectionCache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					MinimumRequiredNodes: SelectCount,
					RequestedCount:       SelectCount,
					ExcludedIDs:          nil,
					MinimumVersion:       "v1.0.0",
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("NodeSelectionCacheGetNodesExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.SelectionCache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					MinimumRequiredNodes: SelectCount,
					RequestedCount:       SelectCount,
					ExcludedIDs:          excludedIDs,
					MinimumVersion:       "v1.0.0",
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})
	})
}
