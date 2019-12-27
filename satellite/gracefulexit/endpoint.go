// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink/eestream"
)

// millis for the transfer queue building ticker
const buildQueueMillis = 100

var (
	// ErrInvalidArgument is an error class for invalid argument errors used to check which rpc code to use.
	ErrInvalidArgument = errs.Class("graceful exit")
)

// drpcEndpoint wraps streaming methods so that they can be used with drpc
type drpcEndpoint struct{ *Endpoint }

// processStream is the minimum interface required to process requests.
type processStream interface {
	Context() context.Context
	Send(*pb.SatelliteMessage) error
	Recv() (*pb.StorageNodeMessage, error)
}

// Endpoint for handling the transfer of pieces for Graceful Exit.
type Endpoint struct {
	log            *zap.Logger
	interval       time.Duration
	signer         signing.Signer
	db             DB
	overlaydb      overlay.DB
	overlay        *overlay.Service
	metainfo       *metainfo.Service
	orders         *orders.Service
	connections    *connectionsTracker
	peerIdentities overlay.PeerIdentities
	config         Config
	recvTimeout    time.Duration
}

// connectionsTracker for tracking ongoing connections on this api server
type connectionsTracker struct {
	mu   sync.RWMutex
	data map[storj.NodeID]struct{}
}

// newConnectionsTracker creates a new connectionsTracker and instantiates the map.
func newConnectionsTracker() *connectionsTracker {
	return &connectionsTracker{
		data: make(map[storj.NodeID]struct{}),
	}
}

// tryAdd adds to the map if the node ID is not already added
// it returns true if succeeded and false if already added.
func (pm *connectionsTracker) tryAdd(nodeID storj.NodeID) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.data[nodeID]; ok {
		return false
	}
	pm.data[nodeID] = struct{}{}
	return true
}

// delete deletes a node ID from the map.
func (pm *connectionsTracker) delete(nodeID storj.NodeID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.data, nodeID)
}

// DRPC returns a DRPC form of the endpoint.
func (endpoint *Endpoint) DRPC() pb.DRPCSatelliteGracefulExitServer {
	return &drpcEndpoint{Endpoint: endpoint}
}

// NewEndpoint creates a new graceful exit endpoint.
func NewEndpoint(log *zap.Logger, signer signing.Signer, db DB, overlaydb overlay.DB, overlay *overlay.Service, metainfo *metainfo.Service, orders *orders.Service,
	peerIdentities overlay.PeerIdentities, config Config) *Endpoint {
	return &Endpoint{
		log:            log,
		interval:       time.Millisecond * buildQueueMillis,
		signer:         signer,
		db:             db,
		overlaydb:      overlaydb,
		overlay:        overlay,
		metainfo:       metainfo,
		orders:         orders,
		connections:    newConnectionsTracker(),
		peerIdentities: peerIdentities,
		config:         config,
		recvTimeout:    config.RecvTimeout,
	}
}

// Process is called by storage nodes to receive pieces to transfer to new nodes and get exit status.
func (endpoint *Endpoint) Process(stream pb.SatelliteGracefulExit_ProcessServer) (err error) {
	return endpoint.doProcess(stream)
}

// Process is called by storage nodes to receive pieces to transfer to new nodes and get exit status.
func (endpoint *drpcEndpoint) Process(stream pb.DRPCSatelliteGracefulExit_ProcessStream) error {
	return endpoint.doProcess(stream)
}

