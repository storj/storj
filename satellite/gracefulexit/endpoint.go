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

	"storj.io/common/context2"
	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

// millis for the transfer queue building ticker.
const buildQueueMillis = 100

var (
	// ErrInvalidArgument is an error class for invalid argument errors used to check which rpc code to use.
	ErrInvalidArgument = errs.Class("graceful exit")
	// ErrIneligibleNodeAge is an error class for when a node has not been on the network long enough to graceful exit.
	ErrIneligibleNodeAge = errs.Class("node is not yet eligible for graceful exit")
)

// Endpoint for handling the transfer of pieces for Graceful Exit.
type Endpoint struct {
	pb.DRPCSatelliteGracefulExitUnimplementedServer

	log            *zap.Logger
	interval       time.Duration
	signer         signing.Signer
	db             DB
	overlaydb      overlay.DB
	overlay        *overlay.Service
	reputation     *reputation.Service
	metabase       *metabase.DB
	orders         *orders.Service
	connections    *connectionsTracker
	peerIdentities overlay.PeerIdentities
	config         Config
	recvTimeout    time.Duration

	nowFunc func() time.Time
}

// connectionsTracker for tracking ongoing connections on this api server.
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

// NewEndpoint creates a new graceful exit endpoint.
func NewEndpoint(log *zap.Logger, signer signing.Signer, db DB, overlaydb overlay.DB, overlay *overlay.Service, reputation *reputation.Service, metabase *metabase.DB, orders *orders.Service,
	peerIdentities overlay.PeerIdentities, config Config) *Endpoint {
	return &Endpoint{
		log:            log,
		interval:       time.Millisecond * buildQueueMillis,
		signer:         signer,
		db:             db,
		overlaydb:      overlaydb,
		overlay:        overlay,
		reputation:     reputation,
		metabase:       metabase,
		orders:         orders,
		connections:    newConnectionsTracker(),
		peerIdentities: peerIdentities,
		config:         config,
		recvTimeout:    config.RecvTimeout,
		nowFunc:        func() time.Time { return time.Now().UTC() },
	}
}

// SetNowFunc applies a function to be used in determining the "now" time for graceful exit
// purposes.
func (endpoint *Endpoint) SetNowFunc(timeFunc func() time.Time) {
	endpoint.nowFunc = timeFunc
}

// Process is called by storage nodes to receive pieces to transfer to new nodes and get exit status.
func (endpoint *Endpoint) Process(stream pb.DRPCSatelliteGracefulExit_ProcessStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("graceful exit process", zap.Stringer("Node ID", peer.ID))

	if endpoint.config.TimeBased {
		return endpoint.processTimeBased(ctx, stream, peer.ID)
	}
	return endpoint.processPiecewise(ctx, stream, peer.ID)
}

