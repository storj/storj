// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	p "storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage"
	pb "storj.io/storj/protos/pointerdb"
)

var (
	mon = monkit.Package()
)

// PointerDB creates a grpcClient
type PointerDB struct {
	grpcClient pb.PointerDBClient
}

// a compiler trick to make sure *Overlay implements Client
var _ Client = (*PointerDB)(nil)

// Client services offerred for the interface
type Client interface {
	Put(ctx context.Context, path p.Path, pointer *pb.Pointer, APIKey []byte) error
	Get(ctx context.Context, path p.Path, APIKey []byte) (*pb.Pointer, error)
	List(ctx context.Context, prefix, startAfter, endBefore p.Path,
		recursive bool, limit int, metaFlags uint64, APIKey []byte) (
		items []storage.ListItem, more bool, err error)
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

// a compiler trick to make sure *PointerDB implements Client
var _ Client = (*PointerDB)(nil)

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

	_, err = pdb.grpcClient.Put(ctx, &pb.PutRequest{Path: path.String(), Pointer: pointer, APIKey: APIKey})

	return err
}

// Get is the interface to make a GET request, needs PATH and APIKey
func (pdb *PointerDB) Get(ctx context.Context, path p.Path, APIKey []byte) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := pdb.grpcClient.Get(ctx, &pb.GetRequest{Path: path.String(), APIKey: APIKey})
	if err != nil {
		return nil, err
	}

	pointer = &pb.Pointer{}
	err = proto.Unmarshal(res.GetPointer(), pointer)

	return pointer, nil
}

// List is the interface to make a LIST request, needs StartingPathKey, Limit, and APIKey
func (pdb *PointerDB) List(ctx context.Context, prefix, startAfter, endBefore p.Path,
	recursive bool, limit int, metaFlags uint64, APIKey []byte) (
	items []storage.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := pdb.grpcClient.List(ctx, &pb.ListRequest{
		Prefix:     prefix.String(),
		StartAfter: startAfter.String(),
		EndBefore:  endBefore.String(),
		Recursive:  recursive,
		Limit:      int32(limit),
		MetaFlags:  metaFlags,
		APIKey:     APIKey,
	})
	if err != nil {
		return nil, false, err
	}

	list := res.GetItems()
	items = make([]storage.ListItem, len(list))
	for i, itm := range list {
		modified, err := ptypes.Timestamp(itm.GetCreationDate())
		if err != nil {
			zap.S().Warnf("Failed converting creation date %v: %v", itm.GetCreationDate(), err)
		}
		expiration, err := ptypes.Timestamp(itm.GetExpirationDate())
		if err != nil {
			zap.S().Warnf("Failed converting expiration date %v: %v", itm.GetExpirationDate(), err)
		}
		items[i] = storage.ListItem{
			Path: p.New(string(itm.GetPath())),
			// TODO(kaloyan): we need to rethink how we return metadata through the layers
			Meta: storage.Meta{
				Modified:   modified,
				Expiration: expiration,
				Size:       itm.GetSize(),
				// TODO UserDefined: itm.GetMetadata(),
			},
		}
	}

	return items, res.GetMore(), nil
}

// Delete is the interface to make a Delete request, needs Path and APIKey
func (pdb *PointerDB) Delete(ctx context.Context, path p.Path, APIKey []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = pdb.grpcClient.Delete(ctx, &pb.DeleteRequest{Path: path.String(), APIKey: APIKey})

	return err
}
