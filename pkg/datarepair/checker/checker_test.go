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
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/datarepair/checker"
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
		for x := 0; x < 1000; x++ {
			makePointer(t, planet, fmt.Sprintf("a-%d", x), false)
		}
		//create piece that needs repair
		makePointer(t, planet, fmt.Sprintf("b"), true)
		//add more noise to metainfo after bad record
		for x := 0; x < 1000; x++ {
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
		pointer := &pb.Pointer{
			CreationDate: time.Now(),
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					MinReq:           int32(3),
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

		// make the  pointer repairable
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
		// put test pointer to db
		metainfo = planet.Satellites[0].Metainfo.Service
		err = metainfo.Put(ctx, "fake-piece-id", pointer)
		require.NoError(t, err)

		err = checker.IdentifyInjuredSegments(ctx)
		require.NoError(t, err)

		remoteSegmentInfo, err = irreparable.Get(ctx, []byte("fake-piece-id"))
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

func TestCheckerResume(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		repairQueue := &mockRepairQueue{}
		irrepairQueue := planet.Satellites[0].DB.Irreparable()
		config := checker.Config{
			Interval:                  30 * time.Second,
			IrreparableInterval:       15 * time.Second,
			ReliabilityCacheStaleness: 5 * time.Minute,
		}
		c := checker.NewChecker(planet.Satellites[0].Metainfo.Service, repairQueue, planet.Satellites[0].Overlay.Service, irrepairQueue, 0, nil, config)

		// create pointer that needs repair
		makePointer(t, planet, "a", true)
		// create pointer that will cause an error
		makePointer(t, planet, "b", true)
		// create pointer that needs repair
		makePointer(t, planet, "c", true)
		// create pointer that will cause an error
		makePointer(t, planet, "d", true)

		err := c.IdentifyInjuredSegments(ctx)
		require.Error(t, err)

		// "a" should be the only segment in the repair queue
		injuredSegment, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, injuredSegment.Path, []byte("a"))
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)
		injuredSegment, err = repairQueue.Select(ctx)
		require.Error(t, err)

		err = c.IdentifyInjuredSegments(ctx)
		require.Error(t, err)

		// "c" should be the only segment in the repair queue
		injuredSegment, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, injuredSegment.Path, []byte("c"))
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)
		injuredSegment, err = repairQueue.Select(ctx)
		require.Error(t, err)

		err = c.IdentifyInjuredSegments(ctx)
		require.Error(t, err)

		// "a" should be the only segment in the repair queue
		injuredSegment, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, injuredSegment.Path, []byte("a"))
		err = repairQueue.Delete(ctx, injuredSegment)
		require.NoError(t, err)
		injuredSegment, err = repairQueue.Select(ctx)
		require.Error(t, err)
	})
}

// mock repair queue used for TestCheckerResume
type mockRepairQueue struct {
	injuredSegments []pb.InjuredSegment
}

func (mockRepairQueue *mockRepairQueue) Insert(ctx context.Context, s *pb.InjuredSegment) error {
	if bytes.Equal(s.Path, []byte("b")) || bytes.Equal(s.Path, []byte("d")) {
		return errs.New("mock Insert error")
	}
	mockRepairQueue.injuredSegments = append(mockRepairQueue.injuredSegments, *s)
	return nil
}

func (mockRepairQueue *mockRepairQueue) Select(ctx context.Context) (*pb.InjuredSegment, error) {
	if len(mockRepairQueue.injuredSegments) == 0 {
		return &pb.InjuredSegment{}, errs.New("mock Select error")
	}
	s := mockRepairQueue.injuredSegments[0]
	return &s, nil
}

func (mockRepairQueue *mockRepairQueue) Delete(ctx context.Context, s *pb.InjuredSegment) error {
	var toDelete int
	found := false
	for i, seg := range mockRepairQueue.injuredSegments {
		if bytes.Equal(seg.Path, s.Path) {
			toDelete = i
			found = true
			break
		}
	}
	if !found {
		return errs.New("mock Delete error")
	}

	mockRepairQueue.injuredSegments = append(mockRepairQueue.injuredSegments[:toDelete], mockRepairQueue.injuredSegments[toDelete+1:]...)
	return nil
}

func (mockRepairQueue *mockRepairQueue) SelectN(ctx context.Context, limit int) ([]pb.InjuredSegment, error) {
	return []pb.InjuredSegment{}, errs.New("mock SelectN error")
}
