// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testblobs"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/uplink"
)

const numObjects = 6

// exitProcessClient is used so we can pass the graceful exit process clients regardless of implementation.
type exitProcessClient interface {
	Send(*pb.StorageNodeMessage) error
	Recv() (*pb.SatelliteMessage, error)
}

func TestSuccess(t *testing.T) {
	testTransfers(t, numObjects, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		var pieceID storj.PieceID
		failedCount := 0
		deletedCount := 0
		for {
			response, err := processClient.Recv()
			if errs.Is(err, io.EOF) {
				// Done
				break
			}
			require.NoError(t, err)

			switch m := response.GetMessage().(type) {
			case *pb.SatelliteMessage_TransferPiece:
				require.NotNil(t, m)

				// pick the first one to fail
				if pieceID.IsZero() {
					pieceID = m.TransferPiece.OriginalPieceId
				}

				if failedCount > 0 || pieceID != m.TransferPiece.OriginalPieceId {

					pieceReader, err := exitingNode.Storage2.Store.Reader(ctx, satellite.ID(), m.TransferPiece.OriginalPieceId)
					require.NoError(t, err)

					header, err := pieceReader.GetPieceHeader()
					require.NoError(t, err)

					orderLimit := header.OrderLimit
					originalPieceHash := &pb.PieceHash{
						PieceId:   orderLimit.PieceId,
						Hash:      header.GetHash(),
						PieceSize: pieceReader.Size(),
						Timestamp: header.GetCreationTime(),
						Signature: header.GetSignature(),
					}

					newPieceHash := &pb.PieceHash{
						PieceId:   m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
						Hash:      originalPieceHash.Hash,
						PieceSize: originalPieceHash.PieceSize,
						Timestamp: time.Now(),
					}

					receivingNodeID := nodeFullIDs[m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId]
					require.NotNil(t, receivingNodeID)
					signer := signing.SignerFromFullIdentity(receivingNodeID)

					signedNewPieceHash, err := signing.SignPieceHash(ctx, signer, newPieceHash)
					require.NoError(t, err)

					success := &pb.StorageNodeMessage{
						Message: &pb.StorageNodeMessage_Succeeded{
							Succeeded: &pb.TransferSucceeded{
								OriginalPieceId:      m.TransferPiece.OriginalPieceId,
								OriginalPieceHash:    originalPieceHash,
								OriginalOrderLimit:   &orderLimit,
								ReplacementPieceHash: signedNewPieceHash,
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
								OriginalPieceId: m.TransferPiece.OriginalPieceId,
								Error:           pb.TransferFailed_UNKNOWN,
							},
						},
					}
					err = processClient.Send(failed)
					require.NoError(t, err)
				}
			case *pb.SatelliteMessage_DeletePiece:
				deletedCount++
			case *pb.SatelliteMessage_ExitCompleted:
				signee := signing.SigneeFromPeerIdentity(satellite.Identity.PeerIdentity())
				err = signing.VerifyExitCompleted(ctx, signee, m.ExitCompleted)
				require.NoError(t, err)
			default:
				t.FailNow()
			}
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.EqualValues(t, numPieces, progress.PiecesTransferred)
		require.EqualValues(t, numPieces, deletedCount)
		// even though we failed 1, it eventually succeeded, so the count should be 0
		require.EqualValues(t, 0, progress.PiecesFailed)
	})
}

func TestConcurrentConnections(t *testing.T) {
	successThreshold := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		// check that there are no exiting nodes.
		exitingNodeIDs, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodeIDs, 0)

		exitingNode, err := findNodeToExit(ctx, planet, 2)
		require.NoError(t, err)

		var group errgroup.Group
		concurrentCalls := 4
		var wg sync.WaitGroup
		wg.Add(1)
		for i := 0; i < concurrentCalls; i++ {
			group.Go(func() (err error) {
				// connect to satellite so we initiate the exit.
				conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
				require.NoError(t, err)
				defer func() {
					err = errs.Combine(err, conn.Close())
				}()

				client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())

				// wait for "main" call to begin
				wg.Wait()

				c, err := client.Process(ctx)
				require.NoError(t, err)

				_, err = c.Recv()
				require.Error(t, err)
				require.True(t, errs2.IsRPC(err, rpcstatus.Aborted))
				return nil
			})
		}

		// connect to satellite so we initiate the exit ("main" call)
		conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())
		// this connection will immediately return since graceful exit has not been initiated yet
		c, err := client.Process(ctx)
		require.NoError(t, err)
		response, err := c.Recv()
		require.NoError(t, err)
		switch response.GetMessage().(type) {
		case *pb.SatelliteMessage_NotReady:
		default:
			t.FailNow()
		}

		// wait for initial loop to start so we have pieces to transfer
		satellite.GracefulExit.Chore.Loop.TriggerWait()

		// this connection should not close immediately, since there are pieces to transfer
		c, err = client.Process(ctx)
		require.NoError(t, err)

		_, err = c.Recv()
		require.NoError(t, err)

		// start receiving from concurrent connections
		wg.Done()

		err = group.Wait()
		require.NoError(t, err)
	})
}

