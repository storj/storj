// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/test"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

func TestFindStorageNodes(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	id, err := node.NewID()
	assert.NoError(t, err)
	id2, err := node.NewID()
	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(id.String()),
			Value: test.NewNodeStorageValue(t, "127.0.0.1:9090"),
		}, {
			Key:   storage.Key(id2.String()),
			Value: test.NewNodeStorageValue(t, "127.0.0.1:9090"),
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

	id, err := node.NewID()

	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(id.String()),
			Value: test.NewNodeStorageValue(t, "127.0.0.1:9090"),
		},
	})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &pb.LookupRequest{NodeID: id.String()})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestOverlayBulkLookup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	id, err := node.NewID()
	assert.NoError(t, err)
	id2, err := node.NewID()
	assert.NoError(t, err)

	srv := NewMockServer([]storage.ListItem{
		{
			Key:   storage.Key(id.String()),
			Value: test.NewNodeStorageValue(t, "127.0.0.1:9090"),
		},
	})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(address, grpc.WithInsecure())
	assert.NoError(t, err)

	req1 := &pb.LookupRequest{NodeID: id.String()}
	req2 := &pb.LookupRequest{NodeID: id2.String()}
	rs := &pb.LookupRequests{Lookuprequest: []*pb.LookupRequest{req1, req2}}
	r, err := c.BulkLookup(context.Background(), rs)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}
