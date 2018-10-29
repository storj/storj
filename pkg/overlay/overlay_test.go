// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

func TestFindStorageNodes(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	fid, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)
	fid2, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(fid.ID.String()),
			Value: newNodeStorageValue(t, "127.0.0.1:9090"),
		}, {
			Key:   storage.Key(fid2.ID.String()),
			Value: newNodeStorageValue(t, "127.0.0.1:9090"),
		},
	})
	assert.NotNil(t, srv)

	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.FindStorageNodes(context.Background(), &pb.FindStorageNodesRequest{Opts: &pb.OverlayOptions{Amount: 2}})
	assert.NoError(t, err)
	assert.NotNil(t, r)

	assert.Len(t, r.Nodes, 2)
}

func TestOverlayLookup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	fid, err := node.NewFullIdentity(ctx, 12, 4)

	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(fid.ID.String()),
			Value: newNodeStorageValue(t, "127.0.0.1:9090"),
		},
	})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &pb.LookupRequest{NodeID: fid.ID.String()})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestOverlayBulkLookup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	fid, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)
	fid2, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(fid.ID.String()),
			Value: newNodeStorageValue(t, "127.0.0.1:9090"),
		},
	})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(address, grpc.WithInsecure())
	assert.NoError(t, err)

	req1 := &pb.LookupRequest{NodeID: fid.ID.String()}
	req2 := &pb.LookupRequest{NodeID: fid2.ID.String()}
	rs := &pb.LookupRequests{Lookuprequest: []*pb.LookupRequest{req1, req2}}
	r, err := c.BulkLookup(context.Background(), rs)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

// newNodeStorageValue provides a convient way to create a node as a storage.Value for testing purposes
func newNodeStorageValue(t *testing.T, address string) storage.Value {
	na := &pb.Node{Id: "", Address: &pb.NodeAddress{Transport: pb.NodeTransport_TCP_TLS_GRPC, Address: address}}
	d, err := proto.Marshal(na)
	assert.NoError(t, err)
	return d
}