func (endpoint *Endpoint) doProcess(stream processStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	nodeID := peer.ID
	endpoint.log.Debug("graceful exit process", zap.Stringer("Node ID", nodeID))

	// ensure that only one connection can be opened for a single node at a time
	if !endpoint.connections.tryAdd(nodeID) {
		return rpcstatus.Error(rpcstatus.Aborted, "Only one concurrent connection allowed for graceful exit")
	}
	defer func() {
		endpoint.connections.delete(nodeID)
	}()

	isDisqualified, err := endpoint.handleDisqualifiedNode(ctx, nodeID)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	if isDisqualified {
		return rpcstatus.Error(rpcstatus.FailedPrecondition, "Disqualified nodes cannot graceful exit")
	}

	msg, err := endpoint.checkExitStatus(ctx, nodeID)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if msg != nil {
		err = stream.Send(msg)
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		return nil
	}

	// maps pieceIDs to pendingTransfers to keep track of ongoing piece transfer requests
	// and handles concurrency between sending logic and receiving logic
	pending := NewPendingMap()

	var group errgroup.Group
	group.Go(func() error {
		incompleteLoop := sync2.NewCycle(endpoint.interval)

		// we cancel this context in all situations where we want to exit the loop
		ctx, cancel := context.WithCancel(ctx)
		loopErr := incompleteLoop.Run(ctx, func(ctx context.Context) error {
			if pending.Length() == 0 {
				incomplete, err := endpoint.db.GetIncompleteNotFailed(ctx, nodeID, endpoint.config.EndpointBatchSize, 0)
				if err != nil {
					cancel()
					return pending.DoneSending(err)
				}

				if len(incomplete) == 0 {
					incomplete, err = endpoint.db.GetIncompleteFailed(ctx, nodeID, endpoint.config.MaxFailuresPerPiece, endpoint.config.EndpointBatchSize, 0)
					if err != nil {
						cancel()
						return pending.DoneSending(err)
					}
				}

				if len(incomplete) == 0 {
					endpoint.log.Debug("no more pieces to transfer for node", zap.Stringer("Node ID", nodeID))
					cancel()
					return pending.DoneSending(nil)
				}

				for _, inc := range incomplete {
					err = endpoint.processIncomplete(ctx, stream, pending, inc)
					if err != nil {
						cancel()
						return pending.DoneSending(err)
					}
				}
			}
			return nil
		})
		return errs2.IgnoreCanceled(loopErr)
	})

	for {
		finishedPromise := pending.IsFinishedPromise()
		finished, err := finishedPromise.Wait(ctx)
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		// if there is no more work to receive send complete
		if finished {
			isDisqualified, err := endpoint.handleDisqualifiedNode(ctx, nodeID)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}
			if isDisqualified {
				return rpcstatus.Error(rpcstatus.FailedPrecondition, "Disqualified nodes cannot graceful exit")
			}

			// update exit status
			exitStatusRequest, exitFailedReason, err := endpoint.generateExitStatusRequest(ctx, nodeID)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}

			err = endpoint.handleFinished(ctx, stream, exitStatusRequest, exitFailedReason)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}
			break
		}

		done := make(chan struct{})
		var request *pb.StorageNodeMessage
		var recvErr error
		go func() {
			request, recvErr = stream.Recv()
			close(done)
		}()

		timer := time.NewTimer(endpoint.recvTimeout)

		select {
		case <-ctx.Done():
			return rpcstatus.Error(rpcstatus.Internal, Error.New("context canceled while waiting to receive message from storagenode").Error())
		case <-timer.C:
			return rpcstatus.Error(rpcstatus.DeadlineExceeded, Error.New("timeout while waiting to receive message from storagenode").Error())
		case <-done:
		}
		if recvErr != nil {
			if errs.Is(recvErr, io.EOF) {
				endpoint.log.Debug("received EOF when trying to receive messages from storage node", zap.Stringer("node ID", nodeID))
				return nil
			}
			return rpcstatus.Error(rpcstatus.Unknown, Error.Wrap(recvErr).Error())
		}

		switch m := request.GetMessage().(type) {
		case *pb.StorageNodeMessage_Succeeded:
			err = endpoint.handleSucceeded(ctx, stream, pending, nodeID, m)
			if err != nil {
				if metainfo.ErrNodeAlreadyExists.Has(err) {
					// this will get retried
					endpoint.log.Warn("node already exists in pointer.", zap.Error(err))

					continue
				}
				if ErrInvalidArgument.Has(err) {
					// immediately fail and complete graceful exit for nodes that fail satellite validation
					err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
					if err != nil {
						return rpcstatus.Error(rpcstatus.Internal, err.Error())
					}

					mon.Meter("graceful_exit_fail_validation").Mark(1) //locked

					exitStatusRequest := &overlay.ExitStatusRequest{
						NodeID:         nodeID,
						ExitFinishedAt: time.Now().UTC(),
						ExitSuccess:    false,
					}

					err := endpoint.handleFinished(ctx, stream, exitStatusRequest, pb.ExitFailed_VERIFICATION_FAILED)
					if err != nil {
						return rpcstatus.Error(rpcstatus.Internal, err.Error())
					}
					break
				}
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}
		case *pb.StorageNodeMessage_Failed:
			err = endpoint.handleFailed(ctx, pending, nodeID, m)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
			}
		default:
			return rpcstatus.Error(rpcstatus.Unknown, Error.New("unknown storage node message: %v", m).Error())
		}
	}

	if err := group.Wait(); err != nil {
		if !errs.Is(err, context.Canceled) {
			return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
		}
	}

	return nil
}