func (endpoint *Endpoint) processTimeBased(ctx context.Context, stream pb.DRPCSatelliteGracefulExit_ProcessStream, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	nodeInfo, err := endpoint.overlay.Get(ctx, nodeID)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	isDisqualified, err := endpoint.handleDisqualifiedNodeTimeBased(ctx, nodeInfo)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	if isDisqualified {
		return rpcstatus.Error(rpcstatus.FailedPrecondition, "node is disqualified")
	}
	if endpoint.handleSuspendedNodeTimeBased(nodeInfo) {
		return rpcstatus.Error(rpcstatus.FailedPrecondition, "node is suspended. Please get node unsuspended before initiating graceful exit")
	}

	msg, err := endpoint.checkExitStatusTimeBased(ctx, nodeInfo)
	if err != nil {
		if ErrIneligibleNodeAge.Has(err) {
			return rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
		}
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	err = stream.Send(msg)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return nil
}

// process is called by storage nodes to receive pieces to transfer to new nodes and get exit status.
func (endpoint *Endpoint) processPiecewise(ctx context.Context, stream pb.DRPCSatelliteGracefulExit_ProcessStream, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

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
		if ErrIneligibleNodeAge.Has(err) {
			return rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
		}
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
	defer func() {
		err2 := errs2.IgnoreCanceled(group.Wait())
		if err2 != nil {
			endpoint.log.Error("incompleteLoop gave error", zap.Error(err2))
		}
	}()

	// we cancel this context in all situations where we want to exit the loop
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var geSuccess bool
	var geSuccessMutex sync.Mutex

	group.Go(func() error {
		incompleteLoop := sync2.NewCycle(endpoint.interval)

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
					geSuccessMutex.Lock()
					geSuccess = true
					geSuccessMutex.Unlock()
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
		err = errs2.IgnoreCanceled(err)
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		// if there is no more work to receive send complete
		if finished {
			// This point is reached both when an exit is entirely successful and
			// when the satellite is being shut down. geSuccess
			// differentiates these cases.
			geSuccessMutex.Lock()
			wasSuccessful := geSuccess
			geSuccessMutex.Unlock()

			if !wasSuccessful {
				return rpcstatus.Error(rpcstatus.Canceled, "graceful exit processing interrupted (node should reconnect and continue)")
			}

			// ignore cancelled context which was triggered to finish loop but we still need to do some DB operations
			ctx = context2.WithoutCancellation(ctx)

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
					endpoint.log.Warn("node already exists in segment.", zap.Error(err))

					continue
				}
				if ErrInvalidArgument.Has(err) {
					messageBytes, marshalErr := pb.Marshal(request)
					if marshalErr != nil {
						return rpcstatus.Error(rpcstatus.Internal, marshalErr.Error())
					}
					endpoint.log.Warn("storagenode failed validation for piece transfer", zap.Stringer("node ID", nodeID), zap.Binary("original message from storagenode", messageBytes), zap.Error(err))

					// immediately fail and complete graceful exit for nodes that fail satellite validation
					err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
					if err != nil {
						return rpcstatus.Error(rpcstatus.Internal, err.Error())
					}

					mon.Meter("graceful_exit_fail_validation").Mark(1) //mon:locked

					exitStatusRequest := &overlay.ExitStatusRequest{
						NodeID:         nodeID,
						ExitFinishedAt: endpoint.nowFunc(),
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

	return nil
}

func (endpoint *Endpoint) processIncomplete(ctx context.Context, stream pb.DRPCSatelliteGracefulExit_ProcessStream, pending *PendingMap, incomplete *TransferQueueItem) error {
	nodeID := incomplete.NodeID

	if incomplete.OrderLimitSendCount >= endpoint.config.MaxOrderLimitSendCount {
		err := endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
		if err != nil {
			return Error.Wrap(err)
		}
		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.StreamID, incomplete.Position, incomplete.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	}

	segment, err := endpoint.getValidSegment(ctx, incomplete.StreamID, incomplete.Position, incomplete.RootPieceID)
	if err != nil {
		endpoint.log.Warn("invalid segment", zap.Error(err))
		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.StreamID, incomplete.Position, incomplete.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	}

	nodePiece, err := endpoint.getNodePiece(ctx, segment, incomplete)
	if err != nil {
		deleteErr := endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.StreamID, incomplete.Position, incomplete.PieceNum)
		if deleteErr != nil {
			return Error.Wrap(deleteErr)
		}
		return Error.Wrap(err)
	}

	pieceSize, err := endpoint.calculatePieceSize(ctx, segment, incomplete)
	if ErrAboveOptimalThreshold.Has(err) {
		err = endpoint.UpdatePiecesCheckDuplicates(ctx, segment, metabase.Pieces{}, metabase.Pieces{nodePiece}, false)
		if err != nil {
			return Error.Wrap(err)
		}

		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.StreamID, incomplete.Position, incomplete.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}
		return nil
	}
	if err != nil {
		return Error.Wrap(err)
	}

	// populate excluded node IDs
	pieces := segment.Pieces
	excludedIDs := make([]storj.NodeID, len(pieces))
	for i, piece := range pieces {
		excludedIDs[i] = piece.StorageNode
	}

	// get replacement node
	request := &overlay.FindStorageNodesRequest{
		RequestedCount: 1,
		ExcludedIDs:    excludedIDs,
		Placement:      segment.Placement,
	}

	newNodes, err := endpoint.overlay.FindStorageNodesForGracefulExit(ctx, *request)
	if err != nil {
		return Error.Wrap(err)
	}

	if len(newNodes) == 0 {
		return Error.New("could not find a node to receive piece transfer: node ID %v, stream_id %v, piece num %v", nodeID, incomplete.StreamID, incomplete.PieceNum)
	}

	newNode := newNodes[0]
	endpoint.log.Debug("found new node for piece transfer", zap.Stringer("original node ID", nodeID), zap.Stringer("replacement node ID", newNode.ID),
		zap.ByteString("streamID", incomplete.StreamID[:]), zap.Uint32("Part", incomplete.Position.Part), zap.Uint32("Index", incomplete.Position.Index),
		zap.Int32("piece num", incomplete.PieceNum))

	pieceID := segment.RootPieceID.Derive(nodeID, incomplete.PieceNum)

	limit, privateKey, err := endpoint.orders.CreateGracefulExitPutOrderLimit(ctx, metabase.BucketLocation{}, newNode.ID, incomplete.PieceNum, segment.RootPieceID, int32(pieceSize))
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

	err = endpoint.db.IncrementOrderLimitSendCount(ctx, nodeID, incomplete.StreamID, incomplete.Position, incomplete.PieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	// update pending queue with the transfer item
	err = pending.Put(pieceID, &PendingTransfer{
		StreamID:            incomplete.StreamID,
		Position:            incomplete.Position,
		PieceSize:           pieceSize,
		SatelliteMessage:    transferMsg,
		OriginalRootPieceID: segment.RootPieceID,
		PieceNum:            uint16(incomplete.PieceNum), // TODO
	})

	return err
}

func (endpoint *Endpoint) handleSucceeded(ctx context.Context, stream pb.DRPCSatelliteGracefulExit_ProcessStream, pending *PendingMap, exitingNodeID storj.NodeID, message *pb.StorageNodeMessage_Succeeded) (err error) {
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
	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, exitingNodeID, transfer.StreamID, transfer.Position, int32(transfer.PieceNum))
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.updateSegment(ctx, exitingNodeID, receivingNodeID, transfer.StreamID, transfer.Position, transfer.PieceNum, transferQueueItem.RootPieceID)
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

	err = endpoint.db.DeleteTransferQueueItem(ctx, exitingNodeID, transfer.StreamID, transfer.Position, int32(transfer.PieceNum))
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

	mon.Meter("graceful_exit_transfer_piece_success").Mark(1) //mon:locked
	return nil
}

func (endpoint *Endpoint) handleFailed(ctx context.Context, pending *PendingMap, nodeID storj.NodeID, message *pb.StorageNodeMessage_Failed) (err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.log.Warn("transfer failed",
		zap.Stringer("Piece ID", message.Failed.OriginalPieceId),
		zap.Stringer("nodeID", nodeID),
		zap.Stringer("transfer error", message.Failed.GetError()),
	)
	mon.Meter("graceful_exit_transfer_piece_fail").Mark(1) //mon:locked

	pieceID := message.Failed.OriginalPieceId
	transfer, ok := pending.Get(pieceID)
	if !ok {
		endpoint.log.Warn("could not find transfer message in pending queue. skipping.", zap.Stringer("Piece ID", pieceID), zap.Stringer("Node ID", nodeID))

		// this should be rare and it is not likely this is someone trying to do something malicious since it is a "failure"
		return nil
	}

	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, nodeID, transfer.StreamID, transfer.Position, int32(transfer.PieceNum))
	if err != nil {
		return Error.Wrap(err)
	}
	now := endpoint.nowFunc()
	failedCount := 1
	if transferQueueItem.FailedCount != nil {
		failedCount = *transferQueueItem.FailedCount + 1
	}

	errorCode := int(pb.TransferFailed_Error_value[message.Failed.Error.String()])

	// If the error code is NOT_FOUND, the node no longer has the piece.
	// Remove the queue item and remove the node from the pointer.
	// If the pointer is not piece hash verified, do not count this as a failure.
	if pb.TransferFailed_Error(errorCode) == pb.TransferFailed_NOT_FOUND {
		endpoint.log.Debug("piece not found on node", zap.Stringer("node ID", nodeID),
			zap.ByteString("streamID", transfer.StreamID[:]), zap.Uint32("Part", transfer.Position.Part), zap.Uint32("Index", transfer.Position.Index),
			zap.Uint16("piece num", transfer.PieceNum))

		segment, err := endpoint.getValidSegment(ctx, transfer.StreamID, transfer.Position, storj.PieceID{})
		if err != nil {
			return Error.Wrap(err)
		}

		pieces := segment.Pieces
		var nodePiece metabase.Piece
		for _, piece := range pieces {
			if piece.StorageNode == nodeID && piece.Number == transfer.PieceNum {
				nodePiece = piece
			}
		}
		if nodePiece == (metabase.Piece{}) {
			err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, transfer.StreamID, transfer.Position, int32(transfer.PieceNum))
			if err != nil {
				return Error.Wrap(err)
			}
			return pending.Delete(pieceID)
		}

		err = endpoint.UpdatePiecesCheckDuplicates(ctx, segment, metabase.Pieces{}, metabase.Pieces{nodePiece}, false)
		if err != nil {
			return Error.Wrap(err)
		}

		err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
		if err != nil {
			return Error.Wrap(err)
		}

		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, transfer.StreamID, transfer.Position, int32(transfer.PieceNum))
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
			ExitFinishedAt: endpoint.nowFunc(),
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

func (endpoint *Endpoint) handleDisqualifiedNodeTimeBased(ctx context.Context, nodeInfo *overlay.NodeDossier) (isDisqualified bool, err error) {
	if nodeInfo.Disqualified != nil {
		if nodeInfo.ExitStatus.ExitInitiatedAt == nil {
			// node never started graceful exit before, and it is already disqualified; nothing
			// for us to do here
			return true, nil
		}
		if nodeInfo.ExitStatus.ExitFinishedAt == nil {
			// node did start graceful exit and hasn't been marked as finished, although it
			// has been disqualified. We'll correct that now.
			exitStatusRequest := &overlay.ExitStatusRequest{
				NodeID:         nodeInfo.Id,
				ExitFinishedAt: endpoint.nowFunc(),
				ExitSuccess:    false,
			}

			_, err = endpoint.overlaydb.UpdateExitStatus(ctx, exitStatusRequest)
			return true, Error.Wrap(err)
		}
		return true, nil
	}
	return false, nil
}

func (endpoint *Endpoint) handleSuspendedNodeTimeBased(nodeInfo *overlay.NodeDossier) (isSuspended bool) {
	if nodeInfo.UnknownAuditSuspended != nil || nodeInfo.OfflineSuspended != nil {
		// If the node already initiated graceful exit, we'll let it carry on until / unless it gets disqualified.
		// Otherwise, the operator should make an effort to get the node un-suspended before initiating GE.
		// (The all-wise Go linter won't let me write this in a clearer way.)
		return nodeInfo.ExitStatus.ExitInitiatedAt == nil
	}
	return false
}

func (endpoint *Endpoint) handleFinished(ctx context.Context, stream pb.DRPCSatelliteGracefulExit_ProcessStream, exitStatusRequest *overlay.ExitStatusRequest, failedReason pb.ExitFailed_Reason) error {
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
		return endpoint.getFinishedSuccessMessage(ctx, nodeID, finishedAt)
	}
	return endpoint.getFinishedFailureMessage(ctx, nodeID, finishedAt, reason)
}

func (endpoint *Endpoint) getFinishedSuccessMessage(ctx context.Context, nodeID storj.NodeID, finishedAt time.Time) (message *pb.SatelliteMessage, err error) {
	unsigned := &pb.ExitCompleted{
		SatelliteId: endpoint.signer.ID(),
		NodeId:      nodeID,
		Completed:   finishedAt,
	}
	signed, err := signing.SignExitCompleted(ctx, endpoint.signer, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_ExitCompleted{
		ExitCompleted: signed,
	}}, nil
}

func (endpoint *Endpoint) getFinishedFailureMessage(ctx context.Context, nodeID storj.NodeID, finishedAt time.Time, reason pb.ExitFailed_Reason) (message *pb.SatelliteMessage, err error) {
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
	if !endpoint.config.TimeBased {
		err = endpoint.overlay.DisqualifyNode(ctx, nodeID, overlay.DisqualificationReasonUnknown)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}
	return message, nil
}

func (endpoint *Endpoint) updateSegment(ctx context.Context, exitingNodeID storj.NodeID, receivingNodeID storj.NodeID, streamID uuid.UUID, position metabase.SegmentPosition, pieceNumber uint16, originalRootPieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	// remove the node from the segment
	segment, err := endpoint.getValidSegment(ctx, streamID, position, originalRootPieceID)
	if err != nil {
		return Error.Wrap(err)
	}

	pieceMap := make(map[storj.NodeID]metabase.Piece)
	for _, piece := range segment.Pieces {
		pieceMap[piece.StorageNode] = piece
	}

	var toRemove metabase.Pieces
	existingPiece, ok := pieceMap[exitingNodeID]
	if !ok {
		return Error.New("node no longer has the piece. Node ID: %s", exitingNodeID.String())
	}
	if existingPiece != (metabase.Piece{}) && existingPiece.Number != pieceNumber {
		return Error.New("invalid existing piece info. Exiting Node ID: %s, PieceNum: %d", exitingNodeID.String(), pieceNumber)
	}
	toRemove = metabase.Pieces{existingPiece}
	delete(pieceMap, exitingNodeID)

	var toAdd metabase.Pieces
	if !receivingNodeID.IsZero() {
		toAdd = metabase.Pieces{{
			Number:      pieceNumber,
			StorageNode: receivingNodeID,
		}}
	}

	err = endpoint.UpdatePiecesCheckDuplicates(ctx, segment, toAdd, toRemove, true)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// checkExitStatus returns a satellite message based on a node current graceful exit status
// if a node hasn't started graceful exit, it will initialize the process
// if a node has finished graceful exit, it will return a finished message
// if a node has started graceful exit, but no transfer item is available yet, it will return an not ready message
// otherwise, the returned message will be nil.
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
		nodeDossier, err := endpoint.overlaydb.Get(ctx, nodeID)
		if err != nil {
			endpoint.log.Error("unable to retrieve node dossier for attempted exiting node", zap.Stringer("node ID", nodeID))
			return nil, Error.Wrap(err)
		}
		geEligibilityDate := nodeDossier.CreatedAt.AddDate(0, endpoint.config.NodeMinAgeInMonths, 0)
		if endpoint.nowFunc().Before(geEligibilityDate) {
			return nil, ErrIneligibleNodeAge.New("will be eligible after %s", geEligibilityDate.String())
		}

		request := &overlay.ExitStatusRequest{NodeID: nodeID, ExitInitiatedAt: endpoint.nowFunc()}
		node, err := endpoint.overlaydb.UpdateExitStatus(ctx, request)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 0)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		reputationInfo, err := endpoint.reputation.Get(ctx, nodeID)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// graceful exit initiation metrics
		age := endpoint.nowFunc().Sub(node.CreatedAt.UTC())
		mon.FloatVal("graceful_exit_init_node_age_seconds").Observe(age.Seconds())                          //mon:locked
		mon.IntVal("graceful_exit_init_node_audit_success_count").Observe(reputationInfo.AuditSuccessCount) //mon:locked
		mon.IntVal("graceful_exit_init_node_audit_total_count").Observe(reputationInfo.TotalAuditCount)     //mon:locked
		mon.IntVal("graceful_exit_init_node_piece_count").Observe(node.PieceCount)                          //mon:locked

		return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}}, nil
	}

	if exitStatus.ExitLoopCompletedAt == nil {
		return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}}, nil
	}

	return nil, nil
}

