// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/shared/location"
	"storj.io/storj/shared/nodetag"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/contact"
)

func TestSegmentRepairPlacement(t *testing.T) {
	piecesCount := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 12, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(1, 1, piecesCount, piecesCount),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.DoDeclumping = false
					// Disable stray node disqualification because the storage nodes' contact chores are paused.
					config.StrayNodes.EnableDQ = false
				},
			),
		},
		Timeout:      -1,
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// disable pinging the Satellite so we can control storagenode status.
		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
		}

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "testbucket"))
		defaultLocation := location.Poland

		_, err := planet.Satellites[0].API.Buckets.Service.UpdateBucket(ctx, buckets.Bucket{
			ProjectID: planet.Uplinks[0].Projects[0].ID,
			Name:      "testbucket",
			Placement: storj.EU,
		})
		require.NoError(t, err)

		type testCase struct {
			piecesOutOfPlacement int
			piecesAfterRepair    int

			// how many from out of placement pieces should be also offline
			piecesOutOfPlacementOffline int
		}

		for _, tc := range []testCase{
			// all pieces/nodes are out of placement, repair download/upload should be triggered
			{piecesOutOfPlacement: piecesCount, piecesAfterRepair: piecesCount},

			// all pieces/nodes are out of placement, repair download/upload should be triggered, some pieces are offline
			{piecesOutOfPlacement: piecesCount, piecesAfterRepair: piecesCount, piecesOutOfPlacementOffline: 1},
			{piecesOutOfPlacement: piecesCount, piecesAfterRepair: piecesCount, piecesOutOfPlacementOffline: 2},

			// few pieces/nodes are out of placement, repair download/upload should be triggered
			{piecesOutOfPlacement: piecesCount - 1, piecesAfterRepair: piecesCount},
			{piecesOutOfPlacement: piecesCount - 1, piecesAfterRepair: piecesCount, piecesOutOfPlacementOffline: 1},

			// single piece/node is out of placement, NO download/upload repair, we are only removing piece from segment
			// as segment is still above repair threshold
			{piecesOutOfPlacement: 1, piecesAfterRepair: piecesCount - 1},
			{piecesOutOfPlacement: 1, piecesAfterRepair: piecesCount - 1, piecesOutOfPlacementOffline: 1},
		} {

			t.Run(fmt.Sprintf("oop_%d_ar_%d_off_%d", tc.piecesOutOfPlacement, tc.piecesAfterRepair, tc.piecesOutOfPlacementOffline), func(t *testing.T) {
				for _, node := range planet.StorageNodes {
					require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), defaultLocation.String()))
				}

				require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))
				require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))

				expectedData := testrand.Bytes(5 * memory.KiB)
				err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", expectedData)
				require.NoError(t, err)

				segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
				require.NoError(t, err)
				require.Len(t, segments, 1)
				require.Len(t, segments[0].Pieces, piecesCount)

				for index, piece := range segments[0].Pieces {
					// make node offline if needed
					node := planet.FindNode(piece.StorageNode)
					if index < tc.piecesOutOfPlacementOffline {
						t.Logf("marking node %s as offline", node.ID())
						require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], node, true, defaultLocation))
					} else {
						t.Logf("marking node %s as online", node.ID())
						require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], node, false, defaultLocation))
					}

					if index < tc.piecesOutOfPlacement {
						t.Logf("marking node %s as out of placement", node.ID())
						require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, piece.StorageNode, "US"))
					}
				}

				// confirm that some pieces are out of placement

				placement, err := planet.Satellites[0].Config.Placement.Parse(planet.Satellites[0].Config.Overlay.Node.CreateDefaultPlacement, nil)
				require.NoError(t, err)

				ok, err := allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[0].Pieces, segments[0].Placement, placement.CreateFilters)
				require.NoError(t, err)
				require.False(t, ok)

				require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))
				require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))

				t.Log("starting repair")
				_, err = planet.Satellites[0].Repairer.SegmentRepairer.Repair(ctx, queue.InjuredSegment{
					StreamID: segments[0].StreamID,
					Position: segments[0].Position,
				})
				t.Log("repair complete")
				require.NoError(t, err)

				// confirm that all pieces have correct placement
				segments, err = planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
				require.NoError(t, err)
				require.Len(t, segments, 1)
				require.NotNil(t, segments[0].RepairedAt)
				require.Len(t, segments[0].Pieces, tc.piecesAfterRepair)

				ok, err = allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[0].Pieces, segments[0].Placement, placement.CreateFilters)
				require.NoError(t, err)
				require.True(t, ok)

				require.NoError(t, planet.Satellites[0].API.Overlay.Service.DownloadSelectionCache.Refresh(ctx))
				require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))

				data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "object")
				require.NoError(t, err)
				require.Equal(t, expectedData, data)
			})
		}
	})
}

