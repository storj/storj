// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"crypto/sha256"
	"hash"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

type Download struct {
	client *Client
	limit  *pb.OrderLimit2
	peer   *identity.PeerIdentity
	stream pb.Piecestore_DownloadClient

	hash           hash.Hash // TODO: use concrete implementation
	undownloaded   int64
	allocationStep int64

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
		undownloaded:   size,
		allocationStep: client.config.InitialStep,
	}, nil
}

func (client *Download) Read(data []byte) error {
	panic("TODO")
	// these correspond to piecestore.Endpoint methods
}

func (client *Download) Close() error {
	panic("TODO")
}
