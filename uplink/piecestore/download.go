// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"

	"storj.io/storj/pkg/auth/signing"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

type Download struct {
	client *Client
	limit  *pb.OrderLimit2
	peer   *identity.PeerIdentity
	stream pb.Piecestore_DownloadClient

	read         int64 // how much data we have read so far
	allocated    int64 // how far have we sent orders
	downloaded   int64 // how much data have we downloaded
	downloadSize int64 // how much do we want to download

	// what is the step we consider to upload
	allocationStep int64

	unread ReadBuffer

	// when there's a send error then it will automatically close
	sendError error
}

func (client *Client) Download(ctx context.Context, limit *pb.OrderLimit2, offset, size int64) (*Download, error) {
	stream, err := client.client.Download(ctx)
	if err != nil {
		return nil, err
	}

	peer, err := identity.PeerIdentityFromContext(stream.Context())
	if err != nil {
		return nil, ErrInternal.Wrap(err)
	}

	err = stream.Send(&pb.PieceDownloadRequest{
		Limit: limit,
		Chunk: &pb.PieceDownloadRequest_Chunk{
			Offset:    offset,
			ChunkSize: size,
		},
	})
	if err != nil {
		_, closeErr := stream.Recv()
		return nil, ErrProtocol.Wrap(combineSendCloseError(err, closeErr))
	}

	return &Download{
		client: client,
		limit:  limit,
		peer:   peer,
		stream: stream,

		read: 0,

		allocated:    0,
		downloaded:   0,
		downloadSize: size,

		allocationStep: client.config.InitialStep,
	}, nil
}

func (client *Download) Read(data []byte) (read int, _ error) {
	for client.read < client.downloadSize {
		// read from buffer
		n, err := client.unread.Read(data)
		client.read += int64(n)
		read += n

		// if we have an error or are pending for an error, avoid further communication
		if err != nil || client.unread.Errored() {
			return read, err
		}

		// do we need to send a new order to storagenode
		if client.allocated-client.downloaded < client.allocationStep {
			newAllocation := client.allocationStep

			// have we downloaded more than we have allocated due to a generous storagenode?
			if client.allocated-client.downloaded < 0 {
				newAllocation += client.downloaded - client.allocated
			}

			// ensure we don't allocate more than we intend to read
			if client.allocated+newAllocation > client.downloadSize {
				newAllocation = client.downloadSize - client.allocated
			}

			// send an order
			if newAllocation > 0 {
				// sign the order
				order, err := signing.SignOrder(client.client.signer, &pb.Order2{
					SerialNumber: client.limit.SerialNumber,
					Amount:       newAllocation,
				})
				// something went wrong
				if err != nil {
					client.unread.IncludeError(err)
					return read, nil
				}

				err = client.stream.Send(&pb.PieceDownloadRequest{
					Order: order,
				})
				if err != nil {
					client.sendError = err
					client.unread.IncludeError(err)
					return read, nil
				}

				// update our allocation step
				client.allocationStep = client.client.nextAllocationStep(client.allocationStep)
			}
		} // if end allocation sending

		// we have data, no need to wait for a chunk
		if read > 0 {
			return read, nil
		}

		// we don't have data, wait for a chunk from storage node
		response, err := client.stream.Recv()
		if response.Chunk != nil {
			client.downloaded += int64(len(response.Chunk.Data))
			client.unread.Fill(response.Chunk.Data)
		}

		// we still need to continue until we have actually handled all of the errors
		client.unread.IncludeError(err)
	}

	// all downloaded
	if read == 0 {
		return 0, io.EOF
	}
	return read, nil
}

func (client *Download) Close() error {
	sendCloseError := client.stream.CloseSend()

	_, recvError := client.stream.Recv()
	// grpc signals good end of stream with io.EOF
	if recvError == io.EOF {
		recvError = nil
	}

	return Error.Wrap(combineSendCloseError(sendCloseError, recvError))

}

type ReadBuffer struct {
	data []byte
	err  error
}

func (buffer *ReadBuffer) Errored() bool { return buffer.err != nil }

func (buffer *ReadBuffer) Empty() bool {
	return len(buffer.data) == 0 && buffer.err == nil
}

func (buffer *ReadBuffer) IncludeError(err error) {
	buffer.err = errs.Combine(buffer.err, err)
}

func (buffer *ReadBuffer) Fill(data []byte) {
	buffer.data = data
}

func (buffer *ReadBuffer) Read(data []byte) (n int, err error) {
	if len(buffer.data) > 0 {
		n = copy(data, buffer.data)
		buffer.data = buffer.data[n:]
		return n, nil
	}

	if buffer.err != nil {
		return 0, buffer.err
	}

	return 0, nil
}