func (endpoint *Endpoint) checkExitStatusTimeBased(ctx context.Context, nodeInfo *overlay.NodeDossier) (*pb.SatelliteMessage, error) {
	if nodeInfo.ExitStatus.ExitFinishedAt != nil {
		// TODO maybe we should store the reason in the DB so we know how it originally failed.
		return endpoint.getFinishedMessage(ctx, nodeInfo.Id, *nodeInfo.ExitStatus.ExitFinishedAt, nodeInfo.ExitStatus.ExitSuccess, -1)
	}

	if nodeInfo.ExitStatus.ExitInitiatedAt == nil {
		// the node has just requested to begin GE. verify eligibility and set it up in the DB.
		geEligibilityDate := nodeInfo.CreatedAt.AddDate(0, endpoint.config.NodeMinAgeInMonths, 0)
		if endpoint.nowFunc().Before(geEligibilityDate) {
			return nil, ErrIneligibleNodeAge.New("will be eligible after %s", geEligibilityDate.String())
		}

		request := &overlay.ExitStatusRequest{
			NodeID:          nodeInfo.Id,
			ExitInitiatedAt: endpoint.nowFunc(),
		}
		node, err := endpoint.overlaydb.UpdateExitStatus(ctx, request)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		reputationInfo, err := endpoint.reputation.Get(ctx, nodeInfo.Id)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// graceful exit initiation metrics
		age := endpoint.nowFunc().Sub(node.CreatedAt)
		mon.FloatVal("graceful_exit_init_node_age_seconds").Observe(age.Seconds())                          //mon:locked
		mon.IntVal("graceful_exit_init_node_audit_success_count").Observe(reputationInfo.AuditSuccessCount) //mon:locked
		mon.IntVal("graceful_exit_init_node_audit_total_count").Observe(reputationInfo.TotalAuditCount)     //mon:locked
		mon.IntVal("graceful_exit_init_node_piece_count").Observe(node.PieceCount)                          //mon:locked
	} else {
		// the node has already initiated GE and hasn't finished yet... or has it?!?!
		geDoneDate := nodeInfo.ExitStatus.ExitInitiatedAt.AddDate(0, 0, endpoint.config.GracefulExitDurationInDays)
		if endpoint.nowFunc().After(geDoneDate) {
			// ok actually it has finished, and this is the first time we've noticed it
			reputationInfo, err := endpoint.reputation.Get(ctx, nodeInfo.Id)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			request := &overlay.ExitStatusRequest{
				NodeID:         nodeInfo.Id,
				ExitFinishedAt: endpoint.nowFunc(),
				ExitSuccess:    true,
			}

			// We don't check the online score constantly over the course of the graceful exit,
			// because we want to give the node a chance to get the score back up if it's
			// temporarily low.
			//
			// Instead, we check the overall score at the end of the GE period.
			if reputationInfo.OnlineScore < endpoint.config.MinimumOnlineScore {
				request.ExitSuccess = false
			}
			endpoint.log.Info("node completed graceful exit",
				zap.Float64("online score", reputationInfo.OnlineScore),
				zap.Bool("success", request.ExitSuccess),
				zap.Stringer("node ID", nodeInfo.Id))
			updatedNode, err := endpoint.overlaydb.UpdateExitStatus(ctx, request)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			if request.ExitSuccess {
				mon.Meter("graceful_exit_success").Mark(1) //mon:locked
				return endpoint.getFinishedSuccessMessage(ctx, updatedNode.Id, *updatedNode.ExitStatus.ExitFinishedAt)
			}
			mon.Meter("graceful_exit_failure").Mark(1)
			return endpoint.getFinishedFailureMessage(ctx, updatedNode.Id, *updatedNode.ExitStatus.ExitFinishedAt, pb.ExitFailed_INACTIVE_TIMEFRAME_EXCEEDED)
		}
	}

	// this will cause the node to disconnect, wait a bit, and then try asking again.
	return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}}, nil
}

