// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Implementations for the interfaces using filestore

type Storage interface {
	Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (PieceWriter, error)
	Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (PieceReader, error)
	Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) error
}

type Table interface {
	Add(ctx context.Context, limit pb.OrderLimit, hash pb.PieceHash) error
	Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) error
}

type Writer interface {
	Write(data []byte) (int64, error)
	Hash() []byte
	Commit(pb.OrderLimit, pb.Order, pb.PieceHash) error
	// alternative: Commit() error, we could save them separately ensuring that we can rebuild

	// Cancel cancels writing to storage
	Cancel()
}

type Reader interface {
	ReadAt(offset int64, data []byte) error
	Size() int64

	Limit() pb.OrderLimit
	Order() pb.Order
	Hash() pb.PieceHash
}
