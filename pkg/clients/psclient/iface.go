// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psclient

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/dtypes"
	"storj.io/storj/pkg/ranger"
)

func NewPSClient(conn *grpc.ClientConn) PSClient {
	panic("TODO")
}

type PSClient interface {
	Put(ctx context.Context, pieceID dtypes.PieceID, data io.Reader,
		expiration time.Time) error
	Get(ctx context.Context, pieceID dtypes.PieceID, size int64) (
		ranger.Ranger, error)
	Delete(ctx context.Context, pieceID dtypes.PieceID) error
}
