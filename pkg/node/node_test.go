// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"fmt"
	"net"
	"testing"

	"storj.io/storj/pkg/dht/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

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
			self:        proto.Node{Id: "hello", Address: &proto.NodeAddress{Address: ":7070"}},
			to:          proto.Node{Id: "hello", Address: &proto.NodeAddress{Address: ":8080"}},
			find:        proto.Node{Id: "hello", Address: &proto.NodeAddress{Address: ":9090"}},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
		assert.NoError(t, err)

		srv, mock := newTestServer()
		go srv.Serve(lis)
		defer srv.Stop()
		ctrl := gomock.NewController(t)

		mdht := mock_dht.NewMockDHT(ctrl)
		mrt := mock_dht.NewMockRoutingTable(ctrl)
		nc, err := NewNodeClient(v.self, mdht)
		assert.NoError(t, err)

		mdht.EXPECT().GetRoutingTable(gomock.Any()).Return(mrt, nil)
		mrt.EXPECT().ConnectionSuccess(gomock.Any()).Return(nil)
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
