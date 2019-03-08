// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"crypto/sha256"
	"hash"
	"io"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

type Upload struct {
	client *Client
	limit  *pb.OrderLimit2
	peer   *identity.PeerIdentity
	stream pb.Piecestore_UploadClient

	hash           hash.Hash // TODO: use concrete implementation
	offset         int64
	allocationStep int64

	// when there's a send error then it will automatically close
	sendError error
}

func (client *Client) Upload(ctx context.Context, limit *pb.OrderLimit2) (*Upload, error) {
	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, ErrInternal.Wrap(err)
	}

	stream, err := client.client.Upload(ctx)
	if err != nil {
		return nil, err
	}

	err = stream.Send(&pb.PieceUploadRequest{
		Limit: limit,
	})
	if err != nil {
		_, closeErr := stream.CloseAndRecv()
		return nil, ErrProtocol.Wrap(combineSendCloseError(err, closeErr))
	}

	return &Upload{
		client: client,
		limit:  limit,
		peer:   peer,
		stream: stream,

		hash:           sha256.New(),
		offset:         0,
		allocationStep: client.config.InitialStep,
	}, nil
}

func (client *Upload) Write(data []byte) (written int, _ error) {
	// if we already encountered an error, keep returning it
	if client.sendError != nil {
		return 0, ErrProtocol.Wrap(client.sendError)
	}

	// hash the content so far
	_, _ = client.hash.Write(data) // guaranteed not to return error

	for len(data) > 0 {
		// pick a data chunk to send
		var sendData []byte
		if client.allocationStep < int64(len(data)) {
			sendData, data = data[:client.allocationStep], data[client.allocationStep:]
		} else {
			sendData, data = data, nil
		}

		// create a signed order for the next chunk
		order, err := client.client.SignOrder(&pb.Order2{
			SerialNumber: client.limit.SerialNumber,
			Amount:       client.offset + int64(len(sendData)),
		})
		if err != nil {
			return written, ErrInternal.Wrap(err)
		}

		// send signed order so that storagenode will accept data
		err = client.stream.Send(&pb.PieceUploadRequest{
			Order: order,
		})
		if err != nil {
			client.sendError = err
			return written, ErrProtocol.Wrap(client.sendError)
		}

		// send data as the next message
		err = client.stream.Send(&pb.PieceUploadRequest{
			Chunk: &pb.PieceUploadRequest_Chunk{
				Offset: client.offset,
				Data:   sendData,
			},
		})
		if err != nil {
			client.sendError = err
			return written, ErrProtocol.Wrap(client.sendError)
		}

		// update our offset
		client.offset += int64(len(sendData))
		written += len(sendData)

		// update allocation step, incrementally building trust
		client.allocationStep *= 3 / 2
		if client.allocationStep > client.client.config.MaximumStep {
			client.allocationStep = client.client.config.MaximumStep
		}
	}

	return written, nil
}

func (client *Upload) Close() (*pb.PieceHash, error) {
	if client.sendError != nil {
		_, closeErr := client.stream.CloseAndRecv()
		return nil, Error.Wrap(closeErr)
	}

	// sign the hash for storage node
	uplinkHash, err := client.client.SignPieceHash(&pb.PieceHash{
		PieceId: client.limit.PieceId,
		Hash:    client.hash.Sum(nil),
	})
	if err != nil {
		_, closeErr := client.stream.CloseAndRecv()
		return nil, Error.Wrap(combineSendCloseError(err, closeErr))
	}

	// exchange signed piece hashes
	err = client.stream.Send(&pb.PieceUploadRequest{
		Done: uplinkHash,
	})
	response, closeErr := client.stream.CloseAndRecv()

	// verification
	verifyErr := client.client.VerifyPieceHash(client.stream.Context(), client.peer, client.limit, response.Done, uplinkHash.Hash)

	// combine all the errors from before
	return response, errs.Combine(combineSendCloseError(err, closeErr), verifyErr)
}

type Download struct {
	Client pb.PiecestoreClient
}

func (client *Client) Download(ctx context.Context, limit *pb.OrderLimit2, offset, size int64) (*Download, error) {
	panic("TODO")
}

func (client *Download) Read(data []byte) error {
	panic("TODO")
	// these correspond to piecestore.Endpoint methods
}

func (client *Download) Close() error {
	panic("TODO")
}

func combineSendCloseError(sendError, closeError error) error {
	if sendError != nil && closeError != nil {
		if sendError == io.EOF {
			sendError = nil
		}
	}
	return errs.Combine(closeError, sendError)
}
