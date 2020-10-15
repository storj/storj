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

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
)

func TestIdentifyInjuredSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		repairQueue := planet.Satellites[0].DB.RepairQueue()

		checker.Loop.Pause()
		planet.Satellites[0].Repair.Repairer.Loop.Pause()

		rs := &pb.RedundancyScheme{
			MinReq:           int32(2),
			RepairThreshold:  int32(3),
			SuccessThreshold: int32(4),
			Total:            int32(5),
			ErasureShareSize: int32(256),
		}

		projectID := testrand.UUID()
		pointerPathPrefix := storj.JoinPaths(projectID.String(), "l", "bucket") + "/"

		// add some valid pointers
		for x := 0; x < 10; x++ {
			insertPointer(ctx, t, planet, rs, pointerPathPrefix+fmt.Sprintf("a-%d", x), false, time.Time{})
		}

		// add pointer that needs repair
		insertPointer(ctx, t, planet, rs, pointerPathPrefix+"b-0", true, time.Time{})

		// add pointer that is unhealthy, but is expired
		insertPointer(ctx, t, planet, rs, pointerPathPrefix+"b-1", true, time.Now().Add(-time.Hour))

		// add some valid pointers
		for x := 0; x < 10; x++ {
			insertPointer(ctx, t, planet, rs, pointerPathPrefix+fmt.Sprintf("c-%d", x), false, time.Time{})
		}

		checker.Loop.TriggerWait()

		// check that the unhealthy, non-expired segment was added to the queue
		// and that the expired segment was ignored
		injuredSegment, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)

		require.Equal(t, []byte(pointerPathPrefix+"b-0"), injuredSegment.Path)
		require.Equal(t, int(rs.SuccessThreshold-rs.MinReq), len(injuredSegment.LostPieces))
		for _, lostPiece := range injuredSegment.LostPieces {
			require.True(t, rs.MinReq <= lostPiece && lostPiece < rs.SuccessThreshold, fmt.Sprintf("%v", lostPiece))
		}

		_, err = repairQueue.Select(ctx)
		require.Error(t, err)
	})
}

func TestIdentifyIrreparableSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		checker.Loop.Stop()
		checker.IrreparableLoop.Stop()

		const numberOfNodes = 10
		pieces := make([]*pb.RemotePiece, 0, numberOfNodes)
		// use online nodes
		for i, storagenode := range planet.StorageNodes {
			pieces = append(pieces, &pb.RemotePiece{
				PieceNum: int32(i),
				NodeId:   storagenode.ID(),
			})
		}

		// simulate offline nodes
		expectedLostPieces := make(map[int32]bool)
		for i := len(pieces); i < numberOfNodes; i++ {
			pieces = append(pieces, &pb.RemotePiece{
				PieceNum: int32(i),
				NodeId:   storj.NodeID{byte(i)},
			})
			expectedLostPieces[int32(i)] = true
		}

		pieceID := testrand.PieceID()

		// when number of healthy piece is less than minimum required number of piece in redundancy,
		// the piece is considered irreparable and will be put into irreparable DB
		pointer := &pb.Pointer{
			Type:         pb.Pointer_REMOTE,
			CreationDate: time.Now(),
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					ErasureShareSize: int32(256),
					MinReq:           int32(4),
					RepairThreshold:  int32(8),
					SuccessThreshold: int32(9),
					Total:            int32(10),
				},
				RootPieceId:  pieceID,
				RemotePieces: pieces,
			},
		}

		projectID := testrand.UUID()
		pointerLocation := metabase.SegmentLocation{
			ProjectID:  projectID,
			BucketName: "bucket",
			Index:      metabase.LastSegmentIndex,
			ObjectKey:  "piece",
		}

		pointerKey := pointerLocation.Encode()
		pointerLocation.ObjectKey += "-expired"
		pointerExpiredKey := pointerLocation.Encode()
		// put test pointer to db
		metainfo := planet.Satellites[0].Metainfo.Service
		err := metainfo.Put(ctx, pointerKey, pointer)
		require.NoError(t, err)
		// modify pointer to make it expired and put to db
		pointer.ExpirationDate = time.Now().Add(-time.Hour)
		err = metainfo.Put(ctx, pointerExpiredKey, pointer)
		require.NoError(t, err)

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

		// make the pointer repairable
		pointer = &pb.Pointer{
			Type:         pb.Pointer_REMOTE,
			CreationDate: time.Now(),
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					ErasureShareSize: int32(256),
					MinReq:           int32(2),
					RepairThreshold:  int32(8),
					SuccessThreshold: int32(9),
					Total:            int32(10),
				},
				RootPieceId:  pieceID,
				RemotePieces: pieces,
			},
		}
		// update test pointer in db
		err = metainfo.UnsynchronizedDelete(ctx, pointerKey)
		require.NoError(t, err)
		err = metainfo.Put(ctx, pointerKey, pointer)
		require.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		_, err = irreparable.Get(ctx, pointerKey)
		require.Error(t, err)
	})
}

