// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/test"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

func TestNewServer(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv := newMockServer()
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv := newMockServer()
	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestProcess(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	o := Service{}
	ctx, cf := context.WithTimeout(context.Background(), 3*time.Second)
	defer cf()
	err := o.Process(ctx)
	assert.NoError(t, err)
}

func newMockServer() *grpc.Server {
	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &MockOverlay{})

	return grpcServer
}

type MockOverlay struct{}

func (o *MockOverlay) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	return &proto.FindStorageNodesResponse{}, nil
}

func (o *MockOverlay) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	return &proto.LookupResponse{}, nil
}
