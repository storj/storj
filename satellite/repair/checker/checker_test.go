// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
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
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), time.Time{})
		}

		// add pointer that needs repair
		expectedLocation.ObjectKey = metabase.ObjectKey("b-0")
		insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), time.Time{})

		// add pointer that is unhealthy, but is expired
		expectedLocation.ObjectKey = metabase.ObjectKey("b-1")
		insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), time.Now().Add(-time.Hour))

		// add some valid pointers
		for x := 0; x < 10; x++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("c-%d", x))
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), time.Time{})
		}

		checker.Loop.TriggerWait()

		// check that the unhealthy, non-expired segment was added to the queue
		// and that the expired segment was ignored
		injuredSegment, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)

		expectedLocation.ObjectKey = "b-0"
		require.Equal(t, string(expectedLocation.Encode()), string(injuredSegment.Path))
		require.Equal(t, int(rs.OptimalShares-rs.RequiredShares), len(injuredSegment.LostPieces))
		for _, lostPiece := range injuredSegment.LostPieces {
			require.True(t, int32(rs.RequiredShares) <= lostPiece && lostPiece < int32(rs.OptimalShares), fmt.Sprintf("%v", lostPiece))
		}

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
		checker.IrreparableLoop.Stop()

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
		// the piece is considered irreparable and will be put into irreparable DB

		expectedLocation.ObjectKey = "piece"
		insertSegment(ctx, t, planet, rs, expectedLocation, pieces, time.Time{})
		pointerKey := expectedLocation.Encode()

		expectedLocation.ObjectKey = "piece-expired"
		insertSegment(ctx, t, planet, rs, expectedLocation, pieces, time.Now().Add(-time.Hour))
		pointerExpiredKey := expectedLocation.Encode()

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		// check if nothing was added to repair queue
		repairQueue := planet.Satellites[0].DB.RepairQueue()
		_, err = repairQueue.Select(ctx)
		require.True(t, storage.ErrEmptyQueue.Has(err))

		// check if the expected segments were added to the irreparable DB
		irreparable := planet.Satellites[0].DB.Irreparable()
		remoteSegmentInfo, err := irreparable.Get(ctx, pointerKey)
		require.NoError(t, err)
		// check that the expired segment was not added to the irreparable DB
		_, err = irreparable.Get(ctx, pointerExpiredKey)
		require.Error(t, err)

		require.Equal(t, len(expectedLostPieces), int(remoteSegmentInfo.LostPieces))
		require.Equal(t, 1, int(remoteSegmentInfo.RepairAttemptCount))
		firstRepair := remoteSegmentInfo.LastRepairAttempt

		// check irreparable once again but wait a second
		time.Sleep(1 * time.Second)
		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		remoteSegmentInfo, err = irreparable.Get(ctx, pointerKey)
		require.NoError(t, err)

		require.Equal(t, len(expectedLostPieces), int(remoteSegmentInfo.LostPieces))
		// check if repair attempt count was incremented
		require.Equal(t, 2, int(remoteSegmentInfo.RepairAttemptCount))
		require.True(t, firstRepair < remoteSegmentInfo.LastRepairAttempt)

		expectedLocation.ObjectKey = "piece"
		_, err = planet.Satellites[0].Metainfo.Metabase.DeleteObjectLatestVersion(ctx, metabase.DeleteObjectLatestVersion{
			ObjectLocation: expectedLocation.Object(),
		})
		require.NoError(t, err)

		rs.RequiredShares = 2
		insertSegment(ctx, t, planet, rs, expectedLocation, pieces, time.Time{})
		pointerKey = expectedLocation.Encode()

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		_, err = irreparable.Get(ctx, pointerKey)
		require.Error(t, err)
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
			insertSegment(ctx, t, planet, rs, expectedLocation, createPieces(planet, rs), time.Time{})
		}
		unhealthyCount := 5
		for i := 0; i < unhealthyCount; i++ {
			expectedLocation.ObjectKey = metabase.ObjectKey(fmt.Sprintf("unhealthy-%d", i))
			insertSegment(ctx, t, planet, rs, expectedLocation, createLostPieces(planet, rs), time.Time{})
		}

		// suspend enough nodes to make healthy pointers unhealthy
		for i := rs.RequiredShares; i < rs.OptimalShares; i++ {
			require.NoError(t, planet.Satellites[0].Overlay.DB.SuspendNodeUnknownAudit(ctx, planet.StorageNodes[i].ID(), time.Now()))
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
			require.NoError(t, planet.Satellites[0].Overlay.DB.UnsuspendNodeUnknownAudit(ctx, planet.StorageNodes[i].ID()))
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

		for _, s := range segs {
			require.True(t, bytes.Contains(s.GetPath(), []byte("unhealthy")))
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

func insertSegment(ctx context.Context, t *testing.T, planet *testplanet.Planet, rs storj.RedundancyScheme, location metabase.SegmentLocation, pieces metabase.Pieces, expire time.Time) {
	var expiresAt *time.Time
	if !expire.IsZero() {
		expiresAt = &expire
	}

	metabaseDB := planet.Satellites[0].Metainfo.Metabase

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
	})
	require.NoError(t, err)

	_, err = metabaseDB.CommitObject(ctx, metabase.CommitObject{
		ObjectStream: obj,
	})
	require.NoError(t, err)
}
