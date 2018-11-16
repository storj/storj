// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/redis/redisserver"
)

var emptyNode = storj.Node{}

type mockNodeID struct {
}

func (m mockNodeID) String() string {
	return "foobar"
}

func (m mockNodeID) Bytes() []byte {
	return []byte("foobar")
}

func (m mockNodeID) Difficulty() uint16 {
	return 12
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
		ca, err := provider.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := NewOverlayClient(identity, v.address)
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		overlay, ok := oc.(*Overlay)
		assert.True(t, ok)
		assert.NotEmpty(t, overlay.client)

	}
}

var (
	id1 = teststorj.NodeIDFromString("n1")
	id2 = teststorj.NodeIDFromString("n2")
	id3 = teststorj.NodeIDFromString("n3")
	id4 = teststorj.NodeIDFromString("n4")
	id5 = teststorj.NodeIDFromString("n5")
	id6 = teststorj.NodeIDFromString("n6")
	id7 = teststorj.NodeIDFromString("n7")
	id8 = teststorj.NodeIDFromString("n8")
	n1  = storj.NewNodeWithID(id1, &pb.Node{})
	n2  = storj.NewNodeWithID(id2, &pb.Node{})
	n3  = storj.NewNodeWithID(id3, &pb.Node{})
	n4  = storj.NewNodeWithID(id4, &pb.Node{})
	n5  = storj.NewNodeWithID(id5, &pb.Node{})
	n6  = storj.NewNodeWithID(id6, &pb.Node{})
	n7  = storj.NewNodeWithID(id7, &pb.Node{})
	n8  = storj.NewNodeWithID(id8, &pb.Node{})
)

func TestChoose(t *testing.T) {
	cases := []struct {
		limit    int
		space    int64
		allNodes []storj.Node
		excluded []storj.NodeID
	}{
		{
			limit: 4,
			space: 0,
			allNodes: []storj.Node{n1, n2, n3, n4, n5, n6, n7, n8},
			excluded: []storj.NodeID{id1, id2, id3, id4},
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)

		var listItems []storage.ListItem
		for _, n := range v.allNodes {
			data, err := proto.Marshal(n)
			assert.NoError(t, err)
			listItems = append(listItems, storage.ListItem{
				Key:   storage.Key(n.Id.Bytes()),
				Value: data,
			})
		}

		ca, err := provider.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		srv := NewMockServer(listItems, func() grpc.ServerOption {
			opt, err := identity.ServerOption()
			assert.NoError(t, err)
			return opt
		}())

		go func() { assert.NoError(t, srv.Serve(lis)) }()
		defer srv.Stop()

		oc, err := NewOverlayClient(identity, lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		overlay, ok := oc.(*Overlay)
		assert.True(t, ok)
		assert.NotEmpty(t, overlay.client)

		newNodes, err := oc.Choose(ctx, Options{Amount: v.limit, Space: v.space, Excluded: v.excluded})
		assert.NoError(t, err)
		for _, new := range newNodes {
			for _, ex := range v.excluded {
				assert.NotEqual(t, ex.String(), new.Id)
			}
		}
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		nodeID        storj.NodeID
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

		ca, err := provider.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := NewOverlayClient(identity, lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		overlay, ok := oc.(*Overlay)
		assert.True(t, ok)
		assert.NotEmpty(t, overlay.client)

		_, err = oc.Lookup(ctx, v.nodeID)
		assert.NoError(t, err)
		assert.Equal(t, mock.lookupCalled, v.expectedCalls)
	}

}
func TestBulkLookup(t *testing.T) {
	cases := []struct {
		nodeIDs       storj.NodeIDList
		expectedCalls int
	}{
		{
			nodeIDs:       storj.NodeIDList{mockNodeID{}, mockNodeID{}, mockNodeID{}},
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

		ca, err := provider.NewTestCA(ctx)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		oc, err := NewOverlayClient(identity, lis.Addr().String())
		assert.NoError(t, err)

		assert.NotNil(t, oc)
		overlay, ok := oc.(*Overlay)
		assert.True(t, ok)
		assert.NotEmpty(t, overlay.client)

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

	ca, err := provider.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	oc, err := NewOverlayClient(identity, lis.Addr().String())
	assert.NoError(t, err)

	assert.NotNil(t, oc)
	overlay, ok := oc.(*Overlay)
	assert.True(t, ok)
	assert.NotEmpty(t, overlay.client)
	nodes := []storj.Node{n1, n2, n3}
	for _, n := range nodes {
		assert.NoError(t, s.cache.Put(n))
	}

	cases := []struct {
		testID    string
		nodeIDs   storj.NodeIDList
		responses []storj.Node
		errors    *errs.Class
	}{
		{testID: "empty id",
			nodeIDs:   storj.NodeIDList{},
			responses: nil,
			errors:    &ClientError,
		},
		{testID: "empty id",
			nodeIDs:   storj.NodeIDList{storj.EmptyNodeID},
			responses: []storj.Node{emptyNode},
			errors:    nil,
		},
		{testID: "valid ids",
			nodeIDs: storj.NodeIDList{id1, id2, id3},
			responses: nodes,
			errors:    nil,
		},
		{testID: "missing ids",
			nodeIDs: storj.NodeIDList{id4, id5},
			responses: []storj.Node{emptyNode, emptyNode},
			errors:    nil,
		},
		{testID: "random order and nil",
			nodeIDs: storj.NodeIDList{id2, id1, id3, id4},
			responses: []storj.Node{nodes[1], nodes[0], nodes[2], emptyNode},
			errors: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ns, err := oc.BulkLookup(ctx, c.nodeIDs)
			assertErrClass(t, c.errors, err)
			if ok := assert.Equal(t, len(ns), len(c.responses)); !ok {
				t.FailNow()
			}
			for i, n := range c.responses {
				assert.True(t, proto.Equal(ns[i].Node, n.Node))
				if n == emptyNode {
					assert.Nil(t, ns[i].Id)
					continue
				}
				assert.True(t, bytes.Compare(ns[i].Id.Bytes(), n.Id.Bytes()) == 0)
			}
		})
	}
}

func newServer(ctx context.Context, redisAddr string) (*grpc.Server, *Server, error) {
	ca, err := provider.NewTestCA(ctx)
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

	pb.RegisterOverlayServer(grpcServer, s)

	return grpcServer, s, nil
}

func newTestServer(ctx context.Context) (*grpc.Server, *mockOverlayServer, error) {
	ca, err := provider.NewTestCA(ctx)
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

	pb.RegisterOverlayServer(grpcServer, mo)

	return grpcServer, mo, nil

}

type mockOverlayServer struct {
	lookupCalled           int
	bulkLookupCalled       int
	FindStorageNodesCalled int
}

func (o *mockOverlayServer) Lookup(ctx context.Context, req *pb.LookupRequest) (*pb.LookupResponse, error) {
	o.lookupCalled++
	return &pb.LookupResponse{}, nil
}

func (o *mockOverlayServer) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (*pb.FindStorageNodesResponse, error) {
	o.FindStorageNodesCalled++
	return &pb.FindStorageNodesResponse{}, nil
}

func (o *mockOverlayServer) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (*pb.LookupResponses, error) {
	o.bulkLookupCalled++
	return &pb.LookupResponses{}, nil
}