func (endpoint *Endpoint) generateExitStatusRequest(ctx context.Context, nodeID storj.NodeID) (*overlay.ExitStatusRequest, pb.ExitFailed_Reason, error) {
	var exitFailedReason pb.ExitFailed_Reason = -1
	progress, err := endpoint.db.GetProgress(ctx, nodeID)
	if err != nil {
		return nil, exitFailedReason, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	mon.IntVal("graceful_exit_final_pieces_failed").Observe(progress.PiecesFailed)         //mon:locked
	mon.IntVal("graceful_exit_final_pieces_succeess").Observe(progress.PiecesTransferred)  //mon:locked
	mon.IntVal("graceful_exit_final_bytes_transferred").Observe(progress.BytesTransferred) //mon:locked
	processed := progress.PiecesFailed + progress.PiecesTransferred

	if processed > 0 {
		mon.IntVal("graceful_exit_successful_pieces_transfer_ratio").Observe(progress.PiecesTransferred / processed) //mon:locked
	}

	exitStatusRequest := &overlay.ExitStatusRequest{
		NodeID:         progress.NodeID,
		ExitFinishedAt: endpoint.nowFunc(),
	}
	// check node's exiting progress to see if it has failed passed max failure threshold
	if processed > 0 && float64(progress.PiecesFailed)/float64(processed)*100 >= float64(endpoint.config.OverallMaxFailuresPercentage) {
		exitStatusRequest.ExitSuccess = false
		exitFailedReason = pb.ExitFailed_OVERALL_FAILURE_PERCENTAGE_EXCEEDED
	} else {
		exitStatusRequest.ExitSuccess = true
	}

	if exitStatusRequest.ExitSuccess {
		mon.Meter("graceful_exit_success").Mark(1) //mon:locked
	} else {
		mon.Meter("graceful_exit_fail_max_failures_percentage").Mark(1) //mon:locked
	}

	return exitStatusRequest, exitFailedReason, nil
}

func (endpoint *Endpoint) calculatePieceSize(ctx context.Context, segment metabase.Segment, incomplete *TransferQueueItem) (int64, error) {
	nodeID := incomplete.NodeID

	if len(segment.Pieces) > int(segment.Redundancy.OptimalShares) {
		endpoint.log.Debug("segment has more pieces than required. removing node from segment.", zap.Stringer("node ID", nodeID), zap.Int32("piece num", incomplete.PieceNum))

		return 0, ErrAboveOptimalThreshold.New("")
	}

	return segment.PieceSize(), nil
}

func (endpoint *Endpoint) getValidSegment(ctx context.Context, streamID uuid.UUID, position metabase.SegmentPosition, originalRootPieceID storj.PieceID) (metabase.Segment, error) {
	segment, err := endpoint.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: streamID,
		Position: position,
	})
	if err != nil {
		return metabase.Segment{}, Error.Wrap(err)
	}

	if !originalRootPieceID.IsZero() && originalRootPieceID != segment.RootPieceID {
		return metabase.Segment{}, Error.New("segment has changed")
	}
	return segment, nil
}