func TestRecvTimeout(t *testing.T) {
	successThreshold := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 1,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			NewStorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
				// This config value will create a very short timeframe allowed for receiving
				// data from storage nodes. This will cause context to cancel with timeout.
				config.GracefulExit.RecvTimeout = 10 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
		}

		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitingNode, err := findNodeToExit(ctx, planet, 1)
		require.NoError(t, err)
		exitingNode.GracefulExit.Chore.Loop.Pause()

		exitStatusReq := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now(),
		}
		_, err = satellite.Overlay.DB.UpdateExitStatus(ctx, &exitStatusReq)
		require.NoError(t, err)

		// run the satellite chore to build the transfer queue.
		satellite.GracefulExit.Chore.Loop.TriggerWait()
		satellite.GracefulExit.Chore.Loop.Pause()

		// check that the satellite knows the storage node is exiting.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 1)
		require.Equal(t, exitingNode.ID(), exitingNodes[0].NodeID)

		queueItems, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 10, 0)
		require.NoError(t, err)
		require.Len(t, queueItems, 1)

		storageNodeDB := exitingNode.DB.(*testblobs.SlowDB)
		// make uploads on storage node slower than the timeout for transferring bytes to another node
		delay := 200 * time.Millisecond
		storageNodeDB.SetLatency(delay)
		store := pieces.NewStore(zaptest.NewLogger(t), storageNodeDB.Pieces(), nil, nil, storageNodeDB.PieceSpaceUsedDB())

		// run the SN chore again to start processing transfers.
		worker := gracefulexit.NewWorker(zaptest.NewLogger(t), store, exitingNode.DB.Satellites(), exitingNode.Dialer, satellite.ID(), satellite.Addr(),
			gracefulexit.Config{
				ChoreInterval:          0,
				NumWorkers:             2,
				NumConcurrentTransfers: 2,
				MinBytesPerSecond:      128,
				MinDownloadTimeout:     2 * time.Minute,
			})
		defer ctx.Check(worker.Close)

		err = worker.Run(ctx, func() {})
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.DeadlineExceeded))
	})
}

func TestInvalidStorageNodeSignature(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		response, err := processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)
			pieceReader, err := exitingNode.Storage2.Store.Reader(ctx, satellite.ID(), m.TransferPiece.OriginalPieceId)
			require.NoError(t, err)

			header, err := pieceReader.GetPieceHeader()
			require.NoError(t, err)

			orderLimit := header.OrderLimit

			originalPieceHash := &pb.PieceHash{
				PieceId:   orderLimit.PieceId,
				Hash:      header.GetHash(),
				PieceSize: pieceReader.Size(),
				Timestamp: header.GetCreationTime(),
				Signature: header.GetSignature(),
			}

			newPieceHash := &pb.PieceHash{
				PieceId:   m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
				Hash:      originalPieceHash.Hash,
				PieceSize: originalPieceHash.PieceSize,
				Timestamp: time.Now(),
			}

			wrongSigner := signing.SignerFromFullIdentity(exitingNode.Identity)

			signedNewPieceHash, err := signing.SignPieceHash(ctx, wrongSigner, newPieceHash)
			require.NoError(t, err)

			message := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Succeeded{
					Succeeded: &pb.TransferSucceeded{
						OriginalPieceId:      m.TransferPiece.OriginalPieceId,
						OriginalPieceHash:    originalPieceHash,
						OriginalOrderLimit:   &orderLimit,
						ReplacementPieceHash: signedNewPieceHash,
					},
				},
			}
			err = processClient.Send(message)
			require.NoError(t, err)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_ExitFailed:
			require.NotNil(t, m)
			require.NotNil(t, m.ExitFailed)
			require.Equal(t, m.ExitFailed.Reason, pb.ExitFailed_VERIFICATION_FAILED)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.Equal(t, int64(0), progress.PiecesTransferred)
		require.Equal(t, int64(1), progress.PiecesFailed)
	})
}

