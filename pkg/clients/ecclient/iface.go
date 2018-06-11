// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/clients/netclient"
	"storj.io/storj/pkg/dtypes"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/ranger"
)

func NewECClient(netclient netclient.NetClient) ECClient {
	panic("TODO")
}

type ECClient interface {
	Put(ctx context.Context, nodes []dtypes.Node, es eestream.ErasureScheme,
		pieceID dtypes.PieceID, data io.Reader, expiration time.Time) error
	Get(ctx context.Context, nodes []dtypes.Node, es eestream.ErasureScheme,
		pieceID dtypes.PieceID, size int64) (ranger.Ranger, error)
	Delete(ctx context.Context, nodes []dtypes.Node, pieceID dtypes.PieceID) error
}