func TestSegmentRepairInMemoryUpload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(1, 1, 2, 2),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryUpload = true
				},
			),
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "testbucket"))

		expectedData := testrand.Bytes(5 * memory.KiB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", expectedData)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.Len(t, segments[0].Pieces, 2)

		require.NoError(t, planet.StopNodeAndUpdate(ctx, planet.FindNode(segments[0].Pieces[0].StorageNode)))
		require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))

		_, err = planet.Satellites[0].Repairer.SegmentRepairer.Repair(ctx, queue.InjuredSegment{
			StreamID: segments[0].StreamID,
			Position: segments[0].Position,
		})
		require.NoError(t, err)

		segmentsAfter, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.NotNil(t, segmentsAfter[0].RepairedAt)
		require.NotEqual(t, segments[0].Pieces, segmentsAfter[0].Pieces)

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "object")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)
	})
}

func TestSegmentRepairWithNodeTags(t *testing.T) {
	t.Skip("flaky")

	satelliteIdentity := signing.SignerFromFullIdentity(testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()))
	testplanet.Run(t, testplanet.Config{
		// we use 23 nodes:
		//      first 0-9: untagged
		//      next 10-19: tagged, used to upload (remaining should be offline during first upload)
		//      next 20-22: tagged, used to upload during repair (4 should be offline from the previous set: we will have 6 pieces + 3 new to these)
		SatelliteCount: 1, StorageNodeCount: 23, UplinkCount: 1,
		Timeout: -1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					tag := fmt.Sprintf(`tag("%s","selected","true")`, satelliteIdentity.ID())
					config.Placement = nodeselection.ConfigurablePlacementRule{
						PlacementRules: fmt.Sprintf("0:exclude(%s);10:%s", tag, tag),
					}
				},
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Overlay.Node.AsOfSystemTime.Enabled = false
				},
				testplanet.ReconfigureRS(4, 6, 8, 10),
			),
			StorageNode: func(index int, config *storagenode.Config) {
				if index >= 10 {
					tags := &pb.NodeTagSet{
						NodeId:   testidentity.MustPregeneratedSignedIdentity(index+1, storj.LatestIDVersion()).ID.Bytes(),
						SignedAt: time.Now().Unix(),
						Tags: []*pb.Tag{
							{
								Name:  "selected",
								Value: []byte("true"),
							},
						},
					}

					signed, err := nodetag.Sign(t.Context(), tags, satelliteIdentity)
					require.NoError(t, err)

					config.Contact.Tags = contact.SignedTags(pb.SignedNodeTagSets{
						Tags: []*pb.SignedNodeTagSet{
							signed,
						},
					})
				}

				// make sure we control the checking requests, and they won't be updated
				config.Contact.Interval = 60 * time.Minute

			},
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// disable pinging the Satellite so we can control storagenode status.
		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
		}

		allTaggedNodes := []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}

		{
			// create two buckets: one normal, one with placement=10

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "generic"))
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "selected"))

			_, err := planet.Satellites[0].API.Buckets.Service.UpdateBucket(ctx, buckets.Bucket{
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Name:      "selected",
				Placement: 10,
			})
			require.NoError(t, err)
		}

		{
			// these nodes will be used during the repair, let's make them offline to make sure, they don't have any pieces
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[20], true, location.Germany))
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[21], true, location.Germany))
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[22], true, location.Germany))

			require.NoError(t, planet.Satellites[0].Overlay.Service.UploadSelectionCache.Refresh(ctx))
			require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))
		}

		expectedData := testrand.Bytes(5 * memory.KiB)
		{
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "selected", "object", expectedData)
			require.NoError(t, err)
		}

		{
			// check the right placement

			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 1)

			placement, err := planet.Satellites[0].Config.Placement.Parse(planet.Satellites[0].Config.Overlay.Node.CreateDefaultPlacement, nil)
			require.NoError(t, err)

			require.Equal(t, storj.PlacementConstraint(10), segments[0].Placement)
			ok, err := allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[0].Pieces, segments[0].Placement, placement.CreateFilters)
			require.NoError(t, err)
			require.True(t, ok)

			err = piecesOnNodeByIndex(planet, segments[0].Pieces, allTaggedNodes)
			require.NoError(t, err)
		}

		{
			// 4 offline nodes should trigger a new repair (6 pieces available)
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[16], true, location.Germany))
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[17], true, location.Germany))
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[18], true, location.Germany))
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[19], true, location.Germany))

			// we need 4 more online (tagged) nodes to repair, let's turn them on
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[20], false, location.Germany))
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[21], false, location.Germany))
			require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.StorageNodes[22], false, location.Germany))

			require.NoError(t, planet.Satellites[0].Repairer.Overlay.UploadSelectionCache.Refresh(ctx))
			require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))
		}

		{
			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			_, err = planet.Satellites[0].Repairer.SegmentRepairer.Repair(ctx, queue.InjuredSegment{
				StreamID: segments[0].StreamID,
				Position: segments[0].Position,
			})
			require.NoError(t, err)
		}

		{
			// check the right placement
			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 1)

			err = piecesOnNodeByIndex(planet, segments[0].Pieces, allTaggedNodes)
			require.NoError(t, err)
		}

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "selected", "object")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

	})
}

