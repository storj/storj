// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/uplink"
)

const numObjects = 6

func TestSuccess(t *testing.T) {
	testTransfers(t, numObjects, func(ctx *testcontext.Context, satellite *testplanet.SatelliteSystem, processClient pb.SatelliteGracefulExit_ProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		var pieceID storj.PieceID
		failedCount := 0
		for {
			response, err := processClient.Recv()
			if err == io.EOF {
				// Done
				break
			}
			require.NoError(t, err)

			switch m := response.GetMessage().(type) {
			case *pb.SatelliteMessage_TransferPiece:
				require.NotNil(t, m)

				// pick the first one to fail
				if pieceID.IsZero() {
					pieceID = m.TransferPiece.PieceId
				}

				if failedCount > 0 || pieceID != m.TransferPiece.PieceId {
					success := &pb.StorageNodeMessage{
						Message: &pb.StorageNodeMessage_Succeeded{
							Succeeded: &pb.TransferSucceeded{
								PieceId:           m.TransferPiece.PieceId,
								OriginalPieceHash: &pb.PieceHash{PieceId: m.TransferPiece.PieceId},
								AddressedOrderLimit: &pb.AddressedOrderLimit{
									Limit: &pb.OrderLimit{
										PieceId: m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
									},
								},
							},
						},
					}
					err = processClient.Send(success)
					require.NoError(t, err)
				} else {
					failedCount++
					failed := &pb.StorageNodeMessage{
						Message: &pb.StorageNodeMessage_Failed{
							Failed: &pb.TransferFailed{
								PieceId: m.TransferPiece.PieceId,
								Error:   pb.TransferFailed_UNKNOWN,
							},
						},
					}
					err = processClient.Send(failed)
					require.NoError(t, err)
				}
			case *pb.SatelliteMessage_ExitCompleted:
				// TODO test completed signature stuff
				break
			default:
				t.FailNow()
			}
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.EqualValues(t, numPieces, progress.PiecesTransferred)
		// even though we failed 1, it eventually succeeded, so the count should be 0
		require.EqualValues(t, 0, progress.PiecesFailed)
	})
}

func TestFailure(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, satellite *testplanet.SatelliteSystem, processClient pb.SatelliteGracefulExit_ProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		for {
			response, err := processClient.Recv()
			if err == io.EOF {
				// Done
				break
			}
			require.NoError(t, err)

			switch m := response.GetMessage().(type) {
			case *pb.SatelliteMessage_TransferPiece:
				require.NotNil(t, m)

				failed := &pb.StorageNodeMessage{
					Message: &pb.StorageNodeMessage_Failed{
						Failed: &pb.TransferFailed{
							PieceId: m.TransferPiece.PieceId,
							Error:   pb.TransferFailed_UNKNOWN,
						},
					},
				}
				err = processClient.Send(failed)
				require.NoError(t, err)
			case *pb.SatelliteMessage_ExitCompleted:
				// TODO test completed signature stuff
				break
			default:
				t.FailNow()
			}
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.Equal(t, int64(0), progress.PiecesTransferred)
		require.Equal(t, int64(1), progress.PiecesFailed)
	})
}

func testTransfers(t *testing.T, objects int, verifier func(ctx *testcontext.Context, satellite *testplanet.SatelliteSystem, processClient pb.SatelliteGracefulExit_ProcessClient, exitingNode *storagenode.Peer, numPieces int)) {
	successThreshold := 8
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  6,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
		}

		for i := 0; i < objects; i++ {
			err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path"+strconv.Itoa(i), testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)
		}
		// check that there are no exiting nodes.
		exitingNodeIDs, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodeIDs, 0)

		exitingNode, err := findNodeToExit(ctx, planet, objects)
		require.NoError(t, err)

		// connect to satellite so we initiate the exit.
		conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
		require.NoError(t, err)
		defer func() {
			err = errs.Combine(err, conn.Close())
		}()

		client := conn.SatelliteGracefulExitClient()

		c, err := client.Process(ctx, grpc.EmptyCallOption{})
		require.NoError(t, err)

		response, err := c.Recv()
		require.NoError(t, err)

		// should get a NotReady since the metainfo loop would not be finished at this point.
		switch response.GetMessage().(type) {
		case *pb.SatelliteMessage_NotReady:
			// now check that the exiting node is initiated.
			exitingNodeIDs, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
			require.NoError(t, err)
			require.Len(t, exitingNodeIDs, 1)

			require.Equal(t, exitingNode.ID(), exitingNodeIDs[0])
		default:
			t.FailNow()
		}
		// close the old client
		require.NoError(t, c.CloseSend())

		// trigger the metainfo loop chore so we can get some pieces to transfer
		satellite.GracefulExit.Chore.Loop.TriggerWait()

		// make sure all the pieces are in the transfer queue
		incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), objects, 0)
		require.NoError(t, err)

		// connect to satellite again to start receiving transfers
		c, err = client.Process(ctx, grpc.EmptyCallOption{})
		require.NoError(t, err)
		defer func() {
			err = errs.Combine(err, c.CloseSend())
		}()

		verifier(ctx, satellite, c, exitingNode, len(incompleteTransfers))
	})
}

func findNodeToExit(ctx context.Context, planet *testplanet.Planet, objects int) (*storagenode.Peer, error) {
	satellite := planet.Satellites[0]
	keys, err := satellite.Metainfo.Database.List(ctx, nil, objects)
	if err != nil {
		return nil, err
	}

	pieceCountMap := make(map[storj.NodeID]int, len(planet.StorageNodes))
	for _, sn := range planet.StorageNodes {
		pieceCountMap[sn.ID()] = 0
	}

	for _, key := range keys {
		pointer, err := satellite.Metainfo.Service.Get(ctx, string(key))
		if err != nil {
			return nil, err
		}
		pieces := pointer.GetRemote().GetRemotePieces()
		for _, piece := range pieces {
			pieceCountMap[piece.NodeId]++
		}
	}

	var exitingNodeID storj.NodeID
	maxCount := 0
	for k, v := range pieceCountMap {
		if exitingNodeID.IsZero() {
			exitingNodeID = k
			maxCount = v
			continue
		}
		if v > maxCount {
			exitingNodeID = k
			maxCount = v
		}
	}

	for _, sn := range planet.StorageNodes {
		if sn.ID() == exitingNodeID {
			return sn, nil
		}
	}

	return nil, nil
}
