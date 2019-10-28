// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"bytes"
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
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
	peerIdentities overlay.PeerIdentities
	config         Config
}

type pendingTransfer struct {
	path             []byte
	pieceSize        int64
	satelliteMessage *pb.SatelliteMessage
	rootPieceID      storj.PieceID
	pieceNum         int32
}

// pendingMap for managing concurrent access to the pending transfer map.
type pendingMap struct {
	mu   sync.RWMutex
	data map[storj.PieceID]*pendingTransfer
}

// newPendingMap creates a new pendingMap and instantiates the map.
func newPendingMap() *pendingMap {
	newData := make(map[storj.PieceID]*pendingTransfer)
	return &pendingMap{
		data: newData,
	}
}

// put adds to the map.
func (pm *pendingMap) put(pieceID storj.PieceID, pendingTransfer *pendingTransfer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.data[pieceID] = pendingTransfer
}

// get returns the pending transfer item from the map, if it exists.
func (pm *pendingMap) get(pieceID storj.PieceID) (pendingTransfer *pendingTransfer, ok bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pendingTransfer, ok = pm.data[pieceID]
	return pendingTransfer, ok
}

// length returns the number of elements in the map.
func (pm *pendingMap) length() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.data)
}

// delete removes the pending transfer item from the map.
func (pm *pendingMap) delete(pieceID storj.PieceID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.data, pieceID)
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
		peerIdentities: peerIdentities,
		config:         config,
	}
}

