// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"crypto/sha256"
	"hash"
	"io"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

type Download struct {
	client *Client
	limit  *pb.OrderLimit2
	peer   *identity.PeerIdentity
	stream pb.Piecestore_DownloadClient

	hash           hash.Hash // TODO: use concrete implementation
	toDownload     int64
	allocationStep int64

	unread []byte

	// when there's a send error then it will automatically close
	sendError error
}

func (client *Client) Download(ctx context.Context, limit *pb.OrderLimit2, offset, size int64) (*Download, error) {
	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, ErrInternal.Wrap(err)
	}

	stream, err := client.client.Download(ctx)
	if err != nil {
		return nil, err
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

		hash:           sha256.New(),
		allocationStep: client.config.InitialStep,
	}, nil
}

func (client *Download) Read(data []byte) (read int, _ error) {
	// todo proper checks
	for client.toDownload > 0 {
		// read from unread buffer

		// check whether we need to send new allocations

		// if we emptied the unread buffer && read > 0
		//     return read, nil

		// shouldn't try to read more data when we have error
		// client.sendError != nil { return read, err }

		// resp, err := client.stream.Recv()
		// add chunk to unread buffer
	}

	// all downloaded
	if read == 0 {
		return 0, io.EOF
	}
	return read, nil
}

func (client *Download) Close() error {
	panic("TODO")
}