func (endpoint *Endpoint) processIncomplete(ctx context.Context, stream processStream, pending *PendingMap, incomplete *TransferQueueItem) error {
	nodeID := incomplete.NodeID

	if incomplete.OrderLimitSendCount >= endpoint.config.MaxOrderLimitSendCount {
		err := endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
		if err != nil {
			return Error.Wrap(err)
		}
		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.Path, incomplete.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	}

	pointer, err := endpoint.getValidPointer(ctx, string(incomplete.Path), incomplete.PieceNum, incomplete.RootPieceID)
	if err != nil {
		endpoint.log.Warn("invalid pointer", zap.Error(err))
		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.Path, incomplete.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	}

	nodePiece, err := endpoint.getNodePiece(ctx, pointer, incomplete)
	if err != nil {
		deleteErr := endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.Path, incomplete.PieceNum)
		if deleteErr != nil {
			return Error.Wrap(deleteErr)
		}
		return Error.Wrap(err)
	}

	pieceSize, err := endpoint.calculatePieceSize(ctx, pointer, incomplete, nodePiece)
	if ErrAboveOptimalThreshold.Has(err) {
		_, err = endpoint.metainfo.UpdatePieces(ctx, string(incomplete.Path), pointer, nil, []*pb.RemotePiece{nodePiece})
		if err != nil {
			return Error.Wrap(err)
		}

		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.Path, incomplete.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}
		return nil
	}
	if err != nil {
		return Error.Wrap(err)
	}

	// populate excluded node IDs
	remote := pointer.GetRemote()
	pieces := remote.RemotePieces
	excludedNodeIDs := make([]storj.NodeID, len(pieces))
	for i, piece := range pieces {
		excludedNodeIDs[i] = piece.NodeId
	}

	// get replacement node
	request := &overlay.FindStorageNodesRequest{
		RequestedCount: 1,
		FreeBandwidth:  pieceSize,
		FreeDisk:       pieceSize,
		ExcludedNodes:  excludedNodeIDs,
	}

	newNodes, err := endpoint.overlay.FindStorageNodes(ctx, *request)
	if err != nil {
		return Error.Wrap(err)
	}

	if len(newNodes) == 0 {
		return Error.New("could not find a node to receive piece transfer: node ID %v, path %v, piece num %v", nodeID, incomplete.Path, incomplete.PieceNum)
	}

	newNode := newNodes[0]
	endpoint.log.Debug("found new node for piece transfer", zap.Stringer("original node ID", nodeID), zap.Stringer("replacement node ID", newNode.Id),
		zap.ByteString("path", incomplete.Path), zap.Int32("piece num", incomplete.PieceNum))

	pieceID := remote.RootPieceId.Derive(nodeID, incomplete.PieceNum)

	parts := storj.SplitPath(storj.Path(incomplete.Path))
	if len(parts) < 2 {
		return Error.New("invalid path for node ID %v, piece ID %v", incomplete.NodeID, pieceID)
	}

	bucketID := []byte(storj.JoinPaths(parts[0], parts[1]))
	limit, privateKey, err := endpoint.orders.CreateGracefulExitPutOrderLimit(ctx, bucketID, newNode.Id, incomplete.PieceNum, remote.RootPieceId, int32(pieceSize))
	if err != nil {
		return Error.Wrap(err)
	}

	transferMsg := &pb.SatelliteMessage{
		Message: &pb.SatelliteMessage_TransferPiece{
			TransferPiece: &pb.TransferPiece{
				OriginalPieceId:     pieceID,
				AddressedOrderLimit: limit,
				PrivateKey:          privateKey,
			},
		},
	}
	err = stream.Send(transferMsg)
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.db.IncrementOrderLimitSendCount(ctx, nodeID, incomplete.Path, incomplete.PieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	// update pending queue with the transfer item
	err = pending.Put(pieceID, &PendingTransfer{
		Path:             incomplete.Path,
		PieceSize:        pieceSize,
		SatelliteMessage: transferMsg,
		OriginalPointer:  pointer,
		PieceNum:         incomplete.PieceNum,
	})

	return err
}

