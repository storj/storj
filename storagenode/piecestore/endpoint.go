// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()

	Error       = errs.Class("piecestore error")
	ErrProtocol = errs.Class("piecestore protocol error")
	ErrInternal = errs.Class("piecestore internal error")
)

type Signer interface {
	ID() storj.NodeID
	HashAndSign(data []byte) ([]byte, error)
}

// TODO: avoid protobuf definitions in interfaces

type PieceMeta interface {
	Add(ctx context.Context, limit *pb.OrderLimit2, hash *pb.PieceHash) error
	Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID2) error
	// Iteration for collector
}

// TODO: should the reader, writer have context for read/write?

var _ pb.PiecestoreServer = (*Endpoint)(nil)

type Config struct {
	ExpirationGracePeriod time.Duration
}

type Endpoint struct {
	log *zap.Logger

	config Config

	signer        Signer
	trust         *trust.Pool
	activeSerials *SerialNumbers

	store *pieces.Store

	pieceMeta PieceMeta
	orders    orders.Table
}

func NewEndpoint(log *zap.Logger) (*Endpoint, error) {
	return &Endpoint{
		log: log,
		// TODO: panic
	}, nil
}

func (endpoint *Endpoint) Delete(ctx context.Context, delete *pb.PieceDeleteRequest) (_ *pb.PieceDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if delete.Limit.Action != pb.Action_DELETE {
		return nil, Error.New("expected delete action got %v", delete.Limit.Action) // TODO: report grpc status unauthorized or bad request
	}

	if err := endpoint.VerifyOrderLimit(ctx, delete.Limit); err != nil {
		// TODO: report grpc status unauthorized or bad request
		return nil, Error.Wrap(err)
	}

	// TODO: parallelize this and maybe return early
	pieceInfoErr := endpoint.pieceMeta.Delete(ctx, delete.Limit.SatelliteId, delete.Limit.PieceId)
	pieceErr := endpoint.store.Delete(ctx, delete.Limit.SatelliteId, delete.Limit.PieceId)

	if err := errs.Combine(pieceInfoErr, pieceErr); err != nil {
		// explicitly ignoring error because the errors
		// TODO: add more debug info
		endpoint.log.Error("unable to delete", zap.Error(err))
		// TODO: report internal server internal or missing error using grpc status,
		// e.g. missing might happen when we get a deletion request after garbage collection has deleted it
	}

	return &pb.PieceDeleteResponse{}, nil
}

func (endpoint *Endpoint) Upload(stream pb.Piecestore_UploadServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	// TODO: set connection timeouts
	// TODO: set maximum message size

	var message *pb.PieceUploadRequest

	message, err = stream.Recv()
	switch {
	case err != nil:
		return ErrProtocol.Wrap(err)
	case message.Limit == nil:
		return ErrProtocol.New("expected order limit as the first message")
	}
	limit := message.Limit

	// TODO: verify that we have have expected amount of storage before continuing

	if limit.Action != pb.Action_PUT && limit.Action != pb.Action_PUT_REPAIR {
		return ErrProtocol.New("expected put or put repair action got %v", limit.Action) // TODO: report grpc status unauthorized or bad request
	}

	if err := endpoint.VerifyOrderLimit(ctx, limit); err != nil {
		return err // TODO: report grpc status unauthorized or bad request
	}

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	pieceWriter, err := endpoint.store.Writer(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
	}
	defer pieceWriter.Cancel() // similarly how transcation Rollback works

	var largestOrder *pb.Order2
	for {
		message, err = stream.Recv() // TODO: reuse messages to avoid allocations
		if err != nil {
			return ErrProtocol.Wrap(err) // TODO: report grpc status bad message
		}

		switch {
		default:
			return ErrProtocol.New("message didn't contain any of order, chunk nor done") // TODO: report grpc status bad message

		case message.Order != nil:
			if err := endpoint.VerifyOrder(ctx, peer, limit, message.Order, largestOrder.Amount); err != nil {
				return err
			}
			largestOrder = message.Order

		case message.Chunk != nil:
			if message.Chunk.Offset != pieceWriter.Size() {
				return ErrProtocol.New("chunk out of order") // TODO: report grpc status bad message
			}

			if largestOrder.Amount < pieceWriter.Size()+int64(len(message.Chunk.Data)) {
				// TODO: should we write currently and give a chance for uplink to remedy the situation?
				return ErrProtocol.New("not enough allocated") // TODO: report grpc status ?
			}

			if _, err := pieceWriter.Write(message.Chunk.Data); err != nil {
				return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
			}

		case message.Done != nil:
			expectedHash := pieceWriter.Hash()
			if err := endpoint.VerifyPieceHash(ctx, peer, limit, message.Done, expectedHash); err != nil {
				return err // TODO: report grpc status internal server error
			}

			if err := pieceWriter.Commit(); err != nil {
				return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
			}

			// TODO: do this in a goroutine
			{
				// TODO: since we have successfully commited, we should try to later recover the orders
				// from piece storage
				if err := endpoint.pieceMeta.Add(ctx, limit, message.Done); err != nil {
					return ErrInternal.Wrap(err)
				}
				if largestOrder != nil {
					if err := endpoint.orders.Add(ctx, limit, largestOrder); err != nil {
						return ErrInternal.Wrap(err)
					}
				}
			}

			storageNodeHash, err := endpoint.SignPieceHash(&pb.PieceHash{
				PieceId: limit.PieceId,
				Hash:    expectedHash,
			})
			if err != nil {
				return ErrInternal.Wrap(err)
			}

			return stream.SendAndClose(&pb.PieceUploadResponse{
				Done: storageNodeHash,
			})
		}
	}
}

