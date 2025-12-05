// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
)

func TestIdentifyInjuredSegmentsObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable ranged loop interval execution
				config.RangedLoop.Interval = -1
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4

				// disable repairer to not interfere with the test
				// as it can drain from the queue in the background
				config.Repairer.Interval = -1
			},
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		repairQueue := planet.Satellites[0].Repair.Queue

		rs := storj.RedundancyScheme{
			RequiredShares: 2,
			RepairShares:   3,
			OptimalShares:  4,
			TotalShares:    5,
			ShareSize:      256,
		}

		projectID := planet.Uplinks[0].Projects[0].ID
		err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		expectedLocation := metabase.SegmentLocation{
			ProjectID:  projectID,
			BucketName: "test-bucket",
		}

		// add some valid segments
		for x := range 10 {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("a-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		// add segments that needs repair
		expectedLocation.ObjectKey = metabase.ObjectKey("b-0")
		b0StreamID := insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), nil)

		// add segment that is unhealthy, but is expired
		expectedLocation.ObjectKey = metabase.ObjectKey("b-1")
		expiresAt := time.Now().Add(-time.Hour)
		insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), &expiresAt)

		// add some valid pointers
		for x := range 10 {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("c-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// check that the unhealthy, non-expired segment was added to the queue
		// and that the expired segment was ignored
		injuredSegments, err := repairQueue.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		err = repairQueue.Release(ctx, injuredSegments[0], true)
		require.NoError(t, err)

		require.Equal(t, b0StreamID, injuredSegments[0].StreamID)

		_, err = repairQueue.Select(ctx, 1, nil, nil)
		require.Error(t, err)
	})
}

func TestIdentifyIrreparableSegmentsObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable ranged loop interval execution
				config.RangedLoop.Interval = -1
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4

				// disable repairer to not interfere with the test
				// as it can drain from the queue in the background
				config.Repairer.Interval = -1
			},
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		rangeLoopService := planet.Satellites[0].RangedLoop.RangedLoop.Service

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
		err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test-bucket")
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

		_, err = rangeLoopService.RunOnce(ctx)
		require.NoError(t, err)

		// check that single irreparable segment was added repair queue
		repairQueue := planet.Satellites[0].Repair.Queue
		items, err := repairQueue.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		require.Len(t, items, 1)
		err = repairQueue.Release(ctx, items[0], false)
		require.NoError(t, err)
		count, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// check irreparable once again
		_, err = rangeLoopService.RunOnce(ctx)
		require.NoError(t, err)

		count, err = repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		expectedLocation.ObjectKey = "piece"
		_, err = planet.Satellites[0].Metabase.DB.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
			ObjectLocation: expectedLocation.Object(),
			Version:        metabase.DefaultVersion,
		})
		require.NoError(t, err)

		// The repair checker runs a repair queue "Clean" operation when it has
		// completed a full iteration. It removes all jobs that have not been
		// changed since before the repair queue started. Sleeping here allows a
		// separation of timestamps that makes the Clean work as expected.
		// TODO: get rid of this.
		time.Sleep(time.Second)

		_, err = rangeLoopService.RunOnce(ctx)
		require.NoError(t, err)

		count, err = repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})
}