func (endpoint *Endpoint) handleSucceeded(ctx context.Context, stream processStream, pending *PendingMap, exitingNodeID storj.NodeID, message *pb.StorageNodeMessage_Succeeded) (err error) {
	defer mon.Task()(&ctx)(&err)

	originalPieceID := message.Succeeded.OriginalPieceId

	transfer, ok := pending.Get(originalPieceID)
	if !ok {
		endpoint.log.Error("Could not find transfer item in pending queue", zap.Stringer("Piece ID", originalPieceID))
		return Error.New("Could not find transfer item in pending queue")
	}

	err = endpoint.validatePendingTransfer(ctx, transfer)
	if err != nil {
		return Error.Wrap(err)
	}

	receivingNodeID := transfer.SatelliteMessage.GetTransferPiece().GetAddressedOrderLimit().GetLimit().StorageNodeId
	// get peerID and signee for new storage node
	peerID, err := endpoint.peerIdentities.Get(ctx, receivingNodeID)
	if err != nil {
		return Error.Wrap(err)
	}
	// verify transferred piece
	err = endpoint.verifyPieceTransferred(ctx, message, transfer, peerID)
	if err != nil {
		return Error.Wrap(err)
	}
	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, exitingNodeID, transfer.Path, transfer.PieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.updatePointer(ctx, transfer.OriginalPointer, exitingNodeID, receivingNodeID, string(transfer.Path), transfer.PieceNum, transferQueueItem.RootPieceID)
	if err != nil {
		// remove the piece from the pending queue so it gets retried
		deleteErr := pending.Delete(originalPieceID)

		return Error.Wrap(errs.Combine(err, deleteErr))
	}

	var failed int64
	if transferQueueItem.FailedCount != nil && *transferQueueItem.FailedCount >= endpoint.config.MaxFailuresPerPiece {
		failed = -1
	}

	err = endpoint.db.IncrementProgress(ctx, exitingNodeID, transfer.PieceSize, 1, failed)
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.db.DeleteTransferQueueItem(ctx, exitingNodeID, transfer.Path, transfer.PieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	err = pending.Delete(originalPieceID)
	if err != nil {
		return err
	}

	deleteMsg := &pb.SatelliteMessage{
		Message: &pb.SatelliteMessage_DeletePiece{
			DeletePiece: &pb.DeletePiece{
				OriginalPieceId: originalPieceID,
			},
		},
	}

	err = stream.Send(deleteMsg)
	if err != nil {
		return Error.Wrap(err)
	}

	mon.Meter("graceful_exit_transfer_piece_success").Mark(1) //locked
	return nil
}

func (endpoint *Endpoint) handleFailed(ctx context.Context, pending *PendingMap, nodeID storj.NodeID, message *pb.StorageNodeMessage_Failed) (err error) {
	defer mon.Task()(&ctx)(&err)
	endpoint.log.Warn("transfer failed", zap.Stringer("Piece ID", message.Failed.OriginalPieceId), zap.Stringer("transfer error", message.Failed.GetError()))
	mon.Meter("graceful_exit_transfer_piece_fail").Mark(1) //locked

	pieceID := message.Failed.OriginalPieceId
	transfer, ok := pending.Get(pieceID)
	if !ok {
		endpoint.log.Debug("could not find transfer message in pending queue. skipping.", zap.Stringer("Piece ID", pieceID))

		// TODO we should probably error out here so we don't get stuck in a loop with a SN that is not behaving properl
	}
	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, nodeID, transfer.Path, transfer.PieceNum)
	if err != nil {
		return Error.Wrap(err)
	}
	now := time.Now().UTC()
	failedCount := 1
	if transferQueueItem.FailedCount != nil {
		failedCount = *transferQueueItem.FailedCount + 1
	}

	errorCode := int(pb.TransferFailed_Error_value[message.Failed.Error.String()])

	// If the error code is NOT_FOUND, the node no longer has the piece.
	// Remove the queue item and remove the node from the pointer.
	// If the pointer is not piece hash verified, do not count this as a failure.
	if pb.TransferFailed_Error(errorCode) == pb.TransferFailed_NOT_FOUND {
		endpoint.log.Debug("piece not found on node", zap.Stringer("node ID", nodeID), zap.ByteString("path", transfer.Path), zap.Int32("piece num", transfer.PieceNum))
		pointer, err := endpoint.metainfo.Get(ctx, string(transfer.Path))
		if err != nil {
			return Error.Wrap(err)
		}
		remote := pointer.GetRemote()
		if remote == nil {
			err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, transfer.Path, transfer.PieceNum)
			if err != nil {
				return Error.Wrap(err)
			}
			return pending.Delete(pieceID)
		}
		pieces := remote.GetRemotePieces()

		var nodePiece *pb.RemotePiece
		for _, piece := range pieces {
			if piece.NodeId == nodeID && piece.PieceNum == transfer.PieceNum {
				nodePiece = piece
			}
		}
		if nodePiece == nil {
			err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, transfer.Path, transfer.PieceNum)
			if err != nil {
				return Error.Wrap(err)
			}
			return pending.Delete(pieceID)
		}

		_, err = endpoint.metainfo.UpdatePieces(ctx, string(transfer.Path), pointer, nil, []*pb.RemotePiece{nodePiece})
		if err != nil {
			return Error.Wrap(err)
		}

		// If the pointer was piece hash verified, we know this node definitely should have the piece
		// Otherwise, no penalty.
		if pointer.PieceHashesVerified {
			err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
			if err != nil {
				return Error.Wrap(err)
			}
		}

		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, transfer.Path, transfer.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}
		return pending.Delete(pieceID)
	}

	transferQueueItem.LastFailedAt = &now
	transferQueueItem.FailedCount = &failedCount
	transferQueueItem.LastFailedCode = &errorCode
	err = endpoint.db.UpdateTransferQueueItem(ctx, *transferQueueItem)
	if err != nil {
		return Error.Wrap(err)
	}

	// only increment overall failed count if piece failures has reached the threshold
	if failedCount == endpoint.config.MaxFailuresPerPiece {
		err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return pending.Delete(pieceID)
}