func (endpoint *Endpoint) Download(stream pb.Piecestore_DownloadServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	// TODO: set connection timeouts
	// TODO: set maximum message size

	var message *pb.PieceDownloadRequest

	// receive limit and chunk from uplink
	message, err = stream.Recv()
	if err != nil {
		return ErrProtocol.Wrap(err)
	}
	if message.Limit == nil || message.Chunk == nil {
		return ErrProtocol.New("expected order limit and chunk as the first message")
	}
	limit, chunk := message.Limit, message.Chunk

	if limit.Action != pb.Action_GET && limit.Action != pb.Action_GET_REPAIR && limit.Action != pb.Action_GET_AUDIT {
		return ErrProtocol.New("expected get or get repair or audit action got %v", limit.Action) // TODO: report grpc status unauthorized or bad request
	}

	if chunk.ChunkSize > limit.Limit {
		return ErrProtocol.New("requested more that order limit allows")
	}

	if err := endpoint.VerifyOrderLimit(ctx, limit); err != nil {
		return Error.Wrap(err) // TODO: report grpc status unauthorized or bad request
	}

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	pieceReader, err := endpoint.store.Reader(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
	}
	defer func() {
		err := pieceReader.Close() // similarly how transcation Rollback works
		if err != nil {
			// no reason to report this error to the uplink
			endpoint.log.Error("failed to close piece reader", zap.Error(err))
		}
	}()

	// TODO: verify chunk.Size behavior logic with regards to reading all
	if chunk.Offset+chunk.ChunkSize > pieceReader.Size() {
		return Error.New("requested more data than available")
	}

	throttle := sync2.NewThrottle()

	// TODO: see whether this can be implemented without a goroutine

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		var maximumChunkSize = 1 * memory.MiB.Int64()

		currentOffset := chunk.Offset
		unsentAmount := chunk.ChunkSize
		for unsentAmount > 0 {
			tryToSend := min(unsentAmount, maximumChunkSize)

			// TODO: add timeout here
			chunkSize, err := throttle.ConsumeOrWait(tryToSend)
			if err != nil {
				return ErrInternal.Wrap(err)
			}

			chunkData := make([]byte, chunkSize)
			err = pieceReader.ReadAt(currentOffset, chunkData)
			if err != nil {
				return ErrInternal.Wrap(err)
			}

			err = stream.Send(&pb.PieceDownloadResponse{
				Chunk: &pb.PieceDownloadResponse_Chunk{
					Offset: currentOffset,
					Data:   chunkData,
				},
			})
			if err != nil {
				return ErrProtocol.Wrap(err)
			}

			currentOffset += chunkSize
			unsentAmount -= chunkSize
		}

		return nil
	})

	recvErr := func() (err error) {
		var largestOrder *pb.Order2

		defer func() {
			// TODO: do this in a goroutine
			if largestOrder != nil {
				if err2 := endpoint.orders.Add(ctx, limit, largestOrder); err2 != nil {
					err = errs.Combine(err, ErrInternal.Wrap(err2))
				}
			}
		}()

		// ensure that we always terminate sending goroutine
		defer throttle.Fail(io.EOF)

		for {
			// TODO: check errors
			// TODO: add timeout here
			message, err = stream.Recv()
			if message.Order == nil {
				return ErrProtocol.New("expected order as the message")
			}
			if err := endpoint.VerifyOrder(ctx, peer, limit, message.Order, largestOrder.Amount); err != nil {
				return err
			}
			if err := throttle.Produce(message.Order.Amount - largestOrder.Amount); err != nil {
				// shouldn't happen since only receiving side is calling Fail
				return ErrProtocol.Wrap(err)
			}
			largestOrder = message.Order
		}
	}()

	// ensure we wait for sender to complete
	sendErr := group.Wait()

	// TODO: combine recvErr and sendErr somehow better
	return Error.Wrap(errs.Combine(sendErr, recvErr))
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
