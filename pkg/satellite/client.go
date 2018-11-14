// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
)

// Error is the pdbclient error class
var Error = errs.Class("satellite client error")

// SatelliteClient SatelliteClient
type SatelliteClient struct {
	client pb.SatelliteClient
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     storj.Path
	Pointer  *pb.Pointer
	IsPrefix bool
}

// NewSatelliteClient initializes a new satellite client
func NewSatelliteClient(identity *provider.FullIdentity, address string, APIKey string) (*SatelliteClient, error) {
	apiKeyInjector := grpcauth.NewAPIKeyInjector(APIKey)
	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(
		context.Background(),
		address,
		grpc.WithUnaryInterceptor(apiKeyInjector),
	)
	if err != nil {
		return nil, err
	}

	return &SatelliteClient{client: pb.NewSatelliteClient(conn)}, nil
}

func (c *SatelliteClient) PutInfo(ctx context.Context, amount int32, space int64, excluded []string) (res *pb.PutInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return c.client.PutInfo(ctx, &pb.PutInfoRequest{Amount: amount, Space: space, Excluded: excluded})
}

func (c *SatelliteClient) GetInfo(ctx context.Context, path storj.Path) (res *pb.GetInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	response, err := c.client.GetInfo(ctx, &pb.GetInfoRequest{Path: path})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, storage.ErrKeyNotFound.Wrap(err)
		}
		return nil, Error.Wrap(err)
	}

	return response, nil
}

func (c *SatelliteClient) PutMeta(ctx context.Context, path storj.Path, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = c.client.PutMeta(ctx, &pb.PutRequest{Path: path, Pointer: pointer})
	if err != nil {
		return err
	}
	return nil
}

func (c *SatelliteClient) DeleteMeta(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = c.client.DeleteMeta(ctx, &pb.DeleteRequest{Path: path})
	if err != nil {
		return err
	}
	return nil
}

func (c *SatelliteClient) List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := c.client.ListMeta(ctx, &pb.ListRequest{
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
