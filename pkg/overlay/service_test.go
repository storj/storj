// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

func TestNewServer(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	srv := newMockServer()
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func newMockServer(opts ...grpc.ServerOption) *grpc.Server {
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterOverlayServer(grpcServer, &TestMockOverlay{})

	return grpcServer
}

type TestMockOverlay struct{}

func (o *TestMockOverlay) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	return &proto.FindStorageNodesResponse{}, nil
}

func (o *TestMockOverlay) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	return &proto.LookupResponse{}, nil
}

func (o *TestMockOverlay) BulkLookup(ctx context.Context, reqs *proto.LookupRequests) (*proto.LookupResponses, error) {
	return &proto.LookupResponses{}, nil
}

func TestNewServerNilArgs(t *testing.T) {

	server := NewServer(nil, nil, nil, nil)

	assert.NotNil(t, server)
}