func (endpoint *Endpoint) handleDisqualifiedNode(ctx context.Context, nodeID storj.NodeID) (isDisqualified bool, err error) {
	// check if node is disqualified
	nodeInfo, err := endpoint.overlay.Get(ctx, nodeID)
	if err != nil {
		return false, Error.Wrap(err)
	}

	if nodeInfo.Disqualified != nil {
		// update graceful exit status to be failed
		exitStatusRequest := &overlay.ExitStatusRequest{
			NodeID:         nodeID,
			ExitFinishedAt: time.Now().UTC(),
			ExitSuccess:    false,
		}

		_, err = endpoint.overlaydb.UpdateExitStatus(ctx, exitStatusRequest)
		if err != nil {
			return true, Error.Wrap(err)
		}

		// remove remaining items from the queue
		err = endpoint.db.DeleteTransferQueueItems(ctx, nodeID)
		if err != nil {
			return true, Error.Wrap(err)
		}

		return true, nil
	}

	return false, nil
}

func (endpoint *Endpoint) handleFinished(ctx context.Context, stream processStream, exitStatusRequest *overlay.ExitStatusRequest, failedReason pb.ExitFailed_Reason) error {
	finishedMsg, err := endpoint.getFinishedMessage(ctx, exitStatusRequest.NodeID, exitStatusRequest.ExitFinishedAt, exitStatusRequest.ExitSuccess, failedReason)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = endpoint.overlaydb.UpdateExitStatus(ctx, exitStatusRequest)
	if err != nil {
		return Error.Wrap(err)
	}

	err = stream.Send(finishedMsg)
	if err != nil {
		return Error.Wrap(err)
	}

	// remove remaining items from the queue after notifying nodes about their exit status
	err = endpoint.db.DeleteTransferQueueItems(ctx, exitStatusRequest.NodeID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (endpoint *Endpoint) getFinishedMessage(ctx context.Context, nodeID storj.NodeID, finishedAt time.Time, success bool, reason pb.ExitFailed_Reason) (message *pb.SatelliteMessage, err error) {
	if success {
		unsigned := &pb.ExitCompleted{
			SatelliteId: endpoint.signer.ID(),
			NodeId:      nodeID,
			Completed:   finishedAt,
		}
		signed, err := signing.SignExitCompleted(ctx, endpoint.signer, unsigned)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		message = &pb.SatelliteMessage{Message: &pb.SatelliteMessage_ExitCompleted{
			ExitCompleted: signed,
		}}
	} else {
		unsigned := &pb.ExitFailed{
			SatelliteId: endpoint.signer.ID(),
			NodeId:      nodeID,
			Failed:      finishedAt,
		}
		if reason >= 0 {
			unsigned.Reason = reason
		}
		signed, err := signing.SignExitFailed(ctx, endpoint.signer, unsigned)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		message = &pb.SatelliteMessage{Message: &pb.SatelliteMessage_ExitFailed{
			ExitFailed: signed,
		}}
	}

	return message, nil
}

func (endpoint *Endpoint) updatePointer(ctx context.Context, originalPointer *pb.Pointer, exitingNodeID storj.NodeID, receivingNodeID storj.NodeID, path string, pieceNum int32, originalRootPieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	// remove the node from the pointer
	pointer, err := endpoint.getValidPointer(ctx, path, pieceNum, originalRootPieceID)
	if err != nil {
		return Error.Wrap(err)
	}
	remote := pointer.GetRemote()
	// nothing to do here
	if remote == nil {
		return nil
	}

	pieceMap := make(map[storj.NodeID]*pb.RemotePiece)
	for _, piece := range remote.GetRemotePieces() {
		pieceMap[piece.NodeId] = piece
	}

	var toRemove []*pb.RemotePiece
	existingPiece, ok := pieceMap[exitingNodeID]
	if !ok {
		return Error.New("node no longer has the piece. Node ID: %s", exitingNodeID.String())
	}
	if existingPiece != nil && existingPiece.PieceNum != pieceNum {
		return Error.New("invalid existing piece info. Exiting Node ID: %s, PieceNum: %d", exitingNodeID.String(), pieceNum)
	}
	toRemove = []*pb.RemotePiece{existingPiece}
	delete(pieceMap, exitingNodeID)

	var toAdd []*pb.RemotePiece
	if !receivingNodeID.IsZero() {
		toAdd = []*pb.RemotePiece{{
			PieceNum: pieceNum,
			NodeId:   receivingNodeID,
		}}
	}
	_, err = endpoint.metainfo.UpdatePiecesCheckDuplicates(ctx, path, originalPointer, toAdd, toRemove, true)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// checkExitStatus returns a satellite message based on a node current graceful exit status
// if a node hasn't started graceful exit, it will initialize the process
// if a node has finished graceful exit, it will return a finished message
// if a node has started graceful exit, but no transfer item is available yet, it will return an not ready message
// otherwise, the returned message will be nil
func (endpoint *Endpoint) checkExitStatus(ctx context.Context, nodeID storj.NodeID) (*pb.SatelliteMessage, error) {
	exitStatus, err := endpoint.overlaydb.GetExitStatus(ctx, nodeID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if exitStatus.ExitFinishedAt != nil {
		// TODO maybe we should store the reason in the DB so we know how it originally failed.
		return endpoint.getFinishedMessage(ctx, nodeID, *exitStatus.ExitFinishedAt, exitStatus.ExitSuccess, -1)
	}

	if exitStatus.ExitInitiatedAt == nil {
		request := &overlay.ExitStatusRequest{NodeID: nodeID, ExitInitiatedAt: time.Now().UTC()}
		node, err := endpoint.overlaydb.UpdateExitStatus(ctx, request)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 0)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// graceful exit initiation metrics
		age := time.Now().UTC().Sub(node.CreatedAt.UTC())
		mon.FloatVal("graceful_exit_init_node_age_seconds").Observe(age.Seconds())                           //locked
		mon.IntVal("graceful_exit_init_node_audit_success_count").Observe(node.Reputation.AuditSuccessCount) //locked
		mon.IntVal("graceful_exit_init_node_audit_total_count").Observe(node.Reputation.AuditCount)          //locked
		mon.IntVal("graceful_exit_init_node_piece_count").Observe(node.PieceCount)                           //locked

		return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}}, nil
	}

	if exitStatus.ExitLoopCompletedAt == nil {
		return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}}, nil
	}

	return nil, nil
}