func TestCleanRepairQueue(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		repairQueue := planet.Satellites[0].DB.RepairQueue()

		checker.Loop.Pause()
		planet.Satellites[0].Repair.Repairer.Loop.Pause()

		rs := &pb.RedundancyScheme{
			MinReq:           int32(2),
			RepairThreshold:  int32(3),
			SuccessThreshold: int32(4),
			Total:            int32(4),
			ErasureShareSize: int32(256),
		}

		projectID := testrand.UUID()
		pointerPathPrefix := storj.JoinPaths(projectID.String(), "l", "bucket") + "/"

		healthyCount := 5
		for i := 0; i < healthyCount; i++ {
			insertPointer(ctx, t, planet, rs, pointerPathPrefix+fmt.Sprintf("healthy-%d", i), false, time.Time{})
		}
		unhealthyCount := 5
		for i := 0; i < unhealthyCount; i++ {
			insertPointer(ctx, t, planet, rs, pointerPathPrefix+fmt.Sprintf("unhealthy-%d", i), true, time.Time{})
		}

		// suspend enough nodes to make healthy pointers unhealthy
		for i := rs.MinReq; i < rs.SuccessThreshold; i++ {
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
		for i := rs.MinReq; i < rs.SuccessThreshold; i++ {
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

func insertPointer(ctx context.Context, t *testing.T, planet *testplanet.Planet, rs *pb.RedundancyScheme, pointerPath string, createLost bool, expire time.Time) {
	pieces := make([]*pb.RemotePiece, rs.SuccessThreshold)
	if !createLost {
		for i := range pieces {
			pieces[i] = &pb.RemotePiece{
				PieceNum: int32(i),
				NodeId:   planet.StorageNodes[i].Identity.ID,
			}
		}
	} else {
		for i := range pieces[:rs.MinReq] {
			pieces[i] = &pb.RemotePiece{
				PieceNum: int32(i),
				NodeId:   planet.StorageNodes[i].Identity.ID,
			}
		}
		for i := rs.MinReq; i < rs.SuccessThreshold; i++ {
			pieces[i] = &pb.RemotePiece{
				PieceNum: i,
				NodeId:   storj.NodeID{byte(0xFF)},
			}
		}
	}

	pointer := &pb.Pointer{
		Type:         pb.Pointer_REMOTE,
		CreationDate: time.Now(),
		Remote: &pb.RemoteSegment{
			Redundancy:   rs,
			RootPieceId:  testrand.PieceID(),
			RemotePieces: pieces,
		},
	}
	if !expire.IsZero() {
		pointer.ExpirationDate = expire
	}

	// put test pointer to db
	pointerdb := planet.Satellites[0].Metainfo.Service
	err := pointerdb.Put(ctx, metabase.SegmentKey(pointerPath), pointer)
	require.NoError(t, err)
}
