// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"

	pb "storj.io/storj/protos/netstate"
	p "storj.io/storj/pkg/paths"
)

// NetState creates a grpcClient
type NetState struct {
	grpcClient pb.NetStateClient
}

// NSClient services offerred for the interface
type NSClient interface {
	Put(ctx context.Context, path p.Path, pointer *pb.Pointer, APIKey []byte) error
	Get(ctx context.Context, path p.Path, APIKey []byte) (*pb.Pointer, error)
	List(ctx context.Context, startingPathKey []byte, limit int64, APIKey []byte) (
		paths []byte, truncated bool, err error)
	Delete(ctx context.Context, path p.Path, APIKey []byte) error
}

// NewNetstateClient initializes a new netstate client
func NewNetstateClient(address string) (*NetState, error) {
	c, err := NewClient(&address, grpc.WithInsecure())

	if err != nil {
		return nil, err
	}
	return &NetState{
		grpcClient: c,
	}, nil
}

// NewClient makes a server connection
func NewClient(serverAddr *string, opts ...grpc.DialOption) (pb.NetStateClient, error) {
	conn, err := grpc.Dial(*serverAddr, opts...)

	if err != nil {
		return nil, err
	}
	return pb.NewNetStateClient(conn), nil
}

// Put is the interface to make a PUT request, needs Pointer and APIKey
func (ns *NetState) Put(ctx context.Context, path p.Path, pointer *pb.Pointer, APIKey []byte) error {
	_, err := ns.grpcClient.Put(ctx, &pb.PutRequest{Path: path.Bytes(), Pointer: pointer, APIKey: APIKey})

	if err != nil {
		return err
	}
	return nil
}

// Get is the interface to make a GET request, needs PATH and APIKey
func (ns *NetState) Get(ctx context.Context, path p.Path, APIKey []byte) (*pb.Pointer, error) {
	res, err := ns.grpcClient.Get(ctx, &pb.GetRequest{Path: path.Bytes(), APIKey: APIKey})

	if err != nil {
		return nil, err
	}

	pointer := &pb.Pointer{}
	err = proto.Unmarshal(res.GetPointer(), pointer)

	if err != nil {
		return nil, err
	}
	return pointer, nil
}

// List is the interface to make a LIST request, needs StartingPathKey, Limit, and APIKey
func (ns *NetState) List(ctx context.Context, startingPathKey p.Path, limit int64, APIKey []byte) (
	paths [][]byte, truncated bool, err error) {
	res, err := ns.grpcClient.List(ctx, &pb.ListRequest{StartingPathKey: startingPathKey.Bytes(), Limit: limit, APIKey: APIKey})

	if err != nil {
		return nil, res.Truncated, err
	}
	
	return res.Paths, res.Truncated, nil
}

// Delete is the interface to make a Delete request, needs Path and APIKey
func (ns *NetState) Delete(ctx context.Context, path p.Path, APIKey []byte) error {
	_, err := ns.grpcClient.Delete(ctx, &pb.DeleteRequest{Path: path.Bytes(), APIKey: APIKey})

	if err != nil {
		return err
	}
	return nil
}
