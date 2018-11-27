// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

var fooID = teststorj.NodeIDFromString("foo")

func TestNewOverlayClient(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cases := []struct {
		address string
	}{
		{
			address: "127.0.0.1:8080",
		},
	}

	for _, v := range cases {
		ca, err := testidentity.NewTestCA(ctx)
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

func TestChoose(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cases := []struct {
		limit        int
		space        int64
		uptime       float64
		uptimeCount int64
		auditSuccess float64
		auditCount   int64
		allNodes     []*pb.Node
		excluded     storj.NodeIDList
	}{
		{
			limit:        4,
			space:        0,
			uptime:       1,
			uptimeCount: 10,
			auditSuccess: 1,
			auditCount:   10,
			allNodes: func() []*pb.Node {
				n1 := teststorj.MockNode("n1")
				n2 := teststorj.MockNode("n2")
				n3 := teststorj.MockNode("n3")
				n4 := teststorj.MockNode("n4")
				n5 := teststorj.MockNode("n5")
				n6 := teststorj.MockNode("n6")
				n7 := teststorj.MockNode("n7")
				n8 := teststorj.MockNode("n8")
				nodes := []*pb.Node{n1, n2, n3, n4, n5, n6, n7, n8}
				for _, n := range nodes {
					n.Type = pb.NodeType_STORAGE
				}
				return nodes
			}(),
			excluded: func() storj.NodeIDList {
				id1 := teststorj.NodeIDFromString("n1")
				id2 := teststorj.NodeIDFromString("n2")
				id3 := teststorj.NodeIDFromString("n3")
				id4 := teststorj.NodeIDFromString("n4")
				return storj.NodeIDList{id1, id2, id3, id4}
			}(),
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
				Key:   n.Id.Bytes(),
				Value: data,
			})
		}

		ca, err := testidentity.NewTestCA(ctx)
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

		newNodes, err := oc.Choose(ctx, Options{
			Amount:       v.limit,
			Space:        v.space,
			Uptime:       v.uptime,
			UptimeCount: v.uptimeCount,
			AuditSuccess: v.auditSuccess,
			AuditCount:   v.auditCount,
			Excluded:     v.excluded,
		})
		assert.NoError(t, err)
		for _, new := range newNodes {
			for _, ex := range v.excluded {
				assert.NotEqual(t, ex.String(), new.Id)
			}
		}
	}
}

func TestLookup(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cases := []struct {
		nodeID        storj.NodeID
		expectedCalls int
	}{
		{
			nodeID:        fooID,
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

		ca, err := testidentity.NewTestCA(ctx)
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cases := []struct {
		nodeIDs       storj.NodeIDList
		expectedCalls int
	}{
		{
			nodeIDs:       storj.NodeIDList{fooID, fooID, fooID},
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

		ca, err := testidentity.NewTestCA(ctx)
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	srv, s, err := newServer(ctx)

	assert.NoError(t, err)
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	ca, err := testidentity.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	oc, err := NewOverlayClient(identity, lis.Addr().String())
	assert.NoError(t, err)

	assert.NotNil(t, oc)
	overlay, ok := oc.(*Overlay)
	assert.True(t, ok)
	assert.NotEmpty(t, overlay.client)
	n1 := teststorj.MockNode("n1")
	n2 := teststorj.MockNode("n2")
	n3 := teststorj.MockNode("n3")
	nodes := []*pb.Node{n1, n2, n3}
	for _, n := range nodes {
		assert.NoError(t, s.cache.Put(n.Id, *n))
	}

	nid1 := teststorj.NodeIDFromString("n1")
	nid2 := teststorj.NodeIDFromString("n2")
	nid3 := teststorj.NodeIDFromString("n3")
	nid4 := teststorj.NodeIDFromString("n4")
	nid5 := teststorj.NodeIDFromString("n5")

	{ // empty id
		_, err := oc.BulkLookup(ctx, storj.NodeIDList{})
		assert.Error(t, err)
	}

	{ // valid ids
		ns, err := oc.BulkLookup(ctx, storj.NodeIDList{nid1, nid2, nid3})
		assert.NoError(t, err)
		assert.Equal(t, nodes, ns)
	}

	{ // missing ids
		ns, err := oc.BulkLookup(ctx, storj.NodeIDList{nid4, nid5})
		assert.NoError(t, err)
		assert.Equal(t, []*pb.Node{nil, nil}, ns)
	}

	{ // different order and missing
		ns, err := oc.BulkLookup(ctx, storj.NodeIDList{nid3, nid4, nid1, nid2, nid5})
		assert.NoError(t, err)
		assert.Equal(t, []*pb.Node{n3, nil, n1, n2, nil}, ns)
	}
}

func newServer(ctx context.Context) (*grpc.Server, *Server, error) {
	ca, err := testidentity.NewTestCA(ctx)
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
	s := &Server{cache: NewOverlayCache(teststore.New(), nil, nil)}

	pb.RegisterOverlayServer(grpcServer, s)

	return grpcServer, s, nil
}

func newTestServer(ctx context.Context) (*grpc.Server, *mockOverlayServer, error) {
	ca, err := testidentity.NewTestCA(ctx)
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
