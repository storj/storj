// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"github.com/golang/protobuf/proto"
	//"go.uber.org/zap"
	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/status"

	"storj.io/storj/pkg/netstate"
	pb "storj.io/storj/protos/netstate"
)

type NetState struct {
	grpcClient pb.NetStateClient
}

type NSClient interface {
	Put(ctx context.Context, path []byte, pointer *pb.Pointer, APIKey []byte) error
	Get(ctx context.Context, path []byte, APIKey []byte) (*pb.Pointer, error)
	Delete(ctx context.Context, path []byte, APIKey []byte) error
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

func (ns *NetState) Put(ctx context.Context, path []byte, pointer *pb.Pointer, APIKey []byte) error {
	_, err := ns.grpcClient.Put(ctx, &pb.PutRequest{Path: path, Pointer: pointer, APIKey: APIKey})
	if err != nil {
		return err
	}
	return nil
}


func (ns *NetState) Get(ctx context.Context, path []byte, APIKey []byte) (*pb.Pointer, error) {
	res, err := ns.grpcClient.Get(ctx, &pb.GetRequest{Path: path, APIKey: APIKey})
	if err != nil {
		return nil, err
	}

	pointer := &pb.Pointer{}
	err = proto.Unmarshal(res.GetPointer(),pointer)
	if err != nil {
		return nil, err
	}
	return pointer, nil
}

// func (ns *NetStateClient ) List()
// func (ns *NetStateClient ) Delete()