func TestExitDisqualifiedNodeFailOnStart(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[0]

		disqualifyNode(t, ctx, satellite, exitingNode.ID())

		conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())
		processClient, err := client.Process(ctx)
		require.NoError(t, err)

		// Process endpoint should return immediately if node is disqualified
		response, err := processClient.Recv()
		require.True(t, errs2.IsRPC(err, rpcstatus.FailedPrecondition))
		require.Nil(t, response)

		// disqualified node should fail graceful exit
		exitStatus, err := satellite.Overlay.DB.GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.NotNil(t, exitStatus.ExitFinishedAt)
		require.False(t, exitStatus.ExitSuccess)
	})

}

func TestExitDisqualifiedNodeFailEventually(t *testing.T) {
	testTransfers(t, numObjects, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		disqualifyNode(t, ctx, satellite, exitingNode.ID())

		deletedCount := 0
		for {
			response, err := processClient.Recv()
			if errs.Is(err, io.EOF) {
				// Done
				break
			}
			if deletedCount >= numPieces {
				// when a disqualified node has finished transfer all pieces, it should receive an error
				require.True(t, errs2.IsRPC(err, rpcstatus.FailedPrecondition))
				break
			} else {
				require.NoError(t, err)
			}

			switch m := response.GetMessage().(type) {
			case *pb.SatelliteMessage_TransferPiece:
				require.NotNil(t, m)

				pieceReader, err := exitingNode.Storage2.Store.Reader(ctx, satellite.ID(), m.TransferPiece.OriginalPieceId)
				require.NoError(t, err)

				header, err := pieceReader.GetPieceHeader()
				require.NoError(t, err)

				orderLimit := header.OrderLimit
				originalPieceHash := &pb.PieceHash{
					PieceId:   orderLimit.PieceId,
					Hash:      header.GetHash(),
					PieceSize: pieceReader.Size(),
					Timestamp: header.GetCreationTime(),
					Signature: header.GetSignature(),
				}

				newPieceHash := &pb.PieceHash{
					PieceId:   m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
					Hash:      originalPieceHash.Hash,
					PieceSize: originalPieceHash.PieceSize,
					Timestamp: time.Now(),
				}

				receivingNodeID := nodeFullIDs[m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId]
				require.NotNil(t, receivingNodeID)
				signer := signing.SignerFromFullIdentity(receivingNodeID)

				signedNewPieceHash, err := signing.SignPieceHash(ctx, signer, newPieceHash)
				require.NoError(t, err)

				success := &pb.StorageNodeMessage{
					Message: &pb.StorageNodeMessage_Succeeded{
						Succeeded: &pb.TransferSucceeded{
							OriginalPieceId:      m.TransferPiece.OriginalPieceId,
							OriginalPieceHash:    originalPieceHash,
							OriginalOrderLimit:   &orderLimit,
							ReplacementPieceHash: signedNewPieceHash,
						},
					},
				}
				err = processClient.Send(success)
				require.NoError(t, err)
			case *pb.SatelliteMessage_DeletePiece:
				deletedCount++
			default:
				t.FailNow()
			}
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.EqualValues(t, numPieces, progress.PiecesTransferred)
		require.EqualValues(t, numPieces, deletedCount)

		// disqualified node should fail graceful exit
		exitStatus, err := satellite.Overlay.DB.GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.NotNil(t, exitStatus.ExitFinishedAt)
		require.False(t, exitStatus.ExitSuccess)
	})
}