func TestObserver_CheckSegmentCopy(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable ranged loop interval execution
				config.RangedLoop.Interval = -1
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4
				config.Metainfo.RS.Min = 2
				config.Metainfo.RS.Repair = 3
				config.Metainfo.RS.Success = 4
				config.Metainfo.RS.Total = 4

				// disable repairer to not interfere with the test
				// as it can drain from the queue in the background
				config.Repairer.Interval = -1
			},
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplink := planet.Uplinks[0]
		metabaseDB := satellite.Metabase.DB

		rangedLoopService := planet.Satellites[0].RangedLoop.RangedLoop.Service
		repairQueue := satellite.Repair.Queue

		err := uplink.TestingCreateBucket(ctx, satellite, "test-bucket")
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

		_, err = rangedLoopService.RunOnce(ctx)
		require.NoError(t, err)

		ensureExistsOnce := func(t *testing.T, got *queue.InjuredSegment) {
			for i, s := range segmentsAfterCopy {
				if s.StreamID == got.StreamID && s.Position == got.Position {
					segmentsAfterCopy = slices.Delete(segmentsAfterCopy, i, i+1)
					return
				}
			}
			t.Fatal("segment not found")
		}

		// check that repair queue has original segment and copied one as it has exactly the same metadata
		for range 2 {
			injuredSegments, err := repairQueue.Select(ctx, 1, nil, nil)
			require.NoError(t, err)
			ensureExistsOnce(t, &injuredSegments[0])
			err = repairQueue.Release(ctx, injuredSegments[0], false)
			require.NoError(t, err)
		}

		injuredSegments, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, injuredSegments)
	})
}

func TestCleanRepairQueueObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable ranged loop interval execution
				config.RangedLoop.Interval = -1
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4

				// disable repairer to not interfere with the test
				// as it can drain from the queue in the background
				config.Repairer.Interval = -1
			},
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var sleepTime time.Duration
		// note: jobq can not run this test normally, because it requires high
		// resolution timestamps. We can make it work if we sleep for at least
		// 1s in the middle of the test, but that's slow. We will skip the jobq
		// test if short tests are requested.
		if _, ok := planet.Satellites[0].Repair.Queue.(*jobq.RepairJobQueue); ok {
			if testing.Short() {
				t.Skip("skipping jobq test in short mode")
			} else {
				sleepTime = time.Second
			}
		}

		rangedLoopService := planet.Satellites[0].RangedLoop.RangedLoop.Service
		repairQueue := planet.Satellites[0].Repair.Queue
		observer := planet.Satellites[0].RangedLoop.Repair.Observer
		planet.Satellites[0].Repair.Repairer.Loop.Pause()

		rs := storj.RedundancyScheme{
			RequiredShares: 2,
			RepairShares:   3,
			OptimalShares:  4,
			TotalShares:    5,
			ShareSize:      256,
		}

		projectID := planet.Uplinks[0].Projects[0].ID
		err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test-bucket")
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

		require.NoError(t, observer.RefreshReliabilityCache(ctx))
		require.NoError(t, planet.Satellites[0].RangedLoop.Overlay.Service.DownloadSelectionCache.Refresh(ctx))

		// check that repair queue is empty to avoid false positive
		count, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		_, err = rangedLoopService.RunOnce(ctx)
		require.NoError(t, err)

		// check that the pointers were put into the repair queue
		// and not cleaned up at the end of the checker iteration
		count, err = repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, healthyCount+unhealthyCount, count)

		// unsuspend nodes to make the previously healthy pointers healthy again
		for i := rs.RequiredShares; i < rs.OptimalShares; i++ {
			require.NoError(t, planet.Satellites[0].Overlay.DB.TestUnsuspendNodeUnknownAudit(ctx, planet.StorageNodes[i].ID()))
		}

		require.NoError(t, observer.RefreshReliabilityCache(ctx))
		require.NoError(t, planet.Satellites[0].RangedLoop.Overlay.Service.DownloadSelectionCache.Refresh(ctx))

		// since this test relies on timestamps below this point being
		// observably higher than timestamps before this point, we need to
		// sleep here, up to the duration of the timestamp resolution.
		time.Sleep(sleepTime)

		// The checker will not insert/update the now healthy segments causing
		// them to be removed from the queue at the end of the checker iteration
		_, err = rangedLoopService.RunOnce(ctx)
		require.NoError(t, err)

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

func TestRepairObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable ranged loop interval execution
				config.RangedLoop.Interval = -1
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4

				// disable repairer to not interfere with the test
				// as it can drain from the queue in the background
				config.Repairer.Interval = -1
			},
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		rs := storj.RedundancyScheme{
			RequiredShares: 2,
			RepairShares:   3,
			OptimalShares:  4,
			TotalShares:    5,
			ShareSize:      256,
		}

		err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "test-bucket")
		require.NoError(t, err)

		expectedLocation := metabase.SegmentLocation{
			ProjectID:  planet.Uplinks[0].Projects[0].ID,
			BucketName: "test-bucket",
		}

		// add some valid segment
		for x := range 20 {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("a-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		var injuredSegmentStreamIDs []uuid.UUID

		// add segment that needs repair
		for x := range 5 {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("b-%d", x))
			injuredSegmentStreamID := insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), nil)
			injuredSegmentStreamIDs = append(injuredSegmentStreamIDs, injuredSegmentStreamID)
		}

		// add pointer that is unhealthy, but is expired
		expectedLocation.ObjectKey = metabase.ObjectKey("d-1")
		expiresAt := time.Now().Add(-time.Hour)
		insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), &expiresAt)

		// add some valid segments
		for x := range 20 {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("c-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), nil)
		}

		compare := func(insertedSegmentsIDs []uuid.UUID, fromRepairQueue []queue.InjuredSegment) bool {
			var repairQueueIDs []uuid.UUID
			for _, v := range fromRepairQueue {
				repairQueueIDs = append(repairQueueIDs, v.StreamID)
			}

			sort.Slice(insertedSegmentsIDs, func(i, j int) bool {
				return insertedSegmentsIDs[i].Less(insertedSegmentsIDs[j])
			})
			sort.Slice(repairQueueIDs, func(i, j int) bool {
				return repairQueueIDs[i].Less(repairQueueIDs[j])
			})

			return reflect.DeepEqual(insertedSegmentsIDs, repairQueueIDs)
		}

		type TestCase struct {
			BatchSize   int
			Parallelism int
		}

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		injuredSegments, err := planet.Satellites[0].Repair.Queue.SelectN(ctx, 10)
		require.NoError(t, err)
		require.Len(t, injuredSegments, 5)
		require.True(t, compare(injuredSegmentStreamIDs, injuredSegments))

		_, err = planet.Satellites[0].Repair.Queue.Clean(ctx, time.Now())
		require.NoError(t, err)

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
			observer := planet.Satellites[0].RangedLoop.Repair.Observer
			config := planet.Satellites[0].Config
			service := rangedloop.NewService(planet.Log(), rangedloop.Config{
				Parallelism: tc.Parallelism,
				BatchSize:   tc.BatchSize,
			}, rangedloop.NewMetabaseRangeSplitter(planet.Log(), planet.Satellites[0].Metabase.DB, config.RangedLoop), []rangedloop.Observer{observer})

			_, err = service.RunOnce(ctx)
			require.NoError(t, err)

			injuredSegments, err = planet.Satellites[0].Repair.Queue.SelectN(ctx, 10)
			require.NoError(t, err)
			require.Len(t, injuredSegments, 5)
			require.True(t, compare(injuredSegmentStreamIDs, injuredSegments))

			_, err = planet.Satellites[0].Repair.Queue.Clean(ctx, time.Now())
			require.NoError(t, err)
		}
	})
}

func createPieces(planet *testplanet.Planet, rs storj.RedundancyScheme) metabase.Pieces {
	pieces := make(metabase.Pieces, rs.OptimalShares)
	for i := range pieces {
		pieces[i] = metabase.Piece{
			Number:      uint16(i),
			StorageNode: planet.StorageNodes[i].Identity.ID,
		}
	}
	return pieces
}

func createLostPieces(planet *testplanet.Planet, rs storj.RedundancyScheme) metabase.Pieces {
	pieces := make(metabase.Pieces, rs.OptimalShares)
	for i := range pieces[:rs.RequiredShares] {
		pieces[i] = metabase.Piece{
			Number:      uint16(i),
			StorageNode: planet.StorageNodes[i].Identity.ID,
		}
	}
	for i := rs.RequiredShares; i < rs.OptimalShares; i++ {
		pieces[i] = metabase.Piece{
			Number:      uint16(i),
			StorageNode: storj.NodeID{byte(0xFF)},
		}
	}
	return pieces
}

