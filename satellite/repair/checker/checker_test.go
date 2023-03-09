// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"context"
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
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/repair/checker"
)

func TestIdentifyInjuredSegments(t *testing.T) {
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

func TestIdentifyIrreparableSegments(t *testing.T) {
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

func TestCleanRepairQueue(t *testing.T) {
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

func TestIgnoringCopiedSegments(t *testing.T) {
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
