// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
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
			insertPointer(ctx, t, planet, rs, pointerPathPrefix+fmt.Sprintf("a-%d", x), false)
		}

		// add pointer that needs repair
		insertPointer(ctx, t, planet, rs, pointerPathPrefix+"b", true)

		// add some valid pointers
		for x := 0; x < 10; x++ {
			insertPointer(ctx, t, planet, rs, pointerPathPrefix+fmt.Sprintf("c-%d", x), false)
		}

		checker.Loop.TriggerWait()

		//check if the expected segments were added to the queue
		injuredSegment, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)

		require.Equal(t, []byte(pointerPathPrefix+"b"), injuredSegment.Path)
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

		projectID := testrand.UUID()
		pointerPath := storj.JoinPaths(projectID.String(), "l", "bucket", "piece")
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

		// put test pointer to db
		metainfo := planet.Satellites[0].Metainfo.Service
		err := metainfo.Put(ctx, pointerPath, pointer)
		require.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		// check if nothing was added to repair queue
		repairQueue := planet.Satellites[0].DB.RepairQueue()
		_, err = repairQueue.Select(ctx)
		require.True(t, storage.ErrEmptyQueue.Has(err))

		//check if the expected segments were added to the irreparable DB
		irreparable := planet.Satellites[0].DB.Irreparable()
		remoteSegmentInfo, err := irreparable.Get(ctx, []byte(pointerPath))
		require.NoError(t, err)

		require.Equal(t, len(expectedLostPieces), int(remoteSegmentInfo.LostPieces))
		require.Equal(t, 1, int(remoteSegmentInfo.RepairAttemptCount))
		firstRepair := remoteSegmentInfo.LastRepairAttempt

		// check irreparable once again but wait a second
		time.Sleep(1 * time.Second)
		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		remoteSegmentInfo, err = irreparable.Get(ctx, []byte(pointerPath))
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
		err = metainfo.UnsynchronizedDelete(ctx, pointerPath)
		require.NoError(t, err)
		err = metainfo.Put(ctx, pointerPath, pointer)
		require.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		_, err = irreparable.Get(ctx, []byte(pointerPath))
		require.Error(t, err)
	})
}

func insertPointer(ctx context.Context, t *testing.T, planet *testplanet.Planet, rs *pb.RedundancyScheme, pointerPath string, createLost bool) {
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

	// put test pointer to db
	pointerdb := planet.Satellites[0].Metainfo.Service
	err := pointerdb.Put(ctx, pointerPath, pointer)
	require.NoError(t, err)
}
