// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/ranger"
	proto "storj.io/storj/protos/overlay"
)

// PieceID is temporarily defined here.
// TODO: remove it.
type PieceID string

// PSClient is temporarily defined here.
// TODO: remove it.
type PSClient interface {
	Put(ctx context.Context, pieceID PieceID, data io.Reader,
		expiration time.Time) error
	Get(ctx context.Context, pieceID PieceID, size int64) (
		ranger.RangeCloser, error)
	Delete(ctx context.Context, pieceID PieceID) error
	CloseConn() error
}

// NewPSClient is temporarily defined here.
// TODO: remove it.
func NewPSClient(conn *grpc.ClientConn) PSClient {
	return nil
}

// TransportClient is temporarily defined here.
// TODO: remove it
type TransportClient interface {
	DialNode(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
}