func TestSegmentRepairPlacementAndClumped(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 8, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// disable pinging the Satellite so we can control storagenode status.
		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
		}

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "testbucket"))

		_, err := planet.Satellites[0].API.Buckets.Service.UpdateBucket(ctx, buckets.Bucket{
			ProjectID: planet.Uplinks[0].Projects[0].ID,
			Name:      "testbucket",
			Placement: storj.EU,
		})
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), "PL"))
		}

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), "PL"))
		}

		require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))
		require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.Len(t, segments[0].Pieces, 4)

		// set nodes to the same placement/country and put all nodes into the same net to mark them as clumped
		node0 := planet.FindNode(segments[0].Pieces[0].StorageNode)
		for _, piece := range segments[0].Pieces {
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, piece.StorageNode, "US"))

			local := node0.Contact.Service.Local()
			checkInInfo := overlay.NodeCheckInInfo{
				NodeID:     piece.StorageNode,
				Address:    &pb.NodeAddress{Address: local.Address},
				LastIPPort: local.Address,
				LastNet:    node0.Contact.Service.Local().Address,
				IsUp:       true,
				Operator:   &local.Operator,
				Capacity:   &local.Capacity,
				Version:    &local.Version,
			}
			err = planet.Satellites[0].DB.OverlayCache().UpdateCheckIn(ctx, checkInInfo, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
		}

		placement, err := planet.Satellites[0].Config.Placement.Parse(planet.Satellites[0].Config.Overlay.Node.CreateDefaultPlacement, nil)
		require.NoError(t, err)

		// confirm that some pieces are out of placement
		ok, err := allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[0].Pieces, segments[0].Placement, placement.CreateFilters)
		require.NoError(t, err)
		require.False(t, ok)

		require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))
		require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))

		_, err = planet.Satellites[0].Repairer.SegmentRepairer.Repair(ctx, queue.InjuredSegment{
			StreamID: segments[0].StreamID,
			Position: segments[0].Position,
		})
		require.NoError(t, err)

		// confirm that all pieces have correct placement
		segments, err = planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.NotNil(t, segments[0].RepairedAt)
		require.Len(t, segments[0].Pieces, 4)

		ok, err = allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[0].Pieces, segments[0].Placement, placement.CreateFilters)
		require.NoError(t, err)
		require.True(t, ok)
	})
}

func TestSegmentRepairPlacementNotEnoughNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(1, 2, 4, 4),
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// disable pinging the Satellite so we can control storagenode placement
		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
		}

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "testbucket"))

		_, err := planet.Satellites[0].API.Buckets.Service.UpdateBucket(ctx, buckets.Bucket{
			ProjectID: planet.Uplinks[0].Projects[0].ID,
			Name:      "testbucket",
			Placement: storj.EU,
		})
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), "PL"))
		}

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		// change all nodes location to US
		for _, node := range planet.StorageNodes {
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), "US"))
		}

		require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))
		require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.Len(t, segments[0].Pieces, 4)

		// we have bucket geofenced to EU but now all nodes are in US, repairing should fail because
		// not enough nodes are available but segment shouldn't be deleted from repair queue
		shouldDelete, err := planet.Satellites[0].Repairer.SegmentRepairer.Repair(ctx, queue.InjuredSegment{
			StreamID: segments[0].StreamID,
			Position: segments[0].Position,
		})
		require.True(t, overlay.ErrNotEnoughNodes.Has(err))
		require.False(t, shouldDelete)
	})
}

func piecesOnNodeByIndex(planet *testplanet.Planet, pieces metabase.Pieces, allowedIndexes []int) error {

	findIndex := func(id storj.NodeID) int {
		for ix, storagenode := range planet.StorageNodes {
			if storagenode.ID() == id {
				return ix
			}
		}
		return -1
	}

	intInSlice := func(allowedNumbers []int, num int) bool {
		for _, n := range allowedNumbers {
			if n == num {
				return true
			}

		}
		return false
	}

	for _, piece := range pieces {
		ix := findIndex(piece.StorageNode)
		if ix == -1 || !intInSlice(allowedIndexes, ix) {
			return errs.New("piece is on storagenode (%s, %d) which is not whitelisted", piece.StorageNode, ix)
		}
	}
	return nil

}

func allPiecesInPlacement(ctx context.Context, overlay *overlay.Service, pieces metabase.Pieces, placement storj.PlacementConstraint, rules nodeselection.PlacementRules) (bool, error) {
	filter, _ := rules(placement)
	for _, piece := range pieces {
		nodeDossier, err := overlay.Get(ctx, piece.StorageNode)
		if err != nil {
			return false, err
		}
		tags, err := overlay.GetNodeTags(ctx, piece.StorageNode)
		if err != nil {
			return false, err
		}
		node := &nodeselection.SelectedNode{
			ID:          nodeDossier.Id,
			CountryCode: nodeDossier.CountryCode,
			Tags:        tags,
		}

		if !filter.Match(node) {
			return false, nil
		}
	}
	return true, nil
}

func updateNodeStatus(ctx context.Context, satellite *testplanet.Satellite, node *testplanet.StorageNode, offline bool, countryCode location.CountryCode) error {
	timestamp := time.Now()
	if offline {
		timestamp = time.Now().Add(-4 * time.Hour)
	}

	return satellite.DB.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
		NodeID:  node.ID(),
		Address: &pb.NodeAddress{Address: node.Addr()},
		LastNet: node.Addr(),
		IsUp:    true,
		Version: &pb.NodeVersion{
			Version:    "v0.0.0",
			CommitHash: "",
			Timestamp:  time.Time{},
			Release:    true,
		},
		Capacity: &pb.NodeCapacity{
			FreeDisk: 1 * memory.GiB.Int64(),
		},
		CountryCode: countryCode,
	}, timestamp, satellite.Config.Overlay.Node)
}