func TestFailureHashMismatch(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		response, err := processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)
			pieceReader, err := exitingNode.Storage2.Store.Reader(ctx, satellite.ID(), m.TransferPiece.OriginalPieceId)
			require.NoError(t, err)

			header, err := pieceReader.GetPieceHeader()
			require.NoError(t, err)

			orderLimit := header.OrderLimit
			originalPieceHash := &pb.PieceHash{
				PieceId:   orderLimit.PieceId,
				Hash:      header.GetHash(),
				PieceSize: pieceReader.Size(),
				Timestamp: header.GetCreationTime(),
				Signature: header.GetSignature(),
			}

			newPieceHash := &pb.PieceHash{
				PieceId:   m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
				Hash:      originalPieceHash.Hash[:1],
				PieceSize: originalPieceHash.PieceSize,
				Timestamp: time.Now(),
			}

			receivingNodeID := nodeFullIDs[m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId]
			require.NotNil(t, receivingNodeID)
			signer := signing.SignerFromFullIdentity(receivingNodeID)

			signedNewPieceHash, err := signing.SignPieceHash(ctx, signer, newPieceHash)
			require.NoError(t, err)

			message := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Succeeded{
					Succeeded: &pb.TransferSucceeded{
						OriginalPieceId:      m.TransferPiece.OriginalPieceId,
						OriginalPieceHash:    originalPieceHash,
						OriginalOrderLimit:   &orderLimit,
						ReplacementPieceHash: signedNewPieceHash,
					},
				},
			}
			err = processClient.Send(message)
			require.NoError(t, err)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_ExitFailed:
			require.NotNil(t, m)
			require.NotNil(t, m.ExitFailed)
			require.Equal(t, m.ExitFailed.Reason, pb.ExitFailed_VERIFICATION_FAILED)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.Equal(t, int64(0), progress.PiecesTransferred)
		require.Equal(t, int64(1), progress.PiecesFailed)
	})
}

func TestFailureUnknownError(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		response, err := processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)
			message := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Failed{
					Failed: &pb.TransferFailed{
						Error:           pb.TransferFailed_UNKNOWN,
						OriginalPieceId: m.TransferPiece.OriginalPieceId,
					},
				},
			}
			err = processClient.Send(message)
			require.NoError(t, err)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.Equal(t, int64(0), progress.PiecesTransferred)
		require.Equal(t, int64(0), progress.PiecesFailed)
	})
}

func TestFailureUplinkSignature(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		response, err := processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)
			pieceReader, err := exitingNode.Storage2.Store.Reader(ctx, satellite.ID(), m.TransferPiece.OriginalPieceId)
			require.NoError(t, err)

			header, err := pieceReader.GetPieceHeader()
			require.NoError(t, err)

			orderLimit := header.OrderLimit
			orderLimit.UplinkPublicKey = storj.PiecePublicKey{}

			originalPieceHash := &pb.PieceHash{
				PieceId:   orderLimit.PieceId,
				Hash:      header.GetHash(),
				PieceSize: pieceReader.Size(),
				Timestamp: header.GetCreationTime(),
				Signature: header.GetSignature(),
			}

			newPieceHash := &pb.PieceHash{
				PieceId:   m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
				Hash:      originalPieceHash.Hash,
				PieceSize: originalPieceHash.PieceSize,
				Timestamp: time.Now(),
			}

			receivingNodeID := nodeFullIDs[m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId]
			require.NotNil(t, receivingNodeID)
			signer := signing.SignerFromFullIdentity(receivingNodeID)

			signedNewPieceHash, err := signing.SignPieceHash(ctx, signer, newPieceHash)
			require.NoError(t, err)

			message := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Succeeded{
					Succeeded: &pb.TransferSucceeded{
						OriginalPieceId:      m.TransferPiece.OriginalPieceId,
						OriginalPieceHash:    originalPieceHash,
						OriginalOrderLimit:   &orderLimit,
						ReplacementPieceHash: signedNewPieceHash,
					},
				},
			}
			err = processClient.Send(message)
			require.NoError(t, err)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_ExitFailed:
			require.NotNil(t, m)
			require.NotNil(t, m.ExitFailed)
			require.Equal(t, m.ExitFailed.Reason, pb.ExitFailed_VERIFICATION_FAILED)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.Equal(t, int64(0), progress.PiecesTransferred)
		require.Equal(t, int64(1), progress.PiecesFailed)
	})
}

