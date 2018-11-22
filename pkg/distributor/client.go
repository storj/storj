// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package distributor

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
var Error = errs.Class("distributor client error")

// DistributorClient DistributorClient
type DistributorClient struct {
	client pb.DistributorClient
}

// NewDistributorClient initializes a new distributor client
func NewDistributorClient(identity *provider.FullIdentity, address string, APIKey string) (*DistributorClient, error) {
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

	return &DistributorClient{client: pb.NewDistributorClient(conn)}, nil
}

func (c *DistributorClient) PutInfo(ctx context.Context, amount int32, space int64, excluded []string) (res *pb.PutInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return c.client.PutInfo(ctx, &pb.PutInfoRequest{Amount: amount, Space: space, Excluded: excluded})
}

func (c *DistributorClient) GetInfo(ctx context.Context, path storj.Path) (res *pb.GetInfoResponse, err error) {
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