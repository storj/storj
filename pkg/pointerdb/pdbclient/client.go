// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pdbclient

import (
	"context"
	"encoding/base64"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/auth/grpcauth"
	p "storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()
)

// PointerDB creates a grpcClient
type PointerDB struct {
	grpcClient      pb.PointerDBClient
	signatureHeader *metadata.MD
	peer            *peer.Peer
}

// New Used as a public function
func New(gcclient pb.PointerDBClient) (pdbc *PointerDB) {
	return &PointerDB{grpcClient: gcclient}
}

// a compiler trick to make sure *Overlay implements Client
var _ Client = (*PointerDB)(nil)

// ListItem is a single item in a listing
type ListItem struct {
	Path     p.Path
	Pointer  *pb.Pointer
	IsPrefix bool
}

// Client services offerred for the interface
type Client interface {
	Put(ctx context.Context, path p.Path, pointer *pb.Pointer) error
	Get(ctx context.Context, path p.Path) (*pb.Pointer, error)
	List(ctx context.Context, prefix, startAfter, endBefore p.Path,
		recursive bool, limit int, metaFlags uint32) (
		items []ListItem, more bool, err error)
	Delete(ctx context.Context, path p.Path) error
}

// NewClient initializes a new pointerdb client
func NewClient(identity *provider.FullIdentity, address string, APIKey string) (*PointerDB, error) {
	dialOpt, err := identity.DialOption()
	if err != nil {
		return nil, err
	}

	signatureHeader := &metadata.MD{}
	peer := &peer.Peer{}
	apiKeyInjector := grpcauth.NewAPIKeyInjector(APIKey, grpc.Header(signatureHeader), grpc.Peer(peer))
	c, err := clientConnection(address, dialOpt, grpc.WithUnaryInterceptor(apiKeyInjector))

	if err != nil {
		return nil, err
	}
	return &PointerDB{grpcClient: c, signatureHeader: signatureHeader, peer: peer}, nil
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
func (pdb *PointerDB) Put(ctx context.Context, path p.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = pdb.grpcClient.Put(ctx, &pb.PutRequest{Path: path.String(), Pointer: pointer})

	return err
}

// Get is the interface to make a GET request, needs PATH and APIKey
func (pdb *PointerDB) Get(ctx context.Context, path p.Path) (pointer *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := pdb.grpcClient.Get(ctx, &pb.GetRequest{Path: path.String()})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, Error.Wrap(err)
	}

	return res.GetPointer(), nil
}

// List is the interface to make a LIST request, needs StartingPathKey, Limit, and APIKey
func (pdb *PointerDB) List(ctx context.Context, prefix, startAfter, endBefore p.Path,
	recursive bool, limit int, metaFlags uint32) (
	items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := pdb.grpcClient.List(ctx, &pb.ListRequest{
		Prefix:     prefix.String(),
		StartAfter: startAfter.String(),
		EndBefore:  endBefore.String(),
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
			Path:     p.New(itm.GetPath()),
			Pointer:  itm.GetPointer(),
			IsPrefix: itm.IsPrefix,
		}
	}

	return items, res.GetMore(), nil
}

// Delete is the interface to make a Delete request, needs Path and APIKey
func (pdb *PointerDB) Delete(ctx context.Context, path p.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = pdb.grpcClient.Delete(ctx, &pb.DeleteRequest{Path: path.String()})

	return err
}

// Auth gets signature auth data from last request
func (pdb *PointerDB) Auth() (*pb.SignatureAuth, error) {
	signature := pdb.signatureHeader.Get("signature")
	if signature == nil {
		return nil, nil
	}

	base64 := base64.StdEncoding
	decodedSignature, err := base64.DecodeString(strings.Join(signature, ""))
	if err != nil {
		return nil, err
	}
	identity, err := provider.PeerIdentityFromPeer(pdb.peer)
	if err != nil {
		return nil, err
	}

	return auth.NewSignatureAuth(decodedSignature, identity)
}