func (endpoint *Endpoint) getNodePiece(ctx context.Context, segment metabase.Segment, incomplete *TransferQueueItem) (metabase.Piece, error) {
	nodeID := incomplete.NodeID

	var nodePiece metabase.Piece
	for _, piece := range segment.Pieces {
		if piece.StorageNode == nodeID && int32(piece.Number) == incomplete.PieceNum {
			nodePiece = piece
		}
	}

	if nodePiece == (metabase.Piece{}) {
		endpoint.log.Debug("piece no longer held by node", zap.Stringer("node ID", nodeID), zap.Int32("piece num", incomplete.PieceNum))
		return metabase.Piece{}, Error.New("piece no longer held by node")
	}

	return nodePiece, nil
}

// GracefulExitFeasibility returns node's joined at date, nodeMinAge and if graceful exit available.
func (endpoint *Endpoint) GracefulExitFeasibility(ctx context.Context, req *pb.GracefulExitFeasibilityRequest) (_ *pb.GracefulExitFeasibilityResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("graceful exit process", zap.Stringer("Node ID", peer.ID))

	var response pb.GracefulExitFeasibilityResponse

	nodeDossier, err := endpoint.overlay.Get(ctx, peer.ID)
	if err != nil {
		endpoint.log.Error("unable to retrieve node dossier for attempted exiting node", zap.Stringer("node ID", peer.ID))
		return nil, Error.Wrap(err)
	}

	eligibilityDate := nodeDossier.CreatedAt.AddDate(0, endpoint.config.NodeMinAgeInMonths, 0)
	if endpoint.nowFunc().Before(eligibilityDate) {
		response.IsAllowed = false
	} else {
		response.IsAllowed = true
	}

	response.JoinedAt = nodeDossier.CreatedAt
	response.MonthsRequired = int32(endpoint.config.NodeMinAgeInMonths)
	return &response, nil
}

