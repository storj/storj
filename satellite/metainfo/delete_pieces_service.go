// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/uplink/piecestore"
)

// ErrDeletePieces is the general error class for DeletePiecesService
var ErrDeletePieces = errs.Class("metainfo storage node service")

// DeletePiecesService is the metainfo service in charge of deleting pieces of
// storage nodes.
//
// architecture: Service
type DeletePiecesService struct {
	log    *zap.Logger
	dialer rpc.Dialer

	// TODO: v3-3406 this values is currently only used to limit the concurrent
	// connections by each single method call.
	maxConns int
}

// NewDeletePiecesService creates a new DeletePiecesService. maxConcurrentConns
// is the maximum number of connections that each single method call uses.
//
// It returns an error if maxConcurrentConns is less or equal than 0, dialer is
// a zero value or log is nil.
func NewDeletePiecesService(log *zap.Logger, dialer rpc.Dialer, maxConcurrentConns int) (*DeletePiecesService, error) {
	// TODO: v3-3476 should we have an upper limit?
	if maxConcurrentConns <= 0 {
		return nil, ErrDeletePieces.New(
			"max concurrent connections must be greater than 0, got %d", maxConcurrentConns,
		)
	}

	if dialer == (rpc.Dialer{}) {
		return nil, ErrDeletePieces.New("%s", "dialer cannot be its zero value")
	}

	if log == nil {
		return nil, ErrDeletePieces.New("%s", "logger cannot be nil")
	}

	return &DeletePiecesService{
		maxConns: maxConcurrentConns,
		dialer:   dialer,
		log:      log,
	}, nil
}

// DeletePieces deletes all the indicated pieces of the nodes which are online
// stopping 300 milliseconds after reaching the successThreshold of the total
// number of pieces otherwise when trying to delete all the pieces finishes.
//
// It only returns an error if sync2.NewSuccessThreshold returns an error.
func (service *DeletePiecesService) DeletePieces(
	ctx context.Context, nodes NodesPieces, successThreshold float64,
) error {
	threshold, err := sync2.NewSuccessThreshold(nodes.NumPieces(), successThreshold)
	if err != nil {
		return err
	}

	// TODO: v3-3476 This timeout will go away in a second commit
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TODO: v3-3406 this limiter will be global to the service instance if we
	// decide to do so
	limiter := sync2.NewLimiter(service.maxConns)
	for _, n := range nodes {
		node := n.Node
		pieces := n.Pieces

		limiter.Go(ctx, func() {
			client, err := piecestore.Dial(
				ctx, service.dialer, node, service.log, piecestore.Config{},
			)
			if err != nil {
				service.log.Warn("unable to dial storage node",
					zap.Stringer("node_id", node.Id),
					zap.Stringer("node_info", node),
					zap.Error(err),
				)

				// Mark all the pieces of this node as failure in the success threshold
				for range pieces {
					threshold.Failure()
				}

				// Pieces will be collected by garbage collector
				return
			}
			defer func() {
				err := client.Close()
				if err != nil {
					service.log.Warn("error closing the storage node client connection",
						zap.Stringer("node_id", node.Id),
						zap.Stringer("node_info", node),
						zap.Error(err),
					)
				}
			}()

			for _, id := range pieces {
				err := client.DeletePiece(ctx, id)
				if err != nil {
					// piece will be collected by garbage collector
					service.log.Warn("unable to delete piece of a storage node",
						zap.Stringer("node_id", node.Id),
						zap.Stringer("piece_id", id),
						zap.Error(err),
					)

					threshold.Failure()
					continue
				}

				threshold.Success()
			}
		})
	}

	threshold.Wait(ctx)
	// return to the client after the success threshold but wait some time before
	// canceling the remaining deletes
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	defer timer.Stop()

	limiter.Wait()
	return nil
}

// Close wait until all the resources used by the service are closed before
// returning.
func (service *DeletePiecesService) Close() error {
	// TODO: orange/v3-3476 it will wait until all the goroutines run by the
	// DeletePieces finish rather than using the current timeout.
	return nil
}

// NodePieces indicates a list of pieces that belong to a storage node.
type NodePieces struct {
	Node   *pb.Node
	Pieces []storj.PieceID
}

// NodesPieces is a slice of NodePieces
type NodesPieces []NodePieces

// NumPieces sums the number of pieces of all the storage nodes of the slice and
// returns it.
func (nodes NodesPieces) NumPieces() int {
	total := 0
	for _, node := range nodes {
		total += len(node.Pieces)
	}

	return total
}
