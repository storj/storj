// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage/redis/redisserver"
)

type mockNodeID struct {
}

func (m mockNodeID) String() string {
	return "foobar"
}

func (m mockNodeID) Bytes() []byte {
	return []byte("foobar")
}

func TestNewOverlayClient(t *testing.T) {
	cases := []struct {
		address string
	}{
		{
			address: "127.0.0.1:8080",
		},
	}

	for _, v := range cases {
		ca, err := provider.NewCA(ctx, 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := NewOverlayClient(identity, v.address)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc.client)

	}
}

func TestChoose(t *testing.T) {
	cases := []struct {
		limit         int
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
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)

		srv, mock, err := newTestServer(ctx)
		assert.NoError(t, err)
		go func() { assert.NoError(t, srv.Serve(lis)) }()
		defer srv.Stop()

		ca, err := provider.NewCA(ctx, 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := NewOverlayClient(identity, lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc.client)

		_, err = oc.Choose(ctx, v.limit, v.space)
		assert.NoError(t, err)
		assert.Equal(t, mock.FindStorageNodesCalled, v.expectedCalls)
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		nodeID        dht.NodeID
		expectedCalls int
	}{
		{
			nodeID:        mockNodeID{},
			expectedCalls: 1,
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)

		srv, mock, err := newTestServer(ctx)
		assert.NoError(t, err)
		go func() { assert.NoError(t, srv.Serve(lis)) }()
		defer srv.Stop()

		ca, err := provider.NewCA(ctx, 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := NewOverlayClient(identity, lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc.client)

		_, err = oc.Lookup(ctx, v.nodeID)
		assert.NoError(t, err)
		assert.Equal(t, mock.lookupCalled, v.expectedCalls)
	}

}
func TestBulkLookup(t *testing.T) {
	cases := []struct {
		nodeIDs       []dht.NodeID
		expectedCalls int
	}{
		{
			nodeIDs:       []dht.NodeID{mockNodeID{}, mockNodeID{}, mockNodeID{}},
			expectedCalls: 1,
		},
	}
	for _, v := range cases {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)

		srv, mock, err := newTestServer(ctx)
		assert.NoError(t, err)
		go func() { assert.NoError(t, srv.Serve(lis)) }()
		defer srv.Stop()

		ca, err := provider.NewCA(ctx, 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := NewOverlayClient(identity, lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		assert.NotEmpty(t, oc.client)

		_, err = oc.BulkLookup(ctx, v.nodeIDs)
		assert.NoError(t, err)
		assert.Equal(t, mock.bulkLookupCalled, v.expectedCalls)
	}
}
func TestBulkLookupV2(t *testing.T) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	srv, s, err := newServer(ctx, redisAddr)

	assert.NoError(t, err)
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	ca, err := provider.NewCA(ctx, 12, 4)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	oc, err := NewOverlayClient(identity, lis.Addr().String())
	assert.NoError(t, err)

	assert.NotNil(t, oc)
	assert.NotEmpty(t, oc.client)
	n1 := &proto.Node{Id: "n1"}
	n2 := &proto.Node{Id: "n2"}
	n3 := &proto.Node{Id: "n3"}
	nodes := []*proto.Node{n1, n2, n3}
	for _, n := range nodes {
		assert.NoError(t, s.cache.Put(n.Id, *n))
	}

	cases := []struct {
		testID    string
		nodeIDs   []dht.NodeID
		responses []*proto.Node
		errors    *errs.Class
	}{
		{testID: "empty id",
			nodeIDs:   []dht.NodeID{},
			responses: nil,
			errors:    &ClientError,
		},
		{testID: "valid ids",
			nodeIDs: func() []dht.NodeID {
				id1 := kademlia.StringToNodeID("n1")
				id2 := kademlia.StringToNodeID("n2")
				id3 := kademlia.StringToNodeID("n3")
				return []dht.NodeID{id1, id2, id3}
			}(),
			responses: nodes,
			errors:    nil,
		},
		{testID: "missing ids",
			nodeIDs: func() []dht.NodeID {
				id1 := kademlia.StringToNodeID("n4")
				id2 := kademlia.StringToNodeID("n5")
				return []dht.NodeID{id1, id2}
			}(),
			responses: []*proto.Node{nil, nil},
			errors:    nil,
		},
		{testID: "random order and nil",
			nodeIDs: func() []dht.NodeID {
				id1 := kademlia.StringToNodeID("n1")
				id2 := kademlia.StringToNodeID("n2")
				id3 := kademlia.StringToNodeID("n3")
				id4 := kademlia.StringToNodeID("n4")
				return []dht.NodeID{id2, id1, id3, id4}
			}(),
			responses: func() []*proto.Node {
				return []*proto.Node{nodes[1], nodes[0], nodes[2], nil}
			}(),
			errors: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ns, err := oc.BulkLookup(ctx, c.nodeIDs)
			assertErrClass(t, c.errors, err)
			assert.Equal(t, c.responses, ns)
		})
	}
}

func newServer(ctx context.Context, redisAddr string) (*grpc.Server, *Server, error) {
	ca, err := provider.NewCA(ctx, 12, 4)
	if err != nil {
		return nil, nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, nil, err
	}
	identOpt, err := identity.ServerOption()
	if err != nil {
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(identOpt)
	cache, err := NewRedisOverlayCache(redisAddr, "", 1, nil)
	if err != nil {
		return nil, nil, err
	}
	s := &Server{cache: cache}

	proto.RegisterOverlayServer(grpcServer, s)

	return grpcServer, s, nil
}

func newTestServer(ctx context.Context) (*grpc.Server, *mockOverlayServer, error) {
	ca, err := provider.NewCA(ctx, 12, 4)
	if err != nil {
		return nil, nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, nil, err
	}
	identOpt, err := identity.ServerOption()
	if err != nil {
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(identOpt)
	mo := &mockOverlayServer{lookupCalled: 0, FindStorageNodesCalled: 0}

	proto.RegisterOverlayServer(grpcServer, mo)

	return grpcServer, mo, nil

}

type mockOverlayServer struct {
	lookupCalled           int
	bulkLookupCalled       int
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

func (o *mockOverlayServer) BulkLookup(ctx context.Context, reqs *proto.LookupRequests) (*proto.LookupResponses, error) {
	o.bulkLookupCalled++
	return &proto.LookupResponses{}, nil
}
