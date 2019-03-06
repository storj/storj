// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	mon = monkit.Package()
)

// Metainfo creates a grpcClient
type Metainfo struct {
	client pb.MetainfoClient
}

// New Used as a public function
func New(gcclient pb.MetainfoClient) (metainfo *Metainfo) {
	return &Metainfo{client: gcclient}
}

// a compiler trick to make sure *Overlay implements Client
var _ Client = (*Metainfo)(nil)

// ListItem is a single item in a listing
type ListItem struct {
	Path     storj.Path
	Pointer  *pb.Pointer
	IsPrefix bool
}

// Client interface for the Metainfo service
type Client interface {
	CreateSegment(ctx context.Context, path storj.Path) (*[]pb.OrderLimit2, error)
	CommitSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) error
	ReadSegment(ctx context.Context, path storj.Path) (*pb.Pointer, *[]pb.OrderLimit2, error)
	DeleteSegment(ctx context.Context, path storj.Path) (*[]pb.OrderLimit2, error)
	ListSegments(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

// NewClient initializes a new metainfo client
func NewClient(ctx context.Context, tc transport.Client, address string, APIKey string) (*Metainfo, error) {
	apiKeyInjector := grpcauth.NewAPIKeyInjector(APIKey)
	conn, err := tc.DialAddress(
		ctx,
		address,
		grpc.WithUnaryInterceptor(apiKeyInjector),
	)
	if err != nil {
		return nil, err
	}

	return &Metainfo{client: pb.NewMetainfoClient(conn)}, nil
}

// a compiler trick to make sure *PointerDB implements Client
var _ Client = (*Metainfo)(nil)

// CreateSegment requests the order limits for creating a new segment
func (metainfo *Metainfo) CreateSegment(ctx context.Context, path storj.Path) (orders *[]pb.OrderLimit2, err error) {
	defer mon.Task()(&ctx)(&err)

	return nil, errors.New("not implemented")
}

// CommitSegment requests to store the pointer for the segment
func (metainfo *Metainfo) CommitSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	return errors.New("not implemented")
}

// ReadSegment requests the order limits for reading a segment
func (metainfo *Metainfo) ReadSegment(ctx context.Context, path storj.Path) (pointer *pb.Pointer, orders *[]pb.OrderLimit2, err error) {
	defer mon.Task()(&ctx)(&err)

	return nil, nil, errors.New("not implemented")
}

// DeleteSegment requests the order limits for deleting a segment
func (metainfo *Metainfo) DeleteSegment(ctx context.Context, path storj.Path) (orders *[]pb.OrderLimit2, err error) {
	defer mon.Task()(&ctx)(&err)

	return nil, errors.New("not implemented")
}

// ListSegments lists the available segments
func (metainfo *Metainfo) ListSegments(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	return nil, false, errors.New("not implemented")
}