func TestSuccessPointerUpdate(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		var recNodeID storj.NodeID

		response, err := processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)

			pieceReader, err := exitingNode.Storage2.Store.Reader(ctx, satellite.ID(), m.TransferPiece.OriginalPieceId)
			require.NoError(t, err)

			header, err := pieceReader.GetPieceHeader()
			require.NoError(t, err)

			orderLimit := header.OrderLimit
			originalPieceHash := &pb.PieceHash{
				PieceId:   orderLimit.PieceId,
				Hash:      header.GetHash(),
				PieceSize: pieceReader.Size(),
				Timestamp: header.GetCreationTime(),
				Signature: header.GetSignature(),
			}

			newPieceHash := &pb.PieceHash{
				PieceId:   m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
				Hash:      originalPieceHash.Hash,
				PieceSize: originalPieceHash.PieceSize,
				Timestamp: time.Now(),
			}

			receivingIdentity := nodeFullIDs[m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId]
			require.NotNil(t, receivingIdentity)

			// get the receiving node piece count before processing
			recNodeID = receivingIdentity.ID

			signer := signing.SignerFromFullIdentity(receivingIdentity)

			signedNewPieceHash, err := signing.SignPieceHash(ctx, signer, newPieceHash)
			require.NoError(t, err)

			success := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Succeeded{
					Succeeded: &pb.TransferSucceeded{
						OriginalPieceId:      m.TransferPiece.OriginalPieceId,
						OriginalPieceHash:    originalPieceHash,
						OriginalOrderLimit:   &orderLimit,
						ReplacementPieceHash: signedNewPieceHash,
					},
				},
			}
			err = processClient.Send(success)
			require.NoError(t, err)
		default:
			t.FailNow()
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch response.GetMessage().(type) {
		case *pb.SatelliteMessage_DeletePiece:
			// expect the delete piece message
		default:
			t.FailNow()
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.EqualValues(t, numPieces, progress.PiecesTransferred)
		// even though we failed 1, it eventually succeeded, so the count should be 0
		require.EqualValues(t, 0, progress.PiecesFailed)

		keys, err := satellite.Metainfo.Database.List(ctx, nil, 1)
		require.NoError(t, err)

		pointer, err := satellite.Metainfo.Service.Get(ctx, string(keys[0]))
		require.NoError(t, err)

		found := 0
		require.NotNil(t, pointer.GetRemote())
		require.True(t, len(pointer.GetRemote().GetRemotePieces()) > 0)
		for _, piece := range pointer.GetRemote().GetRemotePieces() {
			require.NotEqual(t, exitingNode.ID(), piece.NodeId)
			if piece.NodeId == recNodeID {
				found++
			}
		}
		require.Equal(t, 1, found)
	})
}

func TestUpdatePointerFailure_DuplicatedNodeID(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		response, err := processClient.Recv()
		require.NoError(t, err)

		var firstRecNodeID storj.NodeID
		var pieceID storj.PieceID
		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			firstRecNodeID = m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId
			pieceID = m.TransferPiece.OriginalPieceId

			pieceReader, err := exitingNode.Storage2.Store.Reader(ctx, satellite.ID(), pieceID)
			require.NoError(t, err)

			header, err := pieceReader.GetPieceHeader()
			require.NoError(t, err)

			orderLimit := header.OrderLimit
			originalPieceHash := &pb.PieceHash{
				PieceId:   orderLimit.PieceId,
				Hash:      header.GetHash(),
				PieceSize: pieceReader.Size(),
				Timestamp: header.GetCreationTime(),
				Signature: header.GetSignature(),
			}

			newPieceHash := &pb.PieceHash{
				PieceId:   m.TransferPiece.AddressedOrderLimit.Limit.PieceId,
				Hash:      originalPieceHash.Hash,
				PieceSize: originalPieceHash.PieceSize,
				Timestamp: time.Now(),
			}

			receivingNodeIdentity := nodeFullIDs[m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId]
			require.NotNil(t, receivingNodeIdentity)
			signer := signing.SignerFromFullIdentity(receivingNodeIdentity)

			signedNewPieceHash, err := signing.SignPieceHash(ctx, signer, newPieceHash)
			require.NoError(t, err)

			success := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Succeeded{
					Succeeded: &pb.TransferSucceeded{
						OriginalPieceId:      pieceID,
						OriginalPieceHash:    originalPieceHash,
						OriginalOrderLimit:   &orderLimit,
						ReplacementPieceHash: signedNewPieceHash,
					},
				},
			}

			// update pointer to include the new receiving node before responding to satellite
			keys, err := satellite.Metainfo.Database.List(ctx, nil, 1)
			require.NoError(t, err)
			path := string(keys[0])
			pointer, err := satellite.Metainfo.Service.Get(ctx, path)
			require.NoError(t, err)
			require.NotNil(t, pointer.GetRemote())
			require.True(t, len(pointer.GetRemote().GetRemotePieces()) > 0)

			pieceToRemove := make([]*pb.RemotePiece, 1)
			pieceToAdd := make([]*pb.RemotePiece, 1)
			pieces := pointer.GetRemote().GetRemotePieces()

			for _, piece := range pieces {
				if pieceToRemove[0] == nil && piece.NodeId != exitingNode.ID() {
					pieceToRemove[0] = piece
					continue
				}
			}

			pieceToAdd[0] = &pb.RemotePiece{
				PieceNum: pieceToRemove[0].PieceNum,
				NodeId:   firstRecNodeID,
			}

			_, err = satellite.Metainfo.Service.UpdatePieces(ctx, path, pointer, pieceToAdd, pieceToRemove)
			require.NoError(t, err)

			err = processClient.Send(success)
			require.NoError(t, err)
		default:
			t.FailNow()
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			// validate we get a new node to transfer too
			require.True(t, m.TransferPiece.OriginalPieceId == pieceID)
			require.True(t, m.TransferPiece.AddressedOrderLimit.Limit.StorageNodeId != firstRecNodeID)
		default:
			t.FailNow()
		}

		// check exiting node is still in the pointer
		keys, err := satellite.Metainfo.Database.List(ctx, nil, 1)
		require.NoError(t, err)
		path := string(keys[0])
		pointer, err := satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)
		require.NotNil(t, pointer.GetRemote())
		require.True(t, len(pointer.GetRemote().GetRemotePieces()) > 0)

		pieces := pointer.GetRemote().GetRemotePieces()

		pieceMap := make(map[storj.NodeID]int)
		for _, piece := range pieces {
			pieceMap[piece.NodeId]++
		}

		exitingNodeID := exitingNode.ID()
		count, ok := pieceMap[exitingNodeID]
		require.True(t, ok)
		require.Equal(t, 1, count)
		count, ok = pieceMap[firstRecNodeID]
		require.True(t, ok)
		require.Equal(t, 1, count)
	})
}

func TestExitDisabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.GracefulExit.Enabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[0]

		require.Nil(t, satellite.GracefulExit.Chore)
		require.Nil(t, satellite.GracefulExit.Endpoint)

		conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())
		processClient, err := client.Process(ctx)
		require.NoError(t, err)

		// Process endpoint should return immediately if GE is disabled
		response, err := processClient.Recv()
		require.Error(t, err)
		// grpc will return "Unimplemented", drpc will return "Unknown"
		unimplementedOrUnknown := errs2.IsRPC(err, rpcstatus.Unimplemented) || errs2.IsRPC(err, rpcstatus.Unknown)
		require.True(t, unimplementedOrUnknown)
		require.Nil(t, response)
	})
}

func TestPointerChangedOrDeleted(t *testing.T) {
	successThreshold := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path0", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)
		err = uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		exitingNode, err := findNodeToExit(ctx, planet, 2)
		require.NoError(t, err)

		exitRequest := &overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now(),
		}

		_, err = satellite.DB.OverlayCache().UpdateExitStatus(ctx, exitRequest)
		require.NoError(t, err)
		err = satellite.DB.GracefulExit().IncrementProgress(ctx, exitingNode.ID(), 0, 0, 0)
		require.NoError(t, err)

		exitingNodes, err = satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 1)
		require.Equal(t, exitingNode.ID(), exitingNodes[0].NodeID)

		// trigger the metainfo loop chore so we can get some pieces to transfer
		satellite.GracefulExit.Chore.Loop.TriggerWait()

		// make sure all the pieces are in the transfer queue
		incomplete, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 10, 0)
		require.NoError(t, err)
		require.Len(t, incomplete, 2)

		// updating the first object and deleting the second. this will cause a root piece ID change which will result in
		// a successful graceful exit instead of a request to transfer pieces since the root piece IDs will have changed.
		err = uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path0", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)
		err = uplinkPeer.Delete(ctx, satellite, "testbucket", "test/path1")
		require.NoError(t, err)

		// reconnect to the satellite.
		conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())

		c, err := client.Process(ctx)
		require.NoError(t, err)
		defer ctx.Check(c.CloseSend)

		response, err := c.Recv()
		require.NoError(t, err)

		// we expect an exit completed b/c there is nothing to do here
		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_ExitCompleted:
			signee := signing.SigneeFromPeerIdentity(satellite.Identity.PeerIdentity())
			err = signing.VerifyExitCompleted(ctx, signee, m.ExitCompleted)
			require.NoError(t, err)

			exitStatus, err := satellite.DB.OverlayCache().GetExitStatus(ctx, exitingNode.ID())
			require.NoError(t, err)
			require.NotNil(t, exitStatus.ExitFinishedAt)
			require.True(t, exitStatus.ExitSuccess)
		default:
			t.FailNow()
		}

		queueItems, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 2, 0)
		require.NoError(t, err)
		require.Len(t, queueItems, 0)
	})
}

