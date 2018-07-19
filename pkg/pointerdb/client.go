// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	p "storj.io/storj/pkg/paths"
	pb "storj.io/storj/protos/pointerdb"
)

var (
	mon = monkit.Package()
)

// PointerDB creates a grpcClient
type PointerDB struct {
	grpcClient pb.PointerDBClient
}

// Client services offerred for the interface
type Client interface {
	Put(ctx context.Context, path p.Path, pointer *pb.Pointer, APIKey []byte) error
	Get(ctx context.Context, path p.Path, APIKey []byte) (*pb.Pointer, error)
	List(ctx context.Context, startingPathKey []byte, limit int64, APIKey []byte) (
		paths []byte, truncated bool, err error)
	Delete(ctx context.Context, path p.Path, APIKey []byte) error
}

// NewClient initializes a new pointerdb client
func NewClient(address string) (*PointerDB, error) {
	c, err := clientConnection(address, grpc.WithInsecure())

	if err != nil {
		return nil, err
	}
	return &PointerDB{
		grpcClient: c,
	}, nil
}

// ClientConnection makes a server connection
func clientConnection(serverAddr string, opts ...grpc.DialOption) (pb.PointerDBClient, error) {
	conn, err := grpc.Dial(serverAddr, opts...)

	if err != nil {
		return nil, err
	}
	return pb.NewPointerDBClient(conn), nil
}

// Put is the interface to make a PUT request, needs Pointer and APIKey
func (pdb *PointerDB) Put(ctx context.Context, path p.Path, pointer *pb.Pointer, APIKey []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = pdb.grpcClient.Put(ctx, &pb.PutRequest{Path: path.Bytes(), Pointer: pointer, APIKey: APIKey})

	return err
}

// Get is the interface to make a GET request, needs PATH and APIKey
func (pdb *PointerDB) Get(ctx context.Context, path p.Path, APIKey []byte) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := pdb.grpcClient.Get(ctx, &pb.GetRequest{Path: path.Bytes(), APIKey: APIKey})
	if err != nil {
		return nil, err
	}

	pointer = &pb.Pointer{}
	err = proto.Unmarshal(res.GetPointer(), pointer)

	return pointer, nil
}

// List is the interface to make a LIST request, needs StartingPathKey, Limit, and APIKey
func (pdb *PointerDB) List(ctx context.Context, startingPathKey p.Path, limit int64, APIKey []byte) (paths [][]byte, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	res, err := pdb.grpcClient.List(ctx, &pb.ListRequest{StartingPathKey: startingPathKey.Bytes(), Limit: limit, APIKey: APIKey})

	if err != nil {
		return nil, false, err
	}

	return res.Paths, res.Truncated, nil
}

// Delete is the interface to make a Delete request, needs Path and APIKey
func (pdb *PointerDB) Delete(ctx context.Context, path p.Path, APIKey []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = pdb.grpcClient.Delete(ctx, &pb.DeleteRequest{Path: path.Bytes(), APIKey: APIKey})

	return err
}
