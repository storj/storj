// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/ranger"
)

var Error = errs.Class("grpcRanger error")

type grpcRanger struct {
	c    *Client
	id   string
	size int64
}

// GRPCRanger turns a gRPC connection to piece store into a Ranger
func GRPCRanger(ctx context.Context, c *Client, id string) (ranger.Ranger, error) {
	piece, err := c.Meta(ctx, PieceID(id))
	if err != nil {
		return nil, err
	}
	return &grpcRanger{c: c, id: id, size: piece.Size}, nil
}

// GRPCRangerSize creates a GRPCRanger with known size.
// Use it if you know the piece size. This will safe the extra request for
// retrieving the piece size from the piece storage.
func GRPCRangerSize(c *Client, id string, size int64) ranger.Ranger {
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
	reader, err := r.c.Get(ctx, PieceID(r.id), offset, length)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return reader, nil
}
