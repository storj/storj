// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"hash"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
)

type Uploader interface {
	Write([]byte) (int, error)
	Close() (*pb.PieceHash, error)
}

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

func (client *Client) Upload(ctx context.Context, limit *pb.OrderLimit2) (Uploader, error) {
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
		return nil, ErrProtocol.Wrap(errs.Combine(err, closeErr))
	}

	upload := &Upload{
		client: client,
		limit:  limit,
		peer:   peer,
		stream: stream,

		hash:           pkcrypto.NewHash(),
		offset:         0,
		allocationStep: client.config.InitialStep,
	}

	if client.config.UploadBufferSize <= 0 {
		return &LockingUpload{upload: upload}, nil
	}
	return &LockingUpload{
		upload: NewBufferedUpload(upload, int(client.config.UploadBufferSize)),
	}, nil
}

func (client *Upload) Write(data []byte) (written int, _ error) {
	// if we already encountered an error, keep returning it
	if client.sendError != nil {
		return 0, ErrProtocol.Wrap(client.sendError)
	}

	fullData := data
	defer func() {
		// write the hash of the data sent to the server
		// guaranteed not to return error
		_, _ = client.hash.Write(fullData[:written])
	}()

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
		// something happened during sending, try to figure out what exactly
		// since sendError was already reported, we don't need to rehandle it.
		_, closeErr := client.stream.CloseAndRecv()
		return nil, Error.Wrap(closeErr)
	}

	// sign the hash for storage node
	uplinkHash, err := signing.SignPieceHash(client.client.signer, &pb.PieceHash{
		PieceId: client.limit.PieceId,
		Hash:    client.hash.Sum(nil),
	})
	if err != nil {
		// failed to sign, let's close the sending side, no need to wait for a response
		closeErr := client.stream.CloseSend()
		// closeErr being io.EOF doesn't inform us about anything
		return nil, Error.Wrap(errs.Combine(err, ignoreEOF(closeErr)))
	}

	// exchange signed piece hashes
	// 1. send our piece hash
	sendErr := client.stream.Send(&pb.PieceUploadRequest{
		Done: uplinkHash,
	})

	// 2. wait for a piece hash as a response
	response, closeErr := client.stream.CloseAndRecv()
	if response == nil || response.Done == nil {
		// combine all the errors from before
		// sendErr is io.EOF when failed to send, so don't care
		// closeErr is io.EOF when storage node closed before sending us a response
		return nil, errs.Combine(ErrProtocol.New("expected piece hash"), ignoreEOF(sendErr), ignoreEOF(closeErr))
	}

	// verification
	verifyErr := client.client.VerifyPieceHash(client.stream.Context(), client.peer, client.limit, response.Done, uplinkHash.Hash)

	// combine all the errors from before
	// sendErr is io.EOF when we failed to send
	// closeErr is io.EOF when storage node closed properly
	return response.Done, errs.Combine(verifyErr, ignoreEOF(sendErr), ignoreEOF(closeErr))
}