func insertSegment(ctx context.Context, t *testing.T, planet *testplanet.Planet, rs storj.RedundancyScheme, location metabase.SegmentLocation, pieces metabase.Pieces, expiresAt *time.Time) uuid.UUID {
	metabaseDB := planet.Satellites[0].Metabase.DB

	obj := metabase.ObjectStream{
		ProjectID:  location.ProjectID,
		BucketName: location.BucketName,
		ObjectKey:  location.ObjectKey,
		Version:    1,
		StreamID:   testrand.UUID(),
	}

	_, err := metabaseDB.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption: storj.EncryptionParameters{
			CipherSuite: storj.EncAESGCM,
			BlockSize:   256,
		},
		ExpiresAt: expiresAt,
	})
	require.NoError(t, err)

	rootPieceID := testrand.PieceID()
	err = metabaseDB.BeginSegment(ctx, metabase.BeginSegment{
		ObjectStream: obj,
		RootPieceID:  rootPieceID,
		Pieces:       pieces,
	})
	require.NoError(t, err)

	err = metabaseDB.CommitSegment(ctx, metabase.CommitSegment{
		ObjectStream:      obj,
		RootPieceID:       rootPieceID,
		Pieces:            pieces,
		EncryptedKey:      testrand.Bytes(256),
		EncryptedKeyNonce: testrand.Bytes(256),
		PlainSize:         1,
		EncryptedSize:     1,
		Redundancy:        rs,
		ExpiresAt:         expiresAt,
	})
	require.NoError(t, err)

	_, err = metabaseDB.CommitObject(ctx, metabase.CommitObject{
		ObjectStream: obj,
	})
	require.NoError(t, err)

	return obj.StreamID
}

func BenchmarkRemoteSegment(b *testing.B) {
	testplanet.Bench(b, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(b *testing.B, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i := 0; i < 10; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object"+strconv.Itoa(i), testrand.Bytes(10*memory.KiB))
			require.NoError(b, err)
		}

		reliabilityCache := checker.NewReliabilityCache(
			planet.Satellites[0].Auditor.Overlay, planet.Satellites[0].Config.Checker.ReliabilityCacheStaleness,
			planet.Satellites[0].Config.Checker.OnlineWindow,
		)
		health := checker.NewProbabilityHealth(planet.Satellites[0].Config.Checker.NodeFailureRate, reliabilityCache)
		observer := checker.NewObserver(zap.NewNop(), planet.Satellites[0].Repair.Queue,
			planet.Satellites[0].Auditor.Overlay, nodeselection.TestPlacementDefinitionsWithFraction(0.05), planet.Satellites[0].Config.Checker, health)
		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(b, err)

		loopSegments := []rangedloop.Segment{}

		for _, segment := range segments {
			loopSegments = append(loopSegments, rangedloop.Segment{
				StreamID:   segment.StreamID,
				Position:   segment.Position,
				CreatedAt:  segment.CreatedAt,
				ExpiresAt:  segment.ExpiresAt,
				Redundancy: segment.Redundancy,
				Pieces:     segment.Pieces,
			})
		}

		fork, err := observer.Fork(ctx)
		require.NoError(b, err)

		b.Run("healthy segment", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = fork.Process(ctx, loopSegments)
			}
		})
	})

}

