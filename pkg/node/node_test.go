// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/dht/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var ctx = context.Background()

func TestLookup(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cases := []struct {
		self        pb.Node
		to          pb.Node
		find        pb.Node
		expectedErr error
	}{
		{
			self:        pb.Node{Id: "hello", Address: &pb.NodeAddress{Address: ":7070"}},
			to:          pb.Node{Id: "hello", Address: &pb.NodeAddress{Address: ":8080"}},
			find:        pb.Node{Id: "hello", Address: &pb.NodeAddress{Address: ":9090"}},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)

		id := testidentity.NewTestIdentity(t)
		v.to = pb.Node{Id: id.ID.String(), Address: &pb.NodeAddress{Address: lis.Addr().String()}}

		srv, mock, err := newTestServer(ctx, &mockNodeServer{queryCalled: 0}, id)
		assert.NoError(t, err)

		ctx.Go(func() error { return srv.Serve(lis) })
		defer srv.Stop()

		ctrl := gomock.NewController(t)

		mdht := mock_dht.NewMockDHT(ctrl)
		mrt := mock_dht.NewMockRoutingTable(ctrl)

		mdht.EXPECT().GetRoutingTable(gomock.Any()).Return(mrt, nil)
		mrt.EXPECT().ConnectionSuccess(gomock.Any()).Return(nil)

		ca, err := provider.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		nc, err := NewNodeClient(identity, v.self, mdht)
		assert.NoError(t, err)

		_, err = nc.Lookup(ctx, v.to, v.find)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, 1, mock.(*mockNodeServer).queryCalled)
	}
}

func TestPing(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cases := []struct {
		self        pb.Node
		toID        string
		toIdentity  *provider.FullIdentity
		expectedErr error
	}{
		{
			self:        pb.Node{Id: "hello", Address: &pb.NodeAddress{Address: ":7070"}},
			toID:        "",
			toIdentity:  testidentity.NewTestIdentity(t),
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		// new mock DHT for node client
		ctrl := gomock.NewController(t)
		mdht := mock_dht.NewMockDHT(ctrl)
		// set up a node server
		srv := NewServer(mdht)

		msrv, _, err := newTestServer(ctx, srv, v.toIdentity)
		assert.NoError(t, err)
		// start gRPC server
		ctx.Go(func() error { return msrv.Serve(lis) })
		defer msrv.Stop()

		nc, err := NewNodeClient(v.toIdentity, v.self, mdht)
		assert.NoError(t, err)

		id := ID(v.toIdentity.ID)
		ok, err := nc.Ping(ctx, pb.Node{Id: id.String(), Address: &pb.NodeAddress{Address: lis.Addr().String()}})
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, ok, true)
	}
}

func newTestServer(ctx context.Context, ns pb.NodesServer, identity *provider.FullIdentity) (*grpc.Server, pb.NodesServer, error) {
	identOpt, err := identity.ServerOption()
	if err != nil {
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(identOpt)
	pb.RegisterNodesServer(grpcServer, ns)

	return grpcServer, ns, nil

}

type mockNodeServer struct {
	queryCalled int
	pingCalled  int
}

func (mn *mockNodeServer) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	mn.queryCalled++
	return &pb.QueryResponse{}, nil
}

func (mn *mockNodeServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	mn.pingCalled++
	return &pb.PingResponse{}, nil
}

func newTestIdentity(t *testing.T) *provider.FullIdentity {
	ca, err := provider.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	return identity
}
