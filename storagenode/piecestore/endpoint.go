// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bytes"
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
)

var (
	mon = monkit.Package()

	Error       = errs.Class("piecestore error")
	ErrProtocol = errs.Class("piecestore protocol error")
	ErrInternal = errs.Class("piecestore internal error")
)

type Signer interface {
	ID() storj.NodeID
	SignHash(hash []byte) ([]byte, error)
}

type Trust interface {
	VerifySatellite(context.Context, storj.NodeID) error
	VerifyUplink(context.Context, storj.NodeID) error
	VerifySignature(context.Context, []byte, storj.NodeID) error
}

// TODO: avoid protobuf definitions in interfaces

type PieceMeta interface {
	Add(ctx context.Context, limit pb.OrderLimit2, hash pb.PieceHash) error
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

	Config Config

	Signer        Signer
	Trust         Trust
	ActiveSerials *SerialNumbers

	Pieces *pieces.Store

	PieceMeta PieceMeta
	Orders    orders.Table
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
	pieceInfoErr := endpoint.PieceMeta.Delete(ctx, delete.Limit.SatelliteId, delete.Limit.PieceId)
	pieceErr := endpoint.Pieces.Delete(ctx, delete.Limit.SatelliteId, delete.Limit.PieceId)

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

	pieceWriter, err := endpoint.Pieces.Writer(ctx, limit.SatelliteId, limit.PieceId)
	if err != nil {
		return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
	}
	defer pieceWriter.Cancel() // similarly how transcation Rollback works

	var largestOrder = &pb.Order2{}
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
			if message.Done.PieceId != limit.PieceId {
				return ErrProtocol.New("done message piece id was different") // TODO: report grpc status internal server error
			}
			if err := endpoint.VerifyPieceHash(message.Done, peer); err != nil {
				return err // TODO: report grpc status internal server error
			}

			hash := pieceWriter.Hash()
			if !bytes.Equal(hash, message.Hash) {
				return ErrProtocol.New("hash mismatch") // TODO: report grpc status bad message
			}

			if err := pieceWriter.Commit(limit, largestOrder, message.Done); err != nil {
				return ErrInternal.Wrap(err) // TODO: report grpc status internal server error
			}

			// TODO: do this in a goroutine
			{
				// TODO: since we have successfully commited, we should try to later recover the orders
				// from piece storage
				if err := endpoint.PieceMeta.Add(limit, message.Done); err != nil {
					return ErrInternal.Wrap(err)
				}
				if err := endpoint.Orders.Add(limit, largestOrder); err != nil {
					return ErrInternal.Wrap(err)
				}
			}

			return stream.SendAndClose(&pb.PieceUploadResponse{
				Done: endpoint.SignHash(limit.SerialNumber, limit.PieceId, pieceHash),
			})
		}
	}
}

func (endpoint *Endpoint) Download(stream pb.Piecestore_DownloadServer) error {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	// TODO: set connection timeouts
	// TODO: set maximum message size

	var message *pb.PieceDownloadRequest
	var err error

	// receive limit and chunk from uplink
	message, err = stream.Recv()
	if err != nil {
		return ErrProtocol.Wrap(err)
	}
	if message.Limit == nil || message.Chunk == nil {
		return ErrProtocol.New("expected order limit and chunk as the first message")
	}
	limit := message.Limit

	if limit.Action != pb.Action_GET && limit.Action != pb.Action_GET_REPAIR && limit.Action != pb.Action_GET_AUDIT {
		return ErrProtocol.New("expected get or get repair or audit action got %v", limit.Action) // TODO: report grpc status unauthorized or bad request
	}

	if message.Chunk.Size > limit.Limit {
		return ErrProtocol.New("requested more that order limit allows")
	}

	if err := endpoint.VerifyOrderLimit(ctx, limit); err != nil {
		return Error.Wrap(err) // TODO: report grpc status unauthorized or bad request
	}

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	pieceReader, err := endpoint.Pieces.Reader(limit.SatelliteID, limit.PieceID)
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
	if chunk.Offset+chunk.Size > pieceReader.Size() {
		return Error.New("requested more data than available")
	}

	throttle := sync2.NewThrottle()

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		// ensure uplink notices when we have closed things
		defer func() {
			err = errs.Combine(err, stream.CloseSend())
		}()

		var maximumChunkSize = 1 * memory.MiB.Int64()

		currentOffset := chunk.Offset
		unsentAmount := chunk.Size
		for unsentAmount > 0 {
			tryToSend := min(unsentAmount, maximumChunkSize)

			// TODO: add timeout here
			chunkSize, err := throttle.ConsumeOrWait(tryToSend)
			if err != nil {
				return ErrInternal(err)
			}

			chunk := make([]byte, chunkSize)
			err = pieceReader.ReadAt(currentOffset, chunk)
			if err != nil {
				return ErrInternal.Wrap(err)
			}

			err = stream.Send(&pb.DownloadResponseStream{
				Chunk: &pb.DownloadResponseStream_Chunk{
					Offset: currentOffset,
					Chunk:  chunk,
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
		defer func() {
			// TODO: do this in a goroutine
			if err2 := endpoint.Orders.Add(limit, largestOrder); err2 != nil {
				return errs.Combine(err, ErrInternal.Wrap(err2))
			}
		}()

		// ensure that we always terminate sending goroutine
		defer throttle.Fail(io.EOF)

		largestOrder := *pb.Order{}
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
