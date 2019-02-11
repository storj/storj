// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pdbclient

import (
	"context"
	"sync/atomic"
	"unsafe"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()
)

// PointerDB creates a grpcClient
type PointerDB struct {
	client        pb.PointerDBClient
	authorization unsafe.Pointer // *pb.SignedMessage
}

// New Used as a public function
func New(gcclient pb.PointerDBClient) (pdbc *PointerDB) {
	return &PointerDB{client: gcclient}
}

// a compiler trick to make sure *Overlay implements Client
var _ Client = (*PointerDB)(nil)

// ListItem is a single item in a listing
type ListItem struct {
	Path     storj.Path
	Pointer  *pb.Pointer
	IsPrefix bool
}

// Client services offerred for the interface
type Client interface {
	Put(ctx context.Context, path storj.Path, pointer *pb.Pointer) error
	Get(ctx context.Context, path storj.Path) (*pb.Pointer, []*pb.Node, *pb.PayerBandwidthAllocation, error)
	List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
	Delete(ctx context.Context, path storj.Path) error

	SignedMessage() *pb.SignedMessage
	PayerBandwidthAllocation(context.Context, pb.BandwidthAction) (*pb.PayerBandwidthAllocation, error)

	// Disconnect() error // TODO: implement
}

// NewClient initializes a new pointerdb client
func NewClient(tc transport.Client, address string, APIKey string) (*PointerDB, error) {
	return NewClientContext(context.TODO(), tc, address, APIKey)
}

// NewClientContext initializes a new pointerdb client
func NewClientContext(ctx context.Context, tc transport.Client, address string, APIKey string) (*PointerDB, error) {
	apiKeyInjector := grpcauth.NewAPIKeyInjector(APIKey)
	conn, err := tc.DialAddress(
		ctx,
		address,
		grpc.WithUnaryInterceptor(apiKeyInjector),
	)
	if err != nil {
		return nil, err
	}

	return &PointerDB{client: pb.NewPointerDBClient(conn)}, nil
}

// a compiler trick to make sure *PointerDB implements Client
var _ Client = (*PointerDB)(nil)

// Put is the interface to make a PUT request, needs Pointer and APIKey
func (pdb *PointerDB) Put(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = pdb.client.Put(ctx, &pb.PutRequest{Path: path, Pointer: pointer})

	return err
}

// Get is the interface to make a GET request, needs PATH and APIKey
func (pdb *PointerDB) Get(ctx context.Context, path storj.Path) (pointer *pb.Pointer, nodes []*pb.Node, pba *pb.PayerBandwidthAllocation, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := pdb.client.Get(ctx, &pb.GetRequest{Path: path})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil, nil, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, nil, nil, Error.Wrap(err)
	}

	atomic.StorePointer(&pdb.authorization, unsafe.Pointer(res.GetAuthorization()))

	if res.GetPointer().GetType() == pb.Pointer_INLINE {
		return res.GetPointer(), nodes, res.GetPba(), nil
	}

	pieces := res.GetPointer().GetRemote().GetRemotePieces()
	nodes = make([]*pb.Node, len(pieces))

	// fill missing nodes with nil values to match the size and order of remote pieces
	j := 0
	for i := 0; i < len(pieces); i++ {
		if j == len(res.GetNodes()) {
			break
		}
		if pieces[i].NodeId == res.GetNodes()[j].Id {
			nodes[i] = res.GetNodes()[j]
			j++
		}
	}

	return res.GetPointer(), nodes, res.GetPba(), nil
}

// List is the interface to make a LIST request, needs StartingPathKey, Limit, and APIKey
func (pdb *PointerDB) List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := pdb.client.List(ctx, &pb.ListRequest{
		Prefix:     prefix,
		StartAfter: startAfter,
		EndBefore:  endBefore,
		Recursive:  recursive,
		Limit:      int32(limit),
		MetaFlags:  metaFlags,
	})
	if err != nil {
		return nil, false, err
	}

	list := res.GetItems()
	items = make([]ListItem, len(list))
	for i, itm := range list {
		items[i] = ListItem{
			Path:     itm.GetPath(),
			Pointer:  itm.GetPointer(),
			IsPrefix: itm.IsPrefix,
		}
	}

	return items, res.GetMore(), nil
}

// Delete is the interface to make a Delete request, needs Path and APIKey
func (pdb *PointerDB) Delete(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = pdb.client.Delete(ctx, &pb.DeleteRequest{Path: path})

	return err
}

// PayerBandwidthAllocation gets payer bandwidth allocation message
func (pdb *PointerDB) PayerBandwidthAllocation(ctx context.Context, action pb.BandwidthAction) (resp *pb.PayerBandwidthAllocation, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := pdb.client.PayerBandwidthAllocation(ctx, &pb.PayerBandwidthAllocationRequest{Action: action})
	if err != nil {
		return nil, err
	}
	return response.GetPba(), nil
}

// SignedMessage gets signed message from last request
func (pdb *PointerDB) SignedMessage() *pb.SignedMessage {
	return (*pb.SignedMessage)(atomic.LoadPointer(&pdb.authorization))
}