// Process is called by storage nodes to receive pieces to transfer to new nodes and get exit status.
func (endpoint *Endpoint) Process(stream pb.SatelliteGracefulExit_ProcessServer) error {
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
	// TODO should we error if the node is DQ'd?

	nodeID := peer.ID
	endpoint.log.Debug("graceful exit process", zap.Stringer("node ID", nodeID))

	eofHandler := func(err error) error {
		if err == io.EOF {
			endpoint.log.Debug("received EOF when trying to receive messages from storage node", zap.Stringer("node ID", nodeID))
			return nil
		}
		if err != nil {
			return rpcstatus.Error(rpcstatus.Unknown, Error.Wrap(err).Error())
		}
		return nil
	}

	exitStatus, err := endpoint.overlaydb.GetExitStatus(ctx, nodeID)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
	}

	if exitStatus.ExitFinishedAt != nil {
		// TODO maybe we should store the reason in the DB so we know how it originally failed.
		finishedMsg, err := endpoint.getFinishedMessage(ctx, endpoint.signer, nodeID, *exitStatus.ExitFinishedAt, exitStatus.ExitSuccess, -1)
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, err.Error())
		}

		err = stream.Send(finishedMsg)
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
		}
		return nil
	}

	if exitStatus.ExitInitiatedAt == nil {
		request := &overlay.ExitStatusRequest{NodeID: nodeID, ExitInitiatedAt: time.Now().UTC()}
		_, err = endpoint.overlaydb.UpdateExitStatus(ctx, request)
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
		}
		err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 0)
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
		}
		err = stream.Send(&pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}})
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
		}
		return nil
	}

	if exitStatus.ExitLoopCompletedAt == nil {
		err = stream.Send(&pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}})
		if err != nil {
			return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
		}
		return nil
	}

	pending := newPendingMap()

	var morePiecesFlag int32 = 1
	errChan := make(chan error, 1)
	processChan := make(chan bool, 1)

	handleError := func(err error) error {
		errChan <- err
		close(errChan)
		return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
	}

	var group errgroup.Group
	group.Go(func() error {
		ticker := time.NewTicker(endpoint.interval)
		defer ticker.Stop()
		defer func() {
			processChan <- true
			close(processChan)
		}()

		for range ticker.C {
			// exit if context canceled
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if pending.length() == 0 {
				incomplete, err := endpoint.db.GetIncompleteNotFailed(ctx, nodeID, endpoint.config.EndpointBatchSize, 0)
				if err != nil {
					return handleError(err)
				}

				if len(incomplete) == 0 {
					incomplete, err = endpoint.db.GetIncompleteFailed(ctx, nodeID, endpoint.config.MaxFailuresPerPiece, endpoint.config.EndpointBatchSize, 0)
					if err != nil {
						return handleError(err)
					}
				}

				if len(incomplete) == 0 {
					endpoint.log.Debug("no more pieces to transfer for node", zap.Stringer("node ID", nodeID))
					atomic.StoreInt32(&morePiecesFlag, 0)
					break
				}

				for _, inc := range incomplete {
					err = endpoint.processIncomplete(ctx, stream, pending, inc)
					if err != nil {
						return handleError(err)
					}
				}
				processChan <- true
			}
		}
		return nil
	})

	for {
		select {
		case <-errChan:
			return group.Wait()
		default:
		}

		pendingCount := pending.length()

		// wait if there are none pending
		if pendingCount == 0 {
			select {
			case <-processChan:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// if there are no more transfers and the pending queue is empty, send complete
		if atomic.LoadInt32(&morePiecesFlag) == 0 && pendingCount == 0 {
			exitStatusRequest := &overlay.ExitStatusRequest{
				NodeID:         nodeID,
				ExitFinishedAt: time.Now().UTC(),
			}

			progress, err := endpoint.db.GetProgress(ctx, nodeID)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}

			var transferMsg *pb.SatelliteMessage
			processed := progress.PiecesFailed + progress.PiecesTransferred
			// check node's exiting progress to see if it has failed passed max failure threshold
			if processed > 0 && float64(progress.PiecesFailed)/float64(processed)*100 >= float64(endpoint.config.OverallMaxFailuresPercentage) {

				exitStatusRequest.ExitSuccess = false
				transferMsg, err = endpoint.getFinishedMessage(ctx, endpoint.signer, nodeID, exitStatusRequest.ExitFinishedAt, exitStatusRequest.ExitSuccess, pb.ExitFailed_OVERALL_FAILURE_PERCENTAGE_EXCEEDED)
				if err != nil {
					return rpcstatus.Error(rpcstatus.Internal, err.Error())
				}
			} else {
				exitStatusRequest.ExitSuccess = true
				transferMsg, err = endpoint.getFinishedMessage(ctx, endpoint.signer, nodeID, exitStatusRequest.ExitFinishedAt, exitStatusRequest.ExitSuccess, -1)
				if err != nil {
					return rpcstatus.Error(rpcstatus.Internal, err.Error())
				}
			}

			_, err = endpoint.overlaydb.UpdateExitStatus(ctx, exitStatusRequest)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}

			err = stream.Send(transferMsg)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
			}

			// remove remaining items from the queue after notifying nodes about their exit status
			err = endpoint.db.DeleteTransferQueueItems(ctx, nodeID)
			if err != nil {
				return rpcstatus.Error(rpcstatus.Internal, err.Error())
			}
			break
		}

		request, err := stream.Recv()
		if err != nil {
			return eofHandler(err)
		}

		switch m := request.GetMessage().(type) {
		case *pb.StorageNodeMessage_Succeeded:
			err = endpoint.handleSucceeded(ctx, stream, pending, nodeID, m)
			if err != nil {
				if ErrInvalidArgument.Has(err) {
					// immediately fail and complete graceful exit for nodes that fail satellite validation
					exitStatusRequest := &overlay.ExitStatusRequest{
						NodeID:         nodeID,
						ExitFinishedAt: time.Now().UTC(),
						ExitSuccess:    false,
					}

					finishedMsg, err := endpoint.getFinishedMessage(ctx, endpoint.signer, nodeID, exitStatusRequest.ExitFinishedAt, exitStatusRequest.ExitSuccess, pb.ExitFailed_VERIFICATION_FAILED)
					if err != nil {
						return rpcstatus.Error(rpcstatus.Internal, err.Error())
					}

					_, err = endpoint.overlaydb.UpdateExitStatus(ctx, exitStatusRequest)
					if err != nil {
						return rpcstatus.Error(rpcstatus.Internal, err.Error())
					}

					err = stream.Send(finishedMsg)
					if err != nil {
						return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
					}

					// remove remaining items from the queue after notifying nodes about their exit status
					err = endpoint.db.DeleteTransferQueueItems(ctx, nodeID)
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
		return rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
	}

	return nil
}

func (endpoint *Endpoint) processIncomplete(ctx context.Context, stream processStream, pending *pendingMap, incomplete *TransferQueueItem) error {
	nodeID := incomplete.NodeID
	pointer, err := endpoint.metainfo.Get(ctx, string(incomplete.Path))
	if err != nil {
		return Error.Wrap(err)
	}
	remote := pointer.GetRemote()

	pieces := remote.GetRemotePieces()
	var nodePiece *pb.RemotePiece
	excludedNodeIDs := make([]storj.NodeID, len(pieces))
	for i, piece := range pieces {
		if piece.NodeId == nodeID && piece.PieceNum == incomplete.PieceNum {
			nodePiece = piece
		}
		excludedNodeIDs[i] = piece.NodeId
	}

	if nodePiece == nil {
		endpoint.log.Debug("piece no longer held by node", zap.Stringer("node ID", nodeID), zap.ByteString("path", incomplete.Path), zap.Int32("piece num", incomplete.PieceNum))

		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.Path, incomplete.PieceNum)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return Error.Wrap(err)
	}

	if len(remote.GetRemotePieces()) > redundancy.OptimalThreshold() {
		endpoint.log.Debug("pointer has more pieces than required. removing node from pointer.", zap.Stringer("node ID", nodeID), zap.ByteString("path", incomplete.Path), zap.Int32("piece num", incomplete.PieceNum))

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

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)

	request := overlay.FindStorageNodesRequest{
		RequestedCount: 1,
		FreeBandwidth:  pieceSize,
		FreeDisk:       pieceSize,
		ExcludedNodes:  excludedNodeIDs,
	}

	newNodes, err := endpoint.overlay.FindStorageNodes(ctx, request)
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
	pending.put(pieceID, &pendingTransfer{
		path:             incomplete.Path,
		pieceSize:        pieceSize,
		satelliteMessage: transferMsg,
		rootPieceID:      remote.RootPieceId,
		pieceNum:         incomplete.PieceNum,
	})

	return nil
}

