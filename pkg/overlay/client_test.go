// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	proto "storj.io/storj/protos/overlay"
)

func TestNewOverlayClient(t *testing.T) {
	cases := []struct {
		address string
	}{
		{
			address: "127.0.0.1:8080",
		},
	}

	for _, v := range cases {
		oc, err := NewOverlayClient(v.address)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc.client)

	}
}

func TestChoose(t *testing.T) {
	cases := []struct {
		limit         int64
		space         int64
		expectedCalls int
	}{
		{
			limit:         50,
			space:         100,
			expectedCalls: 1,
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
		assert.NoError(t, err)

		srv, mock := NewTestServer()
		go srv.Serve(lis)
		defer srv.Stop()

		oc, err := NewOverlayClient(lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc.client)

		_, err = oc.Choose(context.Background(), v.limit, v.space)
		assert.NoError(t, err)
		assert.Equal(t, mock.FindStorageNodesCalled, v.expectedCalls)
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		nodeID        NodeID
		expectedCalls int
	}{
		{
			nodeID:        "foobar",
			expectedCalls: 1,
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
		assert.NoError(t, err)

		srv, mock := NewTestServer()
		go srv.Serve(lis)
		defer srv.Stop()

		oc, err := NewOverlayClient(lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc.client)

		_, err = oc.Lookup(context.Background(), v.nodeID)
		assert.NoError(t, err)
		assert.Equal(t, mock.lookupCalled, v.expectedCalls)
	}

}

func NewTestServer() (*grpc.Server, *mockOverlayServer) {
	grpcServer := grpc.NewServer()
	mo := &mockOverlayServer{lookupCalled: 0, FindStorageNodesCalled: 0}

	proto.RegisterOverlayServer(grpcServer, mo)

	return grpcServer, mo

}

type mockOverlayServer struct {
	lookupCalled           int
	FindStorageNodesCalled int
}

func (o *mockOverlayServer) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	o.lookupCalled++
	return &proto.LookupResponse{}, nil
}

func (o *mockOverlayServer) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	o.FindStorageNodesCalled++
	return &proto.FindStorageNodesResponse{}, nil
}