// this test creates two keys with two different placement (technically both are PL country restrictions, but different ID)
// when both are placed to wrong nodes (nodes are moved to wrong country), only one of them will be repaired, as repairer
// is configured to include only that placement constraint.
func TestSegmentRepairPlacementRestrictions(t *testing.T) {
	t.Skip("flaky")

	placement := nodeselection.ConfigurablePlacementRule{}
	err := placement.Set(`1:country("PL");2:country("PL")`)
	require.NoError(t, err)

	piecesCount := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 8, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(1, 1, piecesCount, piecesCount),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.DoDeclumping = false
					config.Placement = placement
					config.Repairer.IncludedPlacements = repairer.PlacementList{
						Placements: []storj.PlacementConstraint{1},
					}
					// only on-demand execution
					config.RangedLoop.Interval = 10 * time.Hour
				},
			),
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// disable pinging the Satellite so we can control storagenode placement
		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
		}

		placement, err := planet.Satellites[0].Config.Placement.Parse(planet.Satellites[0].Config.Overlay.Node.CreateDefaultPlacement, nil)
		require.NoError(t, err)

		{
			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "testbucket1"))
			_, err := planet.Satellites[0].API.Buckets.Service.UpdateBucket(ctx, buckets.Bucket{
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Name:      "testbucket1",
				Placement: storj.PlacementConstraint(1),
			})
			require.NoError(t, err)

			require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "testbucket2"))
			_, err = planet.Satellites[0].API.Buckets.Service.UpdateBucket(ctx, buckets.Bucket{
				ProjectID: planet.Uplinks[0].Projects[0].ID,
				Name:      "testbucket2",
				Placement: storj.PlacementConstraint(2),
			})
			require.NoError(t, err)
		}

		goodLocation := location.Poland
		badLocation := location.Germany

		{
			// both upload will use only the first 4 nodes, as we have the right nodes there
			for ix, node := range planet.StorageNodes {
				l := goodLocation
				if ix > 3 {
					l = badLocation
				}
				require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), l.String()))

			}
			require.NoError(t, planet.Satellites[0].Repairer.Overlay.UploadSelectionCache.Refresh(ctx))
			require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))
			require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))
		}

		expectedData := testrand.Bytes(5 * memory.KiB)
		{

			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket1", "object", expectedData)
			require.NoError(t, err)

			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket2", "object", expectedData)
			require.NoError(t, err)
		}

		{
			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 2)
			require.Len(t, segments[0].Pieces, piecesCount)

			// confirm that  pieces are at the good place
			for i := 0; i < 2; i++ {
				ok, err := allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[i].Pieces, segments[i].Placement, placement.CreateFilters)
				require.NoError(t, err)
				require.True(t, ok)
			}
		}

		{
			// time to move the current nodes out of the right country
			for ix, node := range planet.StorageNodes {
				l := goodLocation
				if ix < 4 {
					l = badLocation
				}
				require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), l.String()))

			}
			require.NoError(t, planet.Satellites[0].Repairer.Overlay.UploadSelectionCache.Refresh(ctx))
			require.NoError(t, planet.Satellites[0].Repairer.SegmentRepairer.RefreshParticipatingNodesCache(ctx))
		}

		{
			// confirm that there are out of placement pieces
			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 2)
			require.Len(t, segments[0].Pieces, piecesCount)

			ok, err := allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[0].Pieces, segments[0].Placement, placement.CreateFilters)
			require.NoError(t, err)
			require.False(t, ok)
		}

		{
			// hey repair-checker, do you see any problems?
			planet.Satellites[0].RangedLoop.RangedLoop.Service.Loop.TriggerWait()

			// we should see both segments in repair queue
			n, err := planet.Satellites[0].Repair.Queue.SelectN(ctx, 10)
			require.NoError(t, err)
			require.Len(t, n, 2)
		}

		{
			// this should repair only one segment (where placement=1)
			planet.Satellites[0].Repairer.Repairer.Loop.TriggerWait()
			require.NoError(t, planet.Satellites[0].Repairer.Repairer.WaitForPendingRepairs(ctx))

			// one of the segments are repaired
			n, err := planet.Satellites[0].Repair.Queue.SelectN(ctx, 10)
			require.NoError(t, err)
			require.Len(t, n, 1)

			// segment no2 is still in the repair queue
			require.Equal(t, storj.PlacementConstraint(2), n[0].Placement)

			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 2)
			require.Len(t, segments[0].Pieces, piecesCount)

			require.NotEqual(t, segments[0].Placement, segments[1].Placement)
			for _, segment := range segments {
				ok, err := allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segment.Pieces, segment.Placement, placement.CreateFilters)
				require.NoError(t, err)

				if segment.Placement == 1 {
					require.True(t, ok, "Segment is at wrong place %s", segment.StreamID)
				} else {
					require.False(t, ok)
				}
			}

		}

		// download is still working
		{
			// this is repaired
			data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket1", "object")
			require.NoError(t, err)
			require.Equal(t, expectedData, data)

			// this is not repaired, wrong nodes are filtered out during download --> error
			_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket2", "object")
			require.Error(t, err)

		}

	})
}
