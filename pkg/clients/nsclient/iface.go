// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nsclient

import (
	"context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/dtypes"
	pb "storj.io/storj/protos/netstate"
)

func NewNSClient(conn *grpc.ClientConn) NSClient {
	panic("TODO")
}

type NSClient interface {
	Put(ctx context.Context, path dtypes.Path, pointer *pb.Pointer) error
	Get(ctx context.Context, path dtypes.Path) (*pb.Pointer, error)
	Delete(ctx context.Context, path dtypes.Path) error
	List(ctx context.Context, startingPath, endingPath dtypes.Path) (
		paths []dtypes.Path, truncated bool, err error)
}
