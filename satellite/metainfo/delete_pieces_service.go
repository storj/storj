// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
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

// piecesToDeleteLlimit is the maximum number of piece IDs can be sent to a storagenode in a request. Currently, the calculation is based on DRPC maximum message side, 4MB
const piecesToDeleteLimit = 1000

const minNodeOperationTimeout = 5 * time.Millisecond
const maxNodeOperationTimeout = 10 * time.Minute

// DeletePiecesService is the metainfo service in charge of deleting pieces of
// storage nodes.
//
// architecture: Service
type DeletePiecesService struct {
	log     *zap.Logger
	dialer  rpc.Dialer
	config  DeletePiecesServiceConfig
	limiter *sync2.ParentLimiter
}

// NewDeletePiecesService creates a new DeletePiecesService. maxConcurrentConns
// is the maximum number of connections that each single method call uses.
//
// It returns an error if maxConcurrentConns is less or equal than 0, dialer is
// a zero value or log is nil.
func NewDeletePiecesService(log *zap.Logger, dialer rpc.Dialer, config DeletePiecesServiceConfig) (*DeletePiecesService, error) {
	if config.MaxConcurrentConnection <= 0 {
		return nil, ErrDeletePieces.New(
			"max concurrent connections must be greater than 0, got %d", config.MaxConcurrentConnection,
		)
	}

	if config.NodeOperationTimeout < minNodeOperationTimeout || config.NodeOperationTimeout > maxNodeOperationTimeout {
		return nil, ErrDeletePieces.New(
			"node operation timeout must be greater than %d and less than %d, got %d", minNodeOperationTimeout, maxNodeOperationTimeout, config.NodeOperationTimeout,
		)
	}

	if dialer == (rpc.Dialer{}) {
		return nil, ErrDeletePieces.New("%s", "dialer cannot be its zero value")
	}

	if log == nil {
		return nil, ErrDeletePieces.New("%s", "logger cannot be nil")
	}

	return &DeletePiecesService{
		limiter: sync2.NewParentLimiter(config.MaxConcurrentConnection),
		config:  config,
		dialer:  dialer,
		log:     log,
	}, nil
}

// DeletePieces deletes all the indicated pieces of the nodes which are online
// stopping 300 milliseconds after reaching the successThreshold of the total
// number of pieces otherwise when trying to delete all the pieces finishes.
//
// It only returns an error if sync2.NewSuccessThreshold returns an error.
func (service *DeletePiecesService) DeletePieces(
	ctx context.Context, nodes NodesPieces, successThreshold float64,
) (err error) {
	defer mon.Task()(&ctx, len(nodes), nodes.NumPieces(), successThreshold)(&err)

	threshold, err := sync2.NewSuccessThreshold(len(nodes), successThreshold)
	if err != nil {
		return err
	}

	limiter := service.limiter.Child()
	for _, n := range nodes {
		node := n.Node
		pieces := n.Pieces

		// create batches if number of pieces are more than the maximum of number of piece ids that can be sent in a single request
		pieceBatches := make([][]storj.PieceID, 0, (len(pieces)+piecesToDeleteLimit-1)/piecesToDeleteLimit)
		for len(pieces) > piecesToDeleteLimit {
			pieceBatches = append(pieceBatches, pieces[0:piecesToDeleteLimit])
			pieces = pieces[piecesToDeleteLimit:]
		}
		if len(pieces) > 0 {
			pieceBatches = append(pieceBatches, pieces)
		}

		limiter.Go(ctx, func() {
			ctx, cancel := context.WithTimeout(ctx, service.config.NodeOperationTimeout)
			defer cancel()
			// Track the rate that each single node is dialed
			mon.Event(fmt.Sprintf("DeletePieces_node_%s", node.Id.String()))

			// Track the low/high/recent/average/quantiles of successful nodes dialing.
			// Not stopping the timer doesn't leak resources.
			timerDialSuccess := mon.Timer("DeletePieces_nodes_dial_success").Start()
			client, err := piecestore.Dial(
				ctx, service.dialer, node, service.log, piecestore.Config{},
			)
			if err != nil {
				service.log.Warn("unable to dial storage node",
					zap.Stringer("node_id", node.Id),
					zap.Stringer("node_info", node),
					zap.Error(err),
				)

				threshold.Failure()

				// Pieces will be collected by garbage collector
				return
			}
			timerDialSuccess.Stop()

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

			for _, batch := range pieceBatches {
				err := client.DeletePieces(ctx, batch...)
				if err != nil {
					// piece will be collected by garbage collector
					service.log.Warn("unable to delete pieces of a storage node",
						zap.Stringer("node_id", node.Id),
						zap.Error(err),
					)

					// mark the node as failure if one error is returned since only authentication errors are returned by DeletePieces
					threshold.Failure()

					// Pieces will be collected by garbage collector
					return
				}
			}

			threshold.Success()

		})
	}

	threshold.Wait(ctx)
	return nil
}

// Close wait until all the resources used by the service are closed before
// returning.
func (service *DeletePiecesService) Close() error {
	service.limiter.Wait()
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