func (endpoint *Endpoint) generateExitStatusRequest(ctx context.Context, nodeID storj.NodeID) (*overlay.ExitStatusRequest, pb.ExitFailed_Reason, error) {
	var exitFailedReason pb.ExitFailed_Reason = -1
	progress, err := endpoint.db.GetProgress(ctx, nodeID)
	if err != nil {
		return nil, exitFailedReason, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	mon.IntVal("graceful_exit_final_pieces_failed").Observe(progress.PiecesFailed)         //locked
	mon.IntVal("graceful_exit_final_pieces_succeess").Observe(progress.PiecesTransferred)  //locked
	mon.IntVal("graceful_exit_final_bytes_transferred").Observe(progress.BytesTransferred) //locked
	processed := progress.PiecesFailed + progress.PiecesTransferred

	if processed > 0 {
		mon.IntVal("graceful_exit_successful_pieces_transfer_ratio").Observe(progress.PiecesTransferred / processed) //locked
	}

	exitStatusRequest := &overlay.ExitStatusRequest{
		NodeID:         progress.NodeID,
		ExitFinishedAt: time.Now().UTC(),
	}
	// check node's exiting progress to see if it has failed passed max failure threshold
	if processed > 0 && float64(progress.PiecesFailed)/float64(processed)*100 >= float64(endpoint.config.OverallMaxFailuresPercentage) {
		exitStatusRequest.ExitSuccess = false
		exitFailedReason = pb.ExitFailed_OVERALL_FAILURE_PERCENTAGE_EXCEEDED
	} else {
		exitStatusRequest.ExitSuccess = true
	}

	if exitStatusRequest.ExitSuccess {
		mon.Meter("graceful_exit_success").Mark(1) //locked
	} else {
		mon.Meter("graceful_exit_fail_max_failures_percentage").Mark(1) //locked
	}

	return exitStatusRequest, exitFailedReason, nil

}

func (endpoint *Endpoint) calculatePieceSize(ctx context.Context, pointer *pb.Pointer, incomplete *TransferQueueItem, nodePiece *pb.RemotePiece) (int64, error) {
	nodeID := incomplete.NodeID

	// calculate piece size
	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return 0, Error.Wrap(err)
	}

	pieces := pointer.GetRemote().GetRemotePieces()
	if len(pieces) > redundancy.OptimalThreshold() {
		endpoint.log.Debug("pointer has more pieces than required. removing node from pointer.", zap.Stringer("node ID", nodeID), zap.ByteString("path", incomplete.Path), zap.Int32("piece num", incomplete.PieceNum))

		return 0, ErrAboveOptimalThreshold.New("")
	}

	return eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy), nil
}

