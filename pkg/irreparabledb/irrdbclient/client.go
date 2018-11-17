// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irrdbclient

import (
	"context"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	pb "storj.io/storj/pkg/irreparabledb/proto"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

var (
	mon = monkit.Package()
)

// StatDB creates a grpcClient
type IrreparableDB struct {
	client pb.IrreparableDBClient
	APIKey []byte
}

// Client services offerred for the interface
type Client interface {
	Create(ctx context.Context, rmtsegkey []byte, rmtsegval []byte) error
}

// NewClient initializes a new irreparabledb client
func NewClient(identity *provider.FullIdentity, address string, APIKey []byte) (Client, error) {
	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(context.Background(), address)
	if err != nil {
		return nil, err
	}

	return &IrreparableDB{
		client: pb.NewIrreparableDBClient(conn),
		APIKey: APIKey,
	}, nil
}

// a compiler trick to make sure *IrrreparableDB implements Client
var _ Client = (*IrreparableDB)(nil)

// Create is used for creating a new entry in the irreparable db
func (irrdb *IrreparableDB) Create(ctx context.Context, rmtsegkey []byte, rmtsegval []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	// rmtseginfo := pb.RmtSegInfo{
	// 	RmtSegKey: rmtsegkey,
	// 	RmtSegVal: rmtsegval,
	// }
	createReq := &pb.CreateRequest{
		//RmtsegInfo: &rmtseginfo,
		APIKey: irrdb.APIKey,
	}
	_, err = irrdb.client.Create(ctx, createReq)

	return err
}
