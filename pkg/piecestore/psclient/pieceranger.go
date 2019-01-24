// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psclient

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
)

// Error is the error class for pieceRanger
var Error = errs.Class("pieceRanger error")

type pieceRanger struct {
	c             *PieceStore
	id            PieceID
	size          int64
	stream        pb.PieceStoreRoutes_RetrieveClient
	pba           *pb.PayerBandwidthAllocation
	authorization *pb.SignedMessage
}

// PieceRanger PieceRanger returns a Ranger from a PieceID.
func PieceRanger(ctx context.Context, c *PieceStore, stream pb.PieceStoreRoutes_RetrieveClient, id PieceID, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (ranger.Ranger, error) {
	piece, err := c.Meta(ctx, id)
	if err != nil {
		return nil, err
	}
	return &pieceRanger{c: c, id: id, size: piece.PieceSize, stream: stream, pba: pba, authorization: authorization}, nil
}

// PieceRangerSize creates a PieceRanger with known size.
// Use it if you know the piece size. This will safe the extra request for
// retrieving the piece size from the piece storage.
func PieceRangerSize(c *PieceStore, stream pb.PieceStoreRoutes_RetrieveClient, id PieceID, size int64, pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) ranger.Ranger {
	return &pieceRanger{c: c, id: id, size: size, stream: stream, pba: pba, authorization: authorization}
}

// Size implements Ranger.Size
func (r *pieceRanger) Size() int64 {
	return r.size
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

	// send piece data
	if err := r.stream.Send(&pb.PieceRetrieval{PieceData: &pb.PieceRetrieval_PieceData{Id: r.id.String(), PieceSize: length, Offset: offset}, Authorization: r.authorization}); err != nil {
		return nil, err
	}

	return NewStreamReader(r.c, r.stream, r.pba, r.size), nil
}