// UpdatePiecesCheckDuplicates atomically adds toAdd pieces and removes toRemove pieces from
// the segment.
//
// If checkDuplicates is true it will return an error if the nodes to be
// added are already in the segment.
// Then it will remove the toRemove pieces and then it will add the toAdd pieces.
func (endpoint *Endpoint) UpdatePiecesCheckDuplicates(ctx context.Context, segment metabase.Segment, toAdd, toRemove metabase.Pieces, checkDuplicates bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Return an error if the segment already has a piece for this node
	if checkDuplicates {
		// put all existing pieces to a map
		nodePieceMap := make(map[storj.NodeID]struct{})
		for _, piece := range segment.Pieces {
			nodePieceMap[piece.StorageNode] = struct{}{}
		}

		for _, piece := range toAdd {
			_, ok := nodePieceMap[piece.StorageNode]
			if ok {
				return metainfo.ErrNodeAlreadyExists.New("node id already exists in segment. StreamID: %s, Position: %d, NodeID: %s", segment.StreamID, segment.Position, piece.StorageNode.String())
			}
			nodePieceMap[piece.StorageNode] = struct{}{}
		}
	}

	pieces, err := segment.Pieces.Update(toAdd, toRemove)
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID: segment.StreamID,
		Position: segment.Position,

		OldPieces:     segment.Pieces,
		NewRedundancy: segment.Redundancy,
		NewPieces:     pieces,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			err = metabase.ErrObjectNotFound.Wrap(err)
		}
		return Error.Wrap(err)
	}

	return nil
}