func (endpoint *Endpoint) getValidPointer(ctx context.Context, path string, pieceNum int32, originalRootPieceID storj.PieceID) (*pb.Pointer, error) {
	pointer, err := endpoint.metainfo.Get(ctx, path)
	// TODO we don't know the type of error
	if err != nil {
		return nil, Error.New("pointer path %v no longer exists.", path)
	}

	remote := pointer.GetRemote()
	// no longer a remote segment
	if remote == nil {
		return nil, Error.New("pointer path %v is no longer remote.", path)
	}

	if !originalRootPieceID.IsZero() && originalRootPieceID != remote.RootPieceId {
		return nil, Error.New("pointer path %v has changed.", path)
	}
	return pointer, nil
}

func (endpoint *Endpoint) getNodePiece(ctx context.Context, pointer *pb.Pointer, incomplete *TransferQueueItem) (*pb.RemotePiece, error) {
	remote := pointer.GetRemote()
	nodeID := incomplete.NodeID

	pieces := remote.GetRemotePieces()
	var nodePiece *pb.RemotePiece
	for _, piece := range pieces {
		if piece.NodeId == nodeID && piece.PieceNum == incomplete.PieceNum {
			nodePiece = piece
		}
	}

	if nodePiece == nil {
		endpoint.log.Debug("piece no longer held by node", zap.Stringer("node ID", nodeID), zap.ByteString("path", incomplete.Path), zap.Int32("piece num", incomplete.PieceNum))
		return nil, Error.New("piece no longer held by node")
	}

	return nodePiece, nil
}