func TestFailureNotFoundPieceHashVerified(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		response, err := processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)

			message := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Failed{
					Failed: &pb.TransferFailed{
						OriginalPieceId: m.TransferPiece.OriginalPieceId,
						Error:           pb.TransferFailed_NOT_FOUND,
					},
				},
			}
			err = processClient.Send(message)
			require.NoError(t, err)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_ExitFailed:
			require.NotNil(t, m)
			require.NotNil(t, m.ExitFailed)
			require.Equal(t, m.ExitFailed.Reason, pb.ExitFailed_OVERALL_FAILURE_PERCENTAGE_EXCEEDED)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		// check that node is no longer in the pointer
		keys, err := satellite.Metainfo.Database.List(ctx, nil, -1)
		require.NoError(t, err)

		var pointer *pb.Pointer
		for _, key := range keys {
			p, err := satellite.Metainfo.Service.Get(ctx, string(key))
			require.NoError(t, err)

			if p.GetRemote() != nil {
				pointer = p
				break
			}
		}
		require.NotNil(t, pointer)
		for _, piece := range pointer.GetRemote().GetRemotePieces() {
			require.NotEqual(t, piece.NodeId, exitingNode.ID())
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.Equal(t, int64(0), progress.PiecesTransferred)
		require.Equal(t, int64(1), progress.PiecesFailed)
	})

}

func TestFailureNotFoundPieceHashUnverified(t *testing.T) {
	testTransfers(t, 1, func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int) {
		// retrieve remote segment
		keys, err := satellite.Metainfo.Database.List(ctx, nil, -1)
		require.NoError(t, err)

		var oldPointer *pb.Pointer
		var path []byte
		for _, key := range keys {
			p, err := satellite.Metainfo.Service.Get(ctx, string(key))
			require.NoError(t, err)

			if p.GetRemote() != nil {
				oldPointer = p
				path = key
				break
			}
		}

		// replace pointer with non-piece-hash-verified pointer
		require.NotNil(t, oldPointer)
		oldPointerBytes, err := proto.Marshal(oldPointer)
		require.NoError(t, err)
		newPointer := &pb.Pointer{}
		err = proto.Unmarshal(oldPointerBytes, newPointer)
		require.NoError(t, err)
		newPointer.PieceHashesVerified = false
		newPointerBytes, err := proto.Marshal(newPointer)
		require.NoError(t, err)
		err = satellite.Metainfo.Database.CompareAndSwap(ctx, storage.Key(path), oldPointerBytes, newPointerBytes)
		require.NoError(t, err)

		// begin processing graceful exit messages
		response, err := processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_TransferPiece:
			require.NotNil(t, m)

			message := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Failed{
					Failed: &pb.TransferFailed{
						OriginalPieceId: m.TransferPiece.OriginalPieceId,
						Error:           pb.TransferFailed_NOT_FOUND,
					},
				},
			}
			err = processClient.Send(message)
			require.NoError(t, err)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		response, err = processClient.Recv()
		require.NoError(t, err)

		switch m := response.GetMessage().(type) {
		case *pb.SatelliteMessage_ExitCompleted:
			require.NotNil(t, m)
		default:
			require.FailNow(t, "should not reach this case: %#v", m)
		}

		// check that node is no longer in the pointer
		keys, err = satellite.Metainfo.Database.List(ctx, nil, -1)
		require.NoError(t, err)

		var pointer *pb.Pointer
		for _, key := range keys {
			p, err := satellite.Metainfo.Service.Get(ctx, string(key))
			require.NoError(t, err)

			if p.GetRemote() != nil {
				pointer = p
				break
			}
		}
		require.NotNil(t, pointer)
		for _, piece := range pointer.GetRemote().GetRemotePieces() {
			require.NotEqual(t, piece.NodeId, exitingNode.ID())
		}

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)

		require.Equal(t, int64(0), progress.PiecesTransferred)
		require.Equal(t, int64(0), progress.PiecesFailed)
	})

}