func (endpoint *Endpoint) handleSucceeded(ctx context.Context, stream processStream, pending *pendingMap, exitingNodeID storj.NodeID, message *pb.StorageNodeMessage_Succeeded) (err error) {
	defer mon.Task()(&ctx)(&err)

	originalPieceID := message.Succeeded.OriginalPieceId

	transfer, ok := pending.get(originalPieceID)
	if !ok {
		endpoint.log.Error("Could not find transfer item in pending queue", zap.Stringer("Piece ID", originalPieceID))
		return Error.New("Could not find transfer item in pending queue")
	}

	if transfer.satelliteMessage == nil {
		return Error.New("Satellite message cannot be nil")
	}
	if transfer.satelliteMessage.GetTransferPiece() == nil {
		return Error.New("Satellite message transfer piece cannot be nil")
	}
	if transfer.satelliteMessage.GetTransferPiece().GetAddressedOrderLimit() == nil {
		return Error.New("Addressed order limit on transfer piece cannot be nil")
	}
	if transfer.satelliteMessage.GetTransferPiece().GetAddressedOrderLimit().GetLimit() == nil {
		return Error.New("Addressed order limit on transfer piece cannot be nil")
	}
	if transfer.path == nil {
		return Error.New("Transfer path cannot be nil")
	}

	originalOrderLimit := message.Succeeded.GetOriginalOrderLimit()
	if originalOrderLimit == nil {
		return ErrInvalidArgument.New("Original order limit cannot be nil")
	}
	originalPieceHash := message.Succeeded.GetOriginalPieceHash()
	if originalPieceHash == nil {
		return ErrInvalidArgument.New("Original piece hash cannot be nil")
	}
	replacementPieceHash := message.Succeeded.GetReplacementPieceHash()
	if replacementPieceHash == nil {
		return ErrInvalidArgument.New("Replacement piece hash cannot be nil")
	}

	// verify that the original piece hash and replacement piece hash match
	if !bytes.Equal(originalPieceHash.Hash, replacementPieceHash.Hash) {
		return ErrInvalidArgument.New("Piece hashes for transferred piece don't match")
	}

	// verify that the satellite signed the original order limit
	err = endpoint.orders.VerifyOrderLimitSignature(ctx, originalOrderLimit)
	if err != nil {
		return ErrInvalidArgument.Wrap(err)
	}

	// verify that the public key on the order limit signed the original piece hash
	err = signing.VerifyUplinkPieceHashSignature(ctx, originalOrderLimit.UplinkPublicKey, originalPieceHash)
	if err != nil {
		return ErrInvalidArgument.Wrap(err)
	}

	if originalOrderLimit.PieceId != message.Succeeded.OriginalPieceId {
		return ErrInvalidArgument.New("Invalid original piece ID")
	}

	receivingNodeID := transfer.satelliteMessage.GetTransferPiece().GetAddressedOrderLimit().GetLimit().StorageNodeId

	calculatedNewPieceID := transfer.rootPieceID.Derive(receivingNodeID, transfer.pieceNum)
	if calculatedNewPieceID != replacementPieceHash.PieceId {
		return ErrInvalidArgument.New("Invalid replacement piece ID")
	}

	// get peerID and signee for new storage node
	peerID, err := endpoint.peerIdentities.Get(ctx, receivingNodeID)
	if err != nil {
		return Error.Wrap(err)
	}
	signee := signing.SigneeFromPeerIdentity(peerID)

	// verify that the new node signed the replacement piece hash
	err = signing.VerifyPieceHashSignature(ctx, signee, replacementPieceHash)
	if err != nil {
		return ErrInvalidArgument.Wrap(err)
	}

	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, exitingNodeID, transfer.path, transfer.pieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.updatePointer(ctx, exitingNodeID, receivingNodeID, transfer.path, transfer.pieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	var failed int64
	if transferQueueItem.FailedCount != nil && *transferQueueItem.FailedCount >= endpoint.config.MaxFailuresPerPiece {
		failed = -1
	}

	err = endpoint.db.IncrementProgress(ctx, exitingNodeID, transfer.pieceSize, 1, failed)
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.db.DeleteTransferQueueItem(ctx, exitingNodeID, transfer.path, transfer.pieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	pending.delete(originalPieceID)

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
	return nil
}

func (endpoint *Endpoint) handleFailed(ctx context.Context, pending *pendingMap, nodeID storj.NodeID, message *pb.StorageNodeMessage_Failed) (err error) {
	defer mon.Task()(&ctx)(&err)
	endpoint.log.Warn("transfer failed", zap.Stringer("piece ID", message.Failed.OriginalPieceId), zap.Stringer("transfer error", message.Failed.GetError()))
	pieceID := message.Failed.OriginalPieceId
	transfer, ok := pending.get(pieceID)
	if !ok {
		endpoint.log.Debug("could not find transfer message in pending queue. skipping.", zap.Stringer("piece ID", pieceID))

		// TODO we should probably error out here so we don't get stuck in a loop with a SN that is not behaving properl
	}
	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, nodeID, transfer.path, transfer.pieceNum)
	if err != nil {
		return Error.Wrap(err)
	}
	now := time.Now().UTC()
	failedCount := 1
	if transferQueueItem.FailedCount != nil {
		failedCount = *transferQueueItem.FailedCount + 1
	}

	errorCode := int(pb.TransferFailed_Error_value[message.Failed.Error.String()])

	// TODO if error code is NOT_FOUND, the node no longer has the piece. remove the queue item and the pointer

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

	pending.delete(pieceID)

	return nil
}

func (endpoint *Endpoint) getFinishedMessage(ctx context.Context, signer signing.Signer, nodeID storj.NodeID, finishedAt time.Time, success bool, reason pb.ExitFailed_Reason) (message *pb.SatelliteMessage, err error) {
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

func (endpoint *Endpoint) updatePointer(ctx context.Context, exitingNodeID storj.NodeID, receivingNodeID storj.NodeID, path []byte, pieceNum int32) (err error) {
	defer mon.Task()(&ctx)(&err)

	// remove the node from the pointer
	pointer, err := endpoint.metainfo.Get(ctx, string(path))
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
	// check receiving node id is not already in the pointer
	_, ok = pieceMap[receivingNodeID]
	if ok {
		return Error.New("node id already exists in piece. Path: %s, NodeID: %s", path, receivingNodeID.String())
	}
	if !receivingNodeID.IsZero() {
		toAdd = []*pb.RemotePiece{{
			PieceNum: pieceNum,
			NodeId:   receivingNodeID,
		}}
	}
	_, err = endpoint.metainfo.UpdatePieces(ctx, string(path), pointer, toAdd, toRemove)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
