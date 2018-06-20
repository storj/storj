// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"storj.io/storj/pkg/piecestore/rpc/client"
)

type grpcRanger struct {
	c    *client.Client
	id   string
	size int64
}

// GRPCRanger turns a gRPC connection to piece store into a Ranger
func GRPCRanger(ctx context.Context, c *client.Client, id string) (Ranger, error) {
	piece, err := c.PieceMetaRequest(ctx, id)
	if err != nil {
		return nil, err
	}
	return &grpcRanger{c: c, id: id, size: piece.Size}, nil
}

// GRPCRangerSize creates a GRPCRanger with known size.
// Use it if you know the piece size. This will safe the extra request for
// retrieving the piece size from the piece storage.
func GRPCRangerSize(c *client.Client, id string, size int64) Ranger {
	return &grpcRanger{c: c, id: id, size: size}
}

// Size implements Ranger.Size
func (r *grpcRanger) Size() int64 {
	return r.size
}

// Range implements Ranger.Range
func (r *grpcRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, Error.New("negative offset")
	}
	if length < 0 {
		return nil, Error.New("negative length")
	}
	if offset+length > r.size {
		return nil, Error.New("range beyond end")
	}
	if length == 0 {
		return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
	}
	reader, err := r.c.RetrievePieceRequest(ctx, r.id, offset, length)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return reader, nil
}
