// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"

	"google.golang.org/grpc"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/netstate"
	pb "storj.io/storj/protos/netstate"
)

type NetState struct {
	grpcClient pb.NetStateClient
}

type NSClient interface {
	Put(ctx context.Context, path netstate.Path, pointer *pb.Pointer, APIKey []byte) error
	Get(ctx context.Context, path netstate.Path, APIKey []byte) (*pb.Pointer, error)
	Delete(ctx context.Context, path netstate.Path, APIKey []byte) error
	List(ctx context.Context, startingPath, endingPath netstate.Path, APIKey []byte) (
		path netstate.Path, truncated bool, err error)
}

func NewNetstateClient(address string) (*NetState, error) {
    c, err := NewClient(&address, grpc.WithInsecure())
    if err != nil {
        return nil, err
    }
    return &NetState{
        grpcClient: c,
    }, nil
}

func NewClient(serverAddr *string, opts ...grpc.DialOption) (pb.NetStateClient, error) {
    conn, err := grpc.Dial(*serverAddr, opts...)
    if err != nil {
        return nil, err
    }
    return pb.NewNetStateClient(conn), nil
}

func (ns *NetState) Put(ctx context.Context, path netstate.Path, pointer *pb.Pointer, APIKey []byte) error {
	resp, err := ns.grpcClient.Put(ctx, path, pointer, APIKey)
	if err != nil {
		logger.Error("Failed to make a PUT request ", zap.Error(err))
		return err
	}
	return status.Errorf(codes.Internal, err.Error())
}



// func (ns *NetStateClient ) Get()
// func (ns *NetStateClient ) List()
// func (ns *NetStateClient ) Delete()