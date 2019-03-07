// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type Signer interface {
	ID() storj.NodeID
	SignHash(hash []byte) ([]byte, error)
}

type Config struct {
	StartingAllocationStep int64
	MaximumAllocationStep  int64
}

type Client struct {
	// TODO: hide
	Signer Signer
	Conn   *grpc.ClientConn
	Client pb.PiecestoreClient
	Config Config
}

// These can be used to implement psclient.Client
func (client *Client) Upload(ctx context.Context, limit *pb.OrderLimit2) (*Upload, error) {
	panic("TODO")
}

func (client *Client) Download(ctx context.Context, limit *pb.OrderLimit2, offset, size int64) (*Download, error) {
	panic("TODO")
}

func (client *Client) Delete(ctx context.Context, limit *pb.OrderLimit2) error {
	panic("TODO")
}

func (client *Client) Close() error {
	panic("TODO")
}
