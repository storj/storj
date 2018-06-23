// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"
//	"fmt"

	"google.golang.org/grpc"
	"github.com/golang/protobuf/proto"
	//"go.uber.org/zap"
	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/status"

	//"storj.io/storj/pkg/netstate"
	pb "storj.io/storj/protos/netstate"
)

type NetState struct {
	grpcClient pb.NetStateClient
}

type NSClient interface {
	Put(ctx context.Context, path []byte, pointer *pb.Pointer, APIKey []byte) error
	Get(ctx context.Context, path []byte, APIKey []byte) (*pb.Pointer, error)
	List(ctx context.Context, startingPathKey []byte, limit int64, APIKey []byte) (
		paths []byte, truncated bool, err error)
	Delete(ctx context.Context, path []byte, APIKey []byte) error
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

func (ns *NetState) List(ctx context.Context, startingPathKey []byte, limit int64, APIKey []byte) (
	paths [][]byte, truncated bool, err error) {
	res, err := ns.grpcClient.List(ctx, &pb.ListRequest{StartingPathKey: startingPathKey, Limit: limit, APIKey: APIKey})

	if err != nil {
		return nil, false, err
	 } 
	return res.Paths, true, nil
}

// func (ns *NetState) Delete(ctx context.Context, path []byte, APIKey []byte) error {
// 	_, err := ns.grpcClient.Delete(ctx, &pb.DeleteRequest{Path: path, APIKey: APIKey})

// 	if err != nil {
// 		return err
// 	 } 
// 	 return nil
// }

func (ns *NetState) Delete(ctx context.Context, path []byte, APIKey []byte) error {
	err := ns.grpcClient.Delete(ctx, &pb.DeleteRequest{Path: path, APIKey: APIKey})
	
	if err != nil {
		return err
	}
	return nil
}