// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/repair/checker"
)

func TestObserverIdentifyInjuredSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		repairQueue := planet.Satellites[0].DB.RepairQueue()

		checker.Loop.Pause()
		planet.Satellites[0].Repair.Repairer.Loop.Pause()

		rs := storj.RedundancyScheme{
			RequiredShares: 2,
			RepairShares:   3,
			OptimalShares:  4,
			TotalShares:    5,
			ShareSize:      256,
		}

		projectID := planet.Uplinks[0].Projects[0].ID
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		expectedLocation := metabase.SegmentLocation{
			ProjectID:  projectID,
			BucketName: "test-bucket",
		}

		// add some valid pointers
		for x := 0; x < 10; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("a-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		// add pointer that needs repair
		expectedLocation.ObjectKey = metabase.ObjectKey("b-0")
		b0StreamID := insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), nil)

		// add pointer that is unhealthy, but is expired
		expectedLocation.ObjectKey = metabase.ObjectKey("b-1")
		expiresAt := time.Now().Add(-time.Hour)
		insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), &expiresAt)

		// add some valid pointers
		for x := 0; x < 10; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("c-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		checker.Loop.TriggerWait()

		// check that the unhealthy, non-expired segment was added to the queue
		// and that the expired segment was ignored
		injuredSegment, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)

		require.Equal(t, b0StreamID, injuredSegment.StreamID)

		_, err = repairQueue.Select(ctx)
		require.Error(t, err)
	})
}

func TestObserverIdentifyIrreparableSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		checker.Loop.Stop()

		const numberOfNodes = 10
		pieces := make(metabase.Pieces, 0, numberOfNodes)
		// use online nodes
		for i, storagenode := range planet.StorageNodes {
			pieces = append(pieces, metabase.Piece{
				Number:      uint16(i),
				StorageNode: storagenode.ID(),
			})
		}

		// simulate offline nodes
		expectedLostPieces := make(map[int32]bool)
		for i := len(pieces); i < numberOfNodes; i++ {
			pieces = append(pieces, metabase.Piece{
				Number:      uint16(i),
				StorageNode: storj.NodeID{byte(i)},
			})
			expectedLostPieces[int32(i)] = true
		}

		rs := storj.RedundancyScheme{
			ShareSize:      256,
			RequiredShares: 4,
			RepairShares:   8,
			OptimalShares:  9,
			TotalShares:    10,
		}

		projectID := planet.Uplinks[0].Projects[0].ID
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		expectedLocation := metabase.SegmentLocation{
			ProjectID:  projectID,
			BucketName: "test-bucket",
		}

		// when number of healthy piece is less than minimum required number of piece in redundancy,
		// the piece is considered irreparable but also will be put into repair queue

		expectedLocation.ObjectKey = "piece"
		insertSegment(ctx, t, planet, rs, expectedLocation, pieces, nil)

		expectedLocation.ObjectKey = "piece-expired"
		expiresAt := time.Now().Add(-time.Hour)
		insertSegment(ctx, t, planet, rs, expectedLocation, pieces, &expiresAt)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		// check that single irreparable segment was added repair queue
		repairQueue := planet.Satellites[0].DB.RepairQueue()
		_, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		count, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// check irreparable once again but wait a second
		time.Sleep(1 * time.Second)
		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		expectedLocation.ObjectKey = "piece"
		_, err = planet.Satellites[0].Metabase.DB.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
			ObjectLocation: expectedLocation.Object(),
			Version:        metabase.DefaultVersion,
		})
		require.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		count, err = repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})
}

