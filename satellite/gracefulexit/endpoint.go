// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink/eestream"
)

// millis for the transfer queue building ticker
const buildQueueMillis = 100

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
	log       *zap.Logger
	interval  time.Duration
	db        DB
	overlaydb overlay.DB
	overlay   *overlay.Service
	metainfo  *metainfo.Service
	orders    *orders.Service
	config    Config
}

type pendingTransfer struct {
	path             []byte
	pieceSize        int64
	satelliteMessage *pb.SatelliteMessage
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
func NewEndpoint(log *zap.Logger, db DB, overlaydb overlay.DB, overlay *overlay.Service, metainfo *metainfo.Service, orders *orders.Service, config Config) *Endpoint {
	return &Endpoint{
		log:       log,
		interval:  time.Millisecond * buildQueueMillis,
		db:        db,
		overlaydb: overlaydb,
		overlay:   overlay,
		metainfo:  metainfo,
		orders:    orders,
		config:    config,
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
		return rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	// TODO should we error if the node is DQ'd?

	nodeID := peer.ID
	endpoint.log.Debug("graceful exit process", zap.Stringer("node ID", nodeID))

	eofHandler := func(err error) error {
		if err == io.EOF {
			endpoint.log.Debug("received EOF when trying to receive messages from storage node", zap.Stringer("node ID", nodeID))
			return nil
		}
		return rpcstatus.Error(rpcstatus.Unknown, err.Error())
	}

	exitStatus, err := endpoint.overlaydb.GetExitStatus(ctx, nodeID)
	if err != nil {
		return Error.Wrap(err)
	}

	if exitStatus.ExitFinishedAt != nil {
		// TODO revisit this. Should check if signature was sent
		completed := &pb.SatelliteMessage{Message: &pb.SatelliteMessage_ExitCompleted{ExitCompleted: &pb.ExitCompleted{}}}
		err = stream.Send(completed)
		return Error.Wrap(err)
	}

	if exitStatus.ExitInitiatedAt == nil {
		request := &overlay.ExitStatusRequest{NodeID: nodeID, ExitInitiatedAt: time.Now().UTC()}
		_, err = endpoint.overlaydb.UpdateExitStatus(ctx, request)
		if err != nil {
			return Error.Wrap(err)
		}

		err = stream.Send(&pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}})
		return Error.Wrap(err)
	}

	if exitStatus.ExitLoopCompletedAt == nil {
		err = stream.Send(&pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}})
		return Error.Wrap(err)
	}

	pending := newPendingMap()

	var morePiecesFlag int32 = 1
	errChan := make(chan error, 1)
	handleError := func(err error) error {
		errChan <- err
		close(errChan)
		return Error.Wrap(err)
	}

	var group errgroup.Group
	group.Go(func() error {
		ticker := time.NewTicker(endpoint.interval)
		defer ticker.Stop()

		for range ticker.C {
			if pending.length() == 0 {
				incomplete, err := endpoint.db.GetIncompleteNotFailed(ctx, nodeID, endpoint.config.EndpointBatchSize, 0)
				if err != nil {
					return handleError(err)
				}

				if len(incomplete) == 0 {
					incomplete, err = endpoint.db.GetIncompleteFailed(ctx, nodeID, endpoint.config.EndpointMaxFailures, endpoint.config.EndpointBatchSize, 0)
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
		// if there are no more transfers and the pending queue is empty, send complete
		if atomic.LoadInt32(&morePiecesFlag) == 0 && pendingCount == 0 {
			// TODO check whether failure threshold is met before sending completed
			// TODO needs exit signature
			transferMsg := &pb.SatelliteMessage{
				Message: &pb.SatelliteMessage_ExitCompleted{
					ExitCompleted: &pb.ExitCompleted{},
				},
			}
			err = stream.Send(transferMsg)
			if err != nil {
				return Error.Wrap(err)
			}
			break
		}
		// skip if there are none pending
		if pendingCount == 0 {
			continue
		}

		request, err := stream.Recv()
		if err != nil {
			return eofHandler(err)
		}

		switch m := request.GetMessage().(type) {
		case *pb.StorageNodeMessage_Succeeded:
			err = endpoint.handleSucceeded(ctx, pending, nodeID, m)
			if err != nil {
				return Error.Wrap(err)
			}
			deleteMsg := &pb.SatelliteMessage{
				Message: &pb.SatelliteMessage_DeletePiece{
					DeletePiece: &pb.DeletePiece{
						OriginalPieceId: m.Succeeded.OriginalPieceId,
					},
				},
			}
			err = stream.Send(deleteMsg)
			if err != nil {
				return Error.Wrap(err)
			}
		case *pb.StorageNodeMessage_Failed:
			err = endpoint.handleFailed(ctx, pending, nodeID, m)
			if err != nil {
				return Error.Wrap(err)
			}
		default:
			return Error.New("unknown storage node message: %v", m)
		}
	}

	if err := group.Wait(); err != nil {
		return err
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

		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.Path)
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

		err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, incomplete.Path)
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
	})

	return nil
}

func (endpoint *Endpoint) handleSucceeded(ctx context.Context, pending *pendingMap, nodeID storj.NodeID, message *pb.StorageNodeMessage_Succeeded) (err error) {
	defer mon.Task()(&ctx)(&err)
	if message.Succeeded.GetOriginalOrderLimit() == nil {
		return Error.New("original order limit cannot be nil.")
	}
	if message.Succeeded.GetOriginalPieceHash() == nil {
		return Error.New("original piece hash cannot be nil.")
	}

	pieceID := message.Succeeded.OriginalPieceId
	endpoint.log.Debug("transfer succeeded", zap.Stringer("piece ID", pieceID))

	// TODO validation

	transfer, ok := pending.get(pieceID)
	if !ok {
		endpoint.log.Debug("could not find transfer message in pending queue. skipping.", zap.Stringer("piece ID", pieceID))

		// TODO we should probably error out here so we don't get stuck in a loop with a SN that is not behaving properly
	}

	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, nodeID, transfer.path)
	if err != nil {
		return Error.Wrap(err)
	}

	var failed int64
	if transferQueueItem.FailedCount != nil && *transferQueueItem.FailedCount > 0 {
		failed = -1
	}

	err = endpoint.db.IncrementProgress(ctx, nodeID, transfer.pieceSize, 1, failed)
	if err != nil {
		return Error.Wrap(err)
	}

	err = endpoint.db.DeleteTransferQueueItem(ctx, nodeID, transfer.path)
	if err != nil {
		return Error.Wrap(err)
	}

	pending.delete(pieceID)

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
	transferQueueItem, err := endpoint.db.GetTransferQueueItem(ctx, nodeID, transfer.path)
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

	// only increment failed if it hasn't failed before
	if failedCount == 1 {
		err = endpoint.db.IncrementProgress(ctx, nodeID, 0, 0, 1)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	pending.delete(pieceID)

	return nil
}