func TestObserver_PlacementCheck(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(1, 2, 4, 4),
				func(log *zap.Logger, index int, config *satellite.Config) {
					// disable ranged loop interval execution
					config.RangedLoop.Interval = -1
				},
			),
		},
		ExerciseJobq: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Repair.Repairer.Loop.Pause()

		repairQueue := planet.Satellites[0].Repair.Queue

		require.NoError(t, planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], "testbucket"))

		_, err := planet.Satellites[0].API.Buckets.Service.UpdateBucket(ctx, buckets.Bucket{
			ProjectID: planet.Uplinks[0].Projects[0].ID,
			Name:      "testbucket",
			Placement: storj.PlacementConstraint(1),
		})
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			node.Contact.Chore.Pause(ctx)
			require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), "PL"))
		}

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		type testCase struct {
			piecesOutOfPlacement int
			// how many from out of placement pieces should be also offline
			piecesOutOfPlacementOffline int
		}

		for i, tc := range []testCase{
			// all pieces/nodes are out of placement
			{piecesOutOfPlacement: 4},
			// // few pieces/nodes are out of placement
			{piecesOutOfPlacement: 2},
			// all pieces/nodes are out of placement + 1 from it is offline
			{piecesOutOfPlacement: 4, piecesOutOfPlacementOffline: 1},
			// // few pieces/nodes are out of placement + 1 from it is offline
			{piecesOutOfPlacement: 2, piecesOutOfPlacementOffline: 1},
			// // single piece/node is out of placement and it is offline
			{piecesOutOfPlacement: 1, piecesOutOfPlacementOffline: 1},
		} {
			t.Run("#"+strconv.Itoa(i), func(t *testing.T) {
				for _, node := range planet.StorageNodes {
					require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, node.ID(), "PL"))
				}

				require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))

				segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
				require.NoError(t, err)
				require.Len(t, segments, 1)
				require.Len(t, segments[0].Pieces, 4)

				for index, piece := range segments[0].Pieces {
					if index < tc.piecesOutOfPlacement {
						require.NoError(t, planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, piece.StorageNode, "US"))
					}

					// make node offline if needed
					require.NoError(t, updateNodeStatus(ctx, planet.Satellites[0], planet.FindNode(piece.StorageNode), index < tc.piecesOutOfPlacementOffline))
				}

				// confirm that some pieces are out of placement
				filter, _ := nodeselection.TestPlacementDefinitionsWithFraction(planet.Satellites[0].Config.Overlay.Node.NewNodeFraction).CreateFilters(segments[0].Placement)
				ok, err := allPiecesInPlacement(ctx, planet.Satellites[0].Overlay.Service, segments[0].Pieces, filter)
				require.NoError(t, err)
				require.False(t, ok)

				require.NoError(t, planet.Satellites[0].Repairer.Overlay.DownloadSelectionCache.Refresh(ctx))

				_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
				require.NoError(t, err)

				injuredSegments, err := repairQueue.Select(ctx, 1, nil, nil)
				require.NoError(t, err)
				err = repairQueue.Release(ctx, injuredSegments[0], true)
				require.NoError(t, err)

				require.Equal(t, segments[0].StreamID, injuredSegments[0].StreamID)
				require.Equal(t, segments[0].Placement, injuredSegments[0].Placement)
				require.Equal(t, storj.PlacementConstraint(1), injuredSegments[0].Placement)

				count, err := repairQueue.Count(ctx)
				require.Zero(t, err)
				require.Zero(t, count)
			})
		}
	})
}

func allPiecesInPlacement(ctx context.Context, overlay *overlay.Service, pieces metabase.Pieces, filter nodeselection.NodeFilter) (bool, error) {
	for _, piece := range pieces {
		nodeDossier, err := overlay.Get(ctx, piece.StorageNode)
		if err != nil {
			return false, err
		}
		if !filter.Match(&nodeselection.SelectedNode{
			CountryCode: nodeDossier.CountryCode,
		}) {
			return false, nil
		}
	}
	return true, nil
}

func updateNodeStatus(ctx context.Context, satellite *testplanet.Satellite, node *testplanet.StorageNode, offline bool) error {
	timestamp := time.Now()
	if offline {
		timestamp = time.Now().Add(-4 * time.Hour)
	}

	return satellite.DB.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
		NodeID:  node.ID(),
		Address: &pb.NodeAddress{Address: node.Addr()},
		IsUp:    true,
		Version: &pb.NodeVersion{
			Version:    "v0.0.0",
			CommitHash: "",
			Timestamp:  time.Time{},
			Release:    false,
		},
	}, timestamp, satellite.Config.Overlay.Node)
}
