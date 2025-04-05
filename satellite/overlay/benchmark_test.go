// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func BenchmarkOverlay(b *testing.B) {
	satellitedbtest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db satellite.DB) {
		const (
			TotalNodeCount = 211
			OnlineCount    = 90
			OfflineCount   = 10
		)

		overlaydb := db.OverlayCache()

		var all []storj.NodeID
		var check []storj.NodeID
		for i := 0; i < TotalNodeCount; i++ {
			id := testrand.NodeID()
			all = append(all, id)
			if i < OnlineCount {
				check = append(check, id)
			}
		}

		nodetags := nodeselection.NodeTags{}
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

			nodetags = append(nodetags,
				nodeselection.NodeTag{
					NodeID:   id,
					SignedAt: time.Now().Add(-time.Hour),
					Signer:   id,
					Name:     "alpha",
					Value:    []byte("alpha"),
				},
				nodeselection.NodeTag{
					NodeID:   id,
					SignedAt: time.Now().Add(-time.Hour),
					Signer:   id,
					Name:     "beta",
					Value:    []byte("beta"),
				},
			)
			require.NoError(b, err)
		}

		err := overlaydb.UpdateNodeTags(ctx, nodetags)
		require.NoError(b, err)

		// create random offline node ids to check
		for i := 0; i < OfflineCount; i++ {
			check = append(check, testrand.NodeID())
		}

		b.Run("GetParticipatingNodes", func(b *testing.B) {
			onlineWindow := 1000 * time.Hour
			for i := 0; i < b.N; i++ {
				selectedNodes, err := overlaydb.GetParticipatingNodes(ctx, check, onlineWindow, 0)
				require.NoError(b, err)
				require.Len(b, selectedNodes, len(check))
				foundOnline := 0
				for _, n := range selectedNodes {
					if n.Online {
						foundOnline++
					}
				}
				require.Equal(b, OnlineCount, foundOnline)
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

		b.Run("UpdateCheckInContended-100x", func(b *testing.B) {
			for k := 0; k < b.N; k++ {
				var g errs2.Group
				for i := 0; i < 100; i++ {
					g.Go(func() error {
						d := overlay.NodeCheckInInfo{
							NodeID:     all[0],
							Address:    &pb.NodeAddress{Address: "127.0.0.0:8080"},
							LastIPPort: "127.0.0.0:8080",
							LastNet:    "127.0.0",
							Operator: &pb.NodeOperator{
								Email:  "hello@example.com",
								Wallet: "123123123123",
							},
							Version: &pb.NodeVersion{Version: "v1.0.0"},
							IsUp:    true,
						}
						return overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
					})
				}
				require.NoError(b, errs.Combine(g.Wait()...))
			}
		})

		b.Run("UpdateNodeInfo", func(b *testing.B) {
			now := time.Now()
			for i := 0; i < b.N; i++ {
				id := all[i%len(all)]
				_, err := overlaydb.UpdateNodeInfo(ctx, id, &overlay.InfoResponse{
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
	satellitedbtest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db satellite.DB) {
		var (
			Total       = 10000
			Offline     = 1000
			NodesPerNet = 2

			SelectCount   = 100
			ExcludedCount = 90

			newNodeFraction = 0.05
		)

		if testing.Short() {
			Total /= 10
			Offline /= 10
			SelectCount /= 10
			ExcludedCount /= 10
		}

		now := time.Now()
		twoHoursAgo := now.Add(-2 * time.Hour)

		overlaydb := db.OverlayCache()

		nodeSelectionConfig := overlay.NodeSelectionConfig{
			NewNodeFraction:  newNodeFraction,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 0,
			AsOfSystemTime: overlay.AsOfSystemTimeConfig{
				Enabled:         true,
				DefaultInterval: -time.Microsecond,
			},
		}

		var excludedIDs []storj.NodeID

		for i := 0; i < Total/NodesPerNet; i++ {
			for k := 0; k < NodesPerNet; k++ {
				nodeID := testrand.NodeID()
				address := fmt.Sprintf("127.%d.%d.%d", byte(i>>8), byte(i), byte(k))
				lastNet := fmt.Sprintf("127.%d.%d.0", byte(i>>8), byte(i))

				if i < ExcludedCount && k == 0 {
					excludedIDs = append(excludedIDs, nodeID)
				}

				addr := address + ":12121"
				d := overlay.NodeCheckInInfo{
					NodeID:     nodeID,
					Address:    &pb.NodeAddress{Address: addr},
					LastIPPort: addr,
					IsUp:       true,
					LastNet:    lastNet,
					Version:    &pb.NodeVersion{Version: "v1.0.0"},
					Capacity: &pb.NodeCapacity{
						FreeDisk: 1_000_000_000,
					},
				}
				err := overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
				require.NoError(b, err)

				_, err = overlaydb.UpdateNodeInfo(ctx, nodeID, &overlay.InfoResponse{
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
					_, err = overlaydb.TestVetNode(ctx, nodeID)
					require.NoError(b, err)
				}

				if i > Total-Offline {
					switch i % 3 {
					case 0:
						err := overlaydb.TestSuspendNodeUnknownAudit(ctx, nodeID, now)
						require.NoError(b, err)
					case 1:
						_, err := overlaydb.DisqualifyNode(ctx, nodeID, time.Now(), overlay.DisqualificationReasonUnknown)
						require.NoError(b, err)
					case 2:
						err := overlaydb.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
							NodeID: nodeID,
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

		service, err := overlay.NewService(zap.NewNop(), overlaydb, db.NodeEvents(), nodeselection.TestPlacementDefinitions(), "", "", overlay.Config{
			Node: nodeSelectionConfig,
			NodeSelectionCache: overlay.UploadSelectionCacheConfig{
				Staleness: time.Hour,
			},
		})
		require.NoError(b, err)

		var background errgroup.Group
		serviceCtx, serviceCancel := context.WithCancel(ctx)
		background.Go(func() error { return errs.Wrap(service.Run(serviceCtx)) })
		defer func() { require.NoError(b, background.Wait()) }()
		defer func() { serviceCancel(); _ = service.Close() }()

		b.Run("FindStorageNodes", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
					RequestedCount:  SelectCount,
					AlreadySelected: nil,
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("FindStorageNodesExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
					RequestedCount: SelectCount,
					ExcludedIDs:    excludedIDs,
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("UploadSelectionCacheGetNodes", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.UploadSelectionCache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					RequestedCount:  SelectCount,
					AlreadySelected: nil,
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})

		b.Run("UploadSelectionCacheGetNodesExclusion", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selected, err := service.UploadSelectionCache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					RequestedCount: SelectCount,
					ExcludedIDs:    excludedIDs,
				})
				require.NoError(b, err)
				require.NotEmpty(b, selected)
			}
		})
	})
}