func TestObserverCleanRepairQueue(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		repairQueue := planet.Satellites[0].DB.RepairQueue()

		checker.Loop.Pause()
		planet.Satellites[0].Repair.Repairer.Loop.Pause()

		rs := storj.RedundancyScheme{
			RequiredShares: 2,
			RepairShares:   3,
			OptimalShares:  4,
			TotalShares:    5,
			ShareSize:      256,
		}

		projectID := planet.Uplinks[0].Projects[0].ID
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		expectedLocation := metabase.SegmentLocation{
			ProjectID:  projectID,
			BucketName: "test-bucket",
		}

		healthyCount := 5
		for i := 0; i < healthyCount; i++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("healthy-%d", i))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}
		unhealthyCount := 5
		unhealthyIDs := make(map[uuid.UUID]struct{})
		for i := 0; i < unhealthyCount; i++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("unhealthy-%d", i))
			unhealthyStreamID := insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), nil)
			unhealthyIDs[unhealthyStreamID] = struct{}{}
		}

		// suspend enough nodes to make healthy pointers unhealthy
		for i := rs.RequiredShares; i < rs.OptimalShares; i++ {
			require.NoError(t, planet.Satellites[0].Overlay.DB.TestSuspendNodeUnknownAudit(ctx, planet.StorageNodes[i].ID(), time.Now()))
		}

		require.NoError(t, planet.Satellites[0].Repair.Checker.RefreshReliabilityCache(ctx))

		// check that repair queue is empty to avoid false positive
		count, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		checker.Loop.TriggerWait()

		// check that the pointers were put into the repair queue
		// and not cleaned up at the end of the checker iteration
		count, err = repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, healthyCount+unhealthyCount, count)

		// unsuspend nodes to make the previously healthy pointers healthy again
		for i := rs.RequiredShares; i < rs.OptimalShares; i++ {
			require.NoError(t, planet.Satellites[0].Overlay.DB.TestUnsuspendNodeUnknownAudit(ctx, planet.StorageNodes[i].ID()))
		}

		require.NoError(t, planet.Satellites[0].Repair.Checker.RefreshReliabilityCache(ctx))

		// The checker will not insert/update the now healthy segments causing
		// them to be removed from the queue at the end of the checker iteration
		checker.Loop.TriggerWait()

		// only unhealthy segments should remain
		count, err = repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, unhealthyCount, count)

		segs, err := repairQueue.SelectN(ctx, count)
		require.NoError(t, err)
		require.Equal(t, len(unhealthyIDs), len(segs))

		for _, s := range segs {
			_, ok := unhealthyIDs[s.StreamID]
			require.True(t, ok)
		}
	})
}

func TestObserverIgnoringCopiedSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplink := planet.Uplinks[0]
		metabaseDB := satellite.Metabase.DB

		checker := satellite.Repair.Checker
		repairQueue := satellite.DB.RepairQueue()

		checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		err := uplink.CreateBucket(ctx, satellite, "test-bucket")
		require.NoError(t, err)

		testData := testrand.Bytes(8 * memory.KiB)
		err = uplink.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		project, err := uplink.OpenProject(ctx, satellite)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		_, err = project.CopyObject(ctx, "testbucket", "test/path", "testbucket", "empty", nil)
		require.NoError(t, err)

		segmentsAfterCopy, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segmentsAfterCopy, 2)

		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(segments[0].Pieces[0].StorageNode))
		require.NoError(t, err)

		checker.Loop.TriggerWait()

		// check that injured segment in repair queue streamID is same that in original segment.
		injuredSegment, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, segments[0].StreamID, injuredSegment.StreamID)

		// check that repair queue has only original segment, and not copied one.
		injuredSegments, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, injuredSegments)
	})
}

func BenchmarkObserverRemoteSegment(b *testing.B) {
	testplanet.Bench(b, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(b *testing.B, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", testrand.Bytes(10*memory.KiB))
		require.NoError(b, err)

		observer := checker.NewCheckerObserver(planet.Satellites[0].Repair.Checker)
		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(b, err)

		loopSegment := &segmentloop.Segment{
			StreamID:   segments[0].StreamID,
			Position:   segments[0].Position,
			CreatedAt:  segments[0].CreatedAt,
			ExpiresAt:  segments[0].ExpiresAt,
			Redundancy: segments[0].Redundancy,
			Pieces:     segments[0].Pieces,
		}

		b.Run("healthy segment", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := observer.RemoteSegment(ctx, loopSegment)
				if err != nil {
					b.FailNow()
				}
			}
		})
	})

}

func TestRepairObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		repairChecker := planet.Satellites[0].Repair.Checker
		repairChecker.Loop.Pause()
		planet.Satellites[0].Repair.Repairer.Loop.Pause()

		rs := storj.RedundancyScheme{
			RequiredShares: 2,
			RepairShares:   3,
			OptimalShares:  4,
			TotalShares:    5,
			ShareSize:      256,
		}
		projectID := planet.Uplinks[0].Projects[0].ID

		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		expectedLocation := metabase.SegmentLocation{
			ProjectID:  projectID,
			BucketName: "test-bucket",
		}

		// add some valid pointers
		for x := 0; x < 10; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("a-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		// add pointer that needs repair
		expectedLocation.ObjectKey = metabase.ObjectKey("b-0")
		injuredSegmentStreamID := insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), nil)

		// add pointer that is unhealthy, but is expired
		expectedLocation.ObjectKey = metabase.ObjectKey("b-1")
		expiresAt := time.Now().Add(-time.Hour)
		_ = insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), &expiresAt)

		// add some valid pointers
		for x := 0; x < 10; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("c-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		observer := planet.Satellites[0].Repair.Observer
		p, err := observer.Fork(ctx)
		require.NoError(t, err)
		require.NotNil(t, p)

		rawSegments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)

		var segments []segmentloop.Segment
		for _, v := range rawSegments {
			segments = append(segments, segmentloop.Segment{
				StreamID:      v.StreamID,
				Position:      v.Position,
				CreatedAt:     v.CreatedAt,
				ExpiresAt:     v.ExpiresAt,
				RepairedAt:    v.RepairedAt,
				RootPieceID:   v.RootPieceID,
				EncryptedSize: v.EncryptedSize,
				PlainOffset:   v.PlainOffset,
				PlainSize:     v.PlainSize,
				Redundancy:    v.Redundancy,
				Pieces:        v.Pieces,
				Placement:     v.Placement,
			})
		}

		err = p.Process(ctx, segments)
		require.NoError(t, err)

		err = observer.Join(ctx, p)
		require.NoError(t, err)

		err = observer.Finish(ctx)
		require.NoError(t, err)
		require.NoError(t, observer.TotalStats.Compare(21, 21, 1, 1, 0, 0, nil))
		require.NoError(t, observer.CompareInjuredSegment(ctx, []uuid.UUID{injuredSegmentStreamID}))
	})
}

func TestRangedLoopObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		rs := storj.RedundancyScheme{
			RequiredShares: 2,
			RepairShares:   3,
			OptimalShares:  4,
			TotalShares:    5,
			ShareSize:      256,
		}

		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		expectedLocation := metabase.SegmentLocation{
			ProjectID:  planet.Uplinks[0].Projects[0].ID,
			BucketName: "test-bucket",
		}

		// add some valid pointers
		for x := 0; x < 20; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("a-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		var injuredSegmentStreamIDs []uuid.UUID

		// add pointer that needs repair
		for x := 0; x < 5; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("b-%d", x))
			injuredSegmentStreamID := insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), nil)
			injuredSegmentStreamIDs = append(injuredSegmentStreamIDs, injuredSegmentStreamID)
		}

		// add pointer that is unhealthy, but is expired
		expectedLocation.ObjectKey = metabase.ObjectKey("d-1")
		expiresAt := time.Now().Add(-time.Hour)
		insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), &expiresAt)

		// add some valid pointers
		for x := 0; x < 20; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("c-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		type TestCase struct {
			BatchSize   int
			Parallelism int
		}

		// new segments to will be detected only on 1st test case
		firstRun := true
		var newSegmentsNeedRepair int64

		for _, tc := range []TestCase{
			{1, 1},
			{3, 1},
			{5, 1},
			{1, 3},
			{3, 3},
			{5, 3},
			{1, 5},
			{3, 5},
			{5, 5},
		} {
			observer := planet.Satellites[0].Repair.Observer
			service := rangedloop.NewService(planet.Log(), rangedloop.Config{
				Parallelism: tc.Parallelism,
				BatchSize:   tc.BatchSize,
			}, rangedloop.NewMetabaseRangeSplitter(planet.Satellites[0].Metabase.DB, planet.Satellites[0].Config.RangedLoop.BatchSize), []rangedloop.Observer{observer})
			_, err = service.RunOnce(ctx)
			require.NoError(t, err)

			// if first testcase run - all segments to repair counts as new
			if firstRun {
				newSegmentsNeedRepair = 5
				firstRun = false
			}

			require.NoError(t, observer.TotalStats.Compare(45, 45, 5, newSegmentsNeedRepair, 0, 0, nil))
			require.NoError(t, observer.CompareInjuredSegment(ctx, injuredSegmentStreamIDs))
			newSegmentsNeedRepair = 0
		}
	})
}
