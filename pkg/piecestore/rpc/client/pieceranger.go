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
	pb "storj.io/storj/protos/piecestore"
)

// Error is the error class for pieceRanger
var Error = errs.Class("pieceRanger error")

type pieceRanger struct {
	c    *Client
	id   PieceID
	size int64
}

// PieceRanger PieceRanger returns a RangeCloser from a PieceID.
func PieceRanger(ctx context.Context, c *Client, id PieceID) (ranger.RangeCloser, error) {
	piece, err := c.Meta(ctx, PieceID(id))
	if err != nil {
		return nil, err
	}
	return &pieceRanger{c: c, id: id, size: piece.Size}, nil
}

// PieceRangerSize creates a PieceRanger with known size.
// Use it if you know the piece size. This will safe the extra request for
// retrieving the piece size from the piece storage.
func PieceRangerSize(c *Client, id PieceID, size int64) ranger.RangeCloser {
	return &pieceRanger{c: c, id: id, size: size}
}

// Size implements Ranger.Size
func (r *pieceRanger) Size() int64 {
	return r.size
}

// Size implements Ranger.Size
func (r *pieceRanger) Close() error {
	return r.c.CloseConn()
}

// Range implements Ranger.Range
func (r *pieceRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
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
	stream, err := r.c.route.Retrieve(ctx, &pb.PieceRetrieval{Id: r.id.String(), Size: length, Offset: offset})
	if err != nil {
		return nil, err
	}

	return NewStreamReader(stream), nil
}
