// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
)

func TestNewServer(t *testing.T) {
	t.Skip("flaky")

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	srv := newMockServer()
	assert.NotNil(t, srv)

	// TODO: figure out why the error here fails
	go func() {
		assert.NoError(t, srv.Serve(lis))
	}()
	srv.Stop()
}

func newMockServer(opts ...grpc.ServerOption) *grpc.Server {
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterOverlayServer(grpcServer, &TestMockOverlay{})

	return grpcServer
}

type TestMockOverlay struct{}

func (o *TestMockOverlay) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (*pb.FindStorageNodesResponse, error) {
	return &pb.FindStorageNodesResponse{}, nil
}

func (o *TestMockOverlay) Lookup(ctx context.Context, req *pb.LookupRequest) (*pb.LookupResponse, error) {
	return &pb.LookupResponse{}, nil
}

func (o *TestMockOverlay) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (*pb.LookupResponses, error) {
	return &pb.LookupResponses{}, nil
}

func TestNewServerNilArgs(t *testing.T) {

	server := NewServer(nil, nil, nil, nil)

	assert.NotNil(t, server)
}
