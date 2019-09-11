// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestIdentifyInjuredSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		checker.Loop.Stop()

		//add noise to metainfo before bad record
		for x := 0; x < 10; x++ {
			makePointer(t, planet, fmt.Sprintf("a-%d", x), false)
		}
		//create piece that needs repair
		makePointer(t, planet, fmt.Sprintf("b"), true)
		//add more noise to metainfo after bad record
		for x := 0; x < 10; x++ {
			makePointer(t, planet, fmt.Sprintf("c-%d", x), false)
		}
		err := checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		//check if the expected segments were added to the queue
		repairQueue := planet.Satellites[0].DB.RepairQueue()
		injuredSegment, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)

		numValidNode := int32(len(planet.StorageNodes))
		require.Equal(t, []byte("b"), injuredSegment.Path)
		require.Equal(t, len(planet.StorageNodes), len(injuredSegment.LostPieces))
		for _, lostPiece := range injuredSegment.LostPieces {
			// makePointer() starts with numValidNode good pieces
			require.True(t, lostPiece >= numValidNode, fmt.Sprintf("%d >= %d \n", lostPiece, numValidNode))
			// makePointer() than has numValidNode bad pieces
			require.True(t, lostPiece < numValidNode*2, fmt.Sprintf("%d < %d \n", lostPiece, numValidNode*2))
		}
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

		// when number of healthy piece is less than minimum required number of piece in redundancy,
		// the piece is considered irreparable and will be put into irreparable DB
		pointer := &pb.Pointer{
			CreationDate: time.Now(),
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					MinReq:           int32(4),
					RepairThreshold:  int32(8),
					SuccessThreshold: int32(9),
					Total:            int32(10),
				},
				RootPieceId:  teststorj.PieceIDFromString("fake-piece-id"),
				RemotePieces: pieces,
			},
		}

		// put test pointer to db
		metainfo := planet.Satellites[0].Metainfo.Service
		err := metainfo.Put(ctx, "fake-piece-id", pointer)
		require.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		// check if nothing was added to repair queue
		repairQueue := planet.Satellites[0].DB.RepairQueue()
		_, err = repairQueue.Select(ctx)
		require.True(t, storage.ErrEmptyQueue.Has(err))

		//check if the expected segments were added to the irreparable DB
		irreparable := planet.Satellites[0].DB.Irreparable()
		remoteSegmentInfo, err := irreparable.Get(ctx, []byte("fake-piece-id"))
		require.NoError(t, err)

		require.Equal(t, len(expectedLostPieces), int(remoteSegmentInfo.LostPieces))
		require.Equal(t, 1, int(remoteSegmentInfo.RepairAttemptCount))
		firstRepair := remoteSegmentInfo.LastRepairAttempt

		// check irreparable once again but wait a second
		time.Sleep(1 * time.Second)
		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		remoteSegmentInfo, err = irreparable.Get(ctx, []byte("fake-piece-id"))
		require.NoError(t, err)

		require.Equal(t, len(expectedLostPieces), int(remoteSegmentInfo.LostPieces))
		// check if repair attempt count was incremented
		require.Equal(t, 2, int(remoteSegmentInfo.RepairAttemptCount))
		require.True(t, firstRepair < remoteSegmentInfo.LastRepairAttempt)

		// make the pointer repairable
		pointer = &pb.Pointer{
			CreationDate: time.Now(),
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					MinReq:           int32(2),
					RepairThreshold:  int32(8),
					SuccessThreshold: int32(9),
					Total:            int32(10),
				},
				RootPieceId:  teststorj.PieceIDFromString("fake-piece-id"),
				RemotePieces: pieces,
			},
		}
		// update test pointer in db
		err = metainfo.Delete(ctx, "fake-piece-id")
		require.NoError(t, err)
		err = metainfo.Put(ctx, "fake-piece-id", pointer)
		require.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		_, err = irreparable.Get(ctx, []byte("fake-piece-id"))
		require.Error(t, err)
	})
}

func makePointer(t *testing.T, planet *testplanet.Planet, pieceID string, createLost bool) {
	ctx := context.TODO()
	numOfStorageNodes := len(planet.StorageNodes)
	pieces := make([]*pb.RemotePiece, 0, numOfStorageNodes)
	// use online nodes
	for i := 0; i < numOfStorageNodes; i++ {
		pieces = append(pieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   planet.StorageNodes[i].Identity.ID,
		})
	}
	// simulate offline nodes equal to the number of online nodes
	if createLost {
		for i := 0; i < numOfStorageNodes; i++ {
			pieces = append(pieces, &pb.RemotePiece{
				PieceNum: int32(numOfStorageNodes + i),
				NodeId:   storj.NodeID{byte(i)},
			})
		}
	}
	minReq, repairThreshold := numOfStorageNodes-1, numOfStorageNodes-1
	if createLost {
		minReq, repairThreshold = numOfStorageNodes-1, numOfStorageNodes+1
	}
	pointer := &pb.Pointer{
		CreationDate: time.Now(),
		Remote: &pb.RemoteSegment{
			Redundancy: &pb.RedundancyScheme{
				MinReq:           int32(minReq),
				RepairThreshold:  int32(repairThreshold),
				SuccessThreshold: int32(repairThreshold) + 1,
				Total:            int32(repairThreshold) + 2,
			},
			RootPieceId:  teststorj.PieceIDFromString(pieceID),
			RemotePieces: pieces,
		},
	}
	// put test pointer to db
	pointerdb := planet.Satellites[0].Metainfo.Service
	err := pointerdb.Put(ctx, pieceID, pointer)
	require.NoError(t, err)
}
