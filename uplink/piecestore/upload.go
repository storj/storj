// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"crypto/sha256"
	"hash"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth/signing"
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
	stream, err := client.client.Upload(ctx)
	if err != nil {
		return nil, err
	}

	peer, err := identity.PeerIdentityFromContext(stream.Context())
	if err != nil {
		return nil, ErrInternal.Wrap(err)
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
		order, err := signing.SignOrder(client.client.signer, &pb.Order2{
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
		client.allocationStep = client.client.nextAllocationStep(client.allocationStep)
	}

	return written, nil
}

func (client *Upload) Close() (*pb.PieceHash, error) {
	if client.sendError != nil {
		_, closeErr := client.stream.CloseAndRecv()
		return nil, Error.Wrap(closeErr)
	}

	// sign the hash for storage node
	uplinkHash, err := signing.SignPieceHash(client.client.signer, &pb.PieceHash{
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
	return response.Done, errs.Combine(combineSendCloseError(err, closeErr), verifyErr)
}