func TestFailureStorageNodeIgnoresTransferMessages(t *testing.T) {
	var maxOrderLimitSendCount = 3
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 5,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
				// We don't care whether a node gracefully exits or not in this test,
				// so we set the max failures percentage extra high.
				config.GracefulExit.OverallMaxFailuresPercentage = 101
				config.GracefulExit.MaxOrderLimitSendCount = maxOrderLimitSendCount
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		nodeFullIDs := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			nodeFullIDs[node.ID()] = node.Identity
		}

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     4,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		exitingNode, err := findNodeToExit(ctx, planet, 1)
		require.NoError(t, err)

		// connect to satellite so we initiate the exit.
		conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())

		c, err := client.Process(ctx)
		require.NoError(t, err)

		response, err := c.Recv()
		require.NoError(t, err)

		// should get a NotReady since the metainfo loop would not be finished at this point.
		switch response.GetMessage().(type) {
		case *pb.SatelliteMessage_NotReady:
			// now check that the exiting node is initiated.
			exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
			require.NoError(t, err)
			require.Len(t, exitingNodes, 1)

			require.Equal(t, exitingNode.ID(), exitingNodes[0].NodeID)
		default:
			t.FailNow()
		}
		// close the old client
		require.NoError(t, c.CloseSend())

		// trigger the metainfo loop chore so we can get some pieces to transfer
		satellite.GracefulExit.Chore.Loop.TriggerWait()

		// make sure all the pieces are in the transfer queue
		_, err = satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 1, 0)
		require.NoError(t, err)

		var messageCount int

		// We need to label this outer loop so that we're able to exit it from the inner loop.
		// The outer loop is for sending the request from node to satellite multiple times.
		// The inner loop is for reading the response.
	MessageLoop:
		for {
			var unknownMsgSent bool
			c, err := client.Process(ctx)
			require.NoError(t, err)

			for {
				response, err := c.Recv()
				if unknownMsgSent {
					require.Error(t, err)
					break
				} else {
					require.NoError(t, err)
				}

				switch response.GetMessage().(type) {
				case *pb.SatelliteMessage_ExitCompleted:
					break MessageLoop
				case *pb.SatelliteMessage_TransferPiece:
					messageCount++
					unknownMsgSent = true
					// We send an unknown message because we want to fail the
					// transfer message request we get from the satellite.
					// This allows us to keep the conn open but repopulate
					// the pending queue.
					err = c.Send(&pb.StorageNodeMessage{})
					require.NoError(t, err)
					require.NoError(t, c.CloseSend())
				default:
					t.FailNow()
				}
			}
		}
		require.Equal(t, messageCount, maxOrderLimitSendCount)

		// make sure not responding piece not in queue
		incompletes, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 10, 0)
		require.NoError(t, err)
		require.Len(t, incompletes, 0)

		// check that the exit has completed and we have the correct transferred/failed values
		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.EqualValues(t, 1, progress.PiecesFailed)
		status, err := satellite.DB.OverlayCache().GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.NotNil(t, status.ExitFinishedAt)
	})
}

func testTransfers(t *testing.T, objects int, verifier func(ctx *testcontext.Context, nodeFullIDs map[storj.NodeID]*identity.FullIdentity, satellite *testplanet.SatelliteSystem, processClient exitProcessClient, exitingNode *storagenode.Peer, numPieces int)) {
	successThreshold := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		nodeFullIDs := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			nodeFullIDs[node.ID()] = node.Identity
		}

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
		}

		for i := 0; i < objects; i++ {
			err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path"+strconv.Itoa(i), testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)
		}

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		exitingNode, err := findNodeToExit(ctx, planet, objects)
		require.NoError(t, err)

		// connect to satellite so we initiate the exit.
		conn, err := exitingNode.Dialer.DialAddressID(ctx, satellite.Addr(), satellite.Identity.ID)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())

		c, err := client.Process(ctx)
		require.NoError(t, err)

		response, err := c.Recv()
		require.NoError(t, err)

		// should get a NotReady since the metainfo loop would not be finished at this point.
		switch response.GetMessage().(type) {
		case *pb.SatelliteMessage_NotReady:
			// now check that the exiting node is initiated.
			exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
			require.NoError(t, err)
			require.Len(t, exitingNodes, 1)

			require.Equal(t, exitingNode.ID(), exitingNodes[0].NodeID)
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
		c, err = client.Process(ctx)
		require.NoError(t, err)
		defer ctx.Check(c.CloseSend)

		verifier(ctx, nodeFullIDs, satellite, c, exitingNode, len(incompleteTransfers))
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

func disqualifyNode(t *testing.T, ctx *testcontext.Context, satellite *testplanet.SatelliteSystem, nodeID storj.NodeID) {
	nodeStat, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
		NodeID:       nodeID,
		IsUp:         true,
		AuditSuccess: false,
		AuditLambda:  0,
		AuditWeight:  1,
		AuditDQ:      0.5,
		UptimeLambda: 1,
		UptimeWeight: 1,
		UptimeDQ:     0.5,
	})
	require.NoError(t, err)
	require.NotNil(t, nodeStat.Disqualified)
}
