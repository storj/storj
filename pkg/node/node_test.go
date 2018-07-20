// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/test"
	proto "storj.io/storj/protos/overlay"
)

func TestLookup(t *testing.T) {
	cases := []struct {
		self             proto.Node
		to               proto.Node
		find             proto.Node
		expectedErr      error
		expectedNumNodes int
	}{
		{
			self:        proto.Node{Id: test.NewNodeID(t), Address: &proto.NodeAddress{Address: ":7070"}},
			to:          proto.Node{Id: test.NewNodeID(t), Address: &proto.NodeAddress{Address: ":8080"}},
			find:        proto.Node{Id: test.NewNodeID(t), Address: &proto.NodeAddress{Address: ":9090"}},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
		assert.NoError(t, err)

		srv, mock := newTestServer()
		go srv.Serve(lis)
		defer srv.Stop()

		nc, err := NewNodeClient(v.self)
		assert.NoError(t, err)

		_, err = nc.Lookup(context.Background(), v.to, v.find)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, 1, mock.queryCalled)
	}
}

func newTestServer() (*grpc.Server, *mockNodeServer) {
	grpcServer := grpc.NewServer()
	mn := &mockNodeServer{queryCalled: 0}

	proto.RegisterNodesServer(grpcServer, mn)

	return grpcServer, mn

}

type mockNodeServer struct {
	queryCalled int
}

func (mn *mockNodeServer) Query(ctx context.Context, req *proto.QueryRequest) (*proto.QueryResponse, error) {
	mn.queryCalled++
	return &proto.QueryResponse{}, nil
}
