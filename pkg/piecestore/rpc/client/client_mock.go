// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"golang.org/x/net/context"
	pb "storj.io/storj/protos/piecestore"
)

func NewMock(ctx context.Context, route pb.PieceStoreRoutesClient) *Client {
	return &Client{ctx, route}
}
