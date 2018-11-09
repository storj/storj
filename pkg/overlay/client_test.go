// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb/sdbclient"
	"storj.io/storj/storage"
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

func TestChoose(t *testing.T) {
	defaultRep := &pb.NodeRep{
		UptimeRatio:       1,
		AuditSuccessRatio: 1,
		AuditCount:        20,
	}
	cases := []struct {
		limit        int
		space        int64
		uptime       float64
		auditSuccess float64
		auditCount   int64
		allNodes     []*pb.Node
		excluded     []dht.NodeID
	}{
		{
			limit:        4,
			space:        0,
			uptime:       1,
			auditSuccess: 1,
			auditCount:   10,
			allNodes: func() []*pb.Node {
				n1 := &pb.Node{Id: "n1", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.1"}}
				n2 := &pb.Node{Id: "n2", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.2"}}
				n3 := &pb.Node{Id: "n3", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.3"}}
				n4 := &pb.Node{Id: "n4", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.4"}}
				n5 := &pb.Node{Id: "n5", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.5"}}
				n6 := &pb.Node{Id: "n6", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.6"}}
				n7 := &pb.Node{Id: "n7", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.7"}}
				n8 := &pb.Node{Id: "n8", Reputation: defaultRep,
					Address: &pb.NodeAddress{Address: "127.0.0.8"}}
				return []*pb.Node{n1, n2, n3, n4, n5, n6, n7, n8}
			}(),
			excluded: func() []dht.NodeID {
				id1 := node.IDFromString("n1")
				id2 := node.IDFromString("n2")
				id3 := node.IDFromString("n3")
				id4 := node.IDFromString("n4")
				return []dht.NodeID{id1, id2, id3, id4}
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
				Key:   storage.Key(n.Id),
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

		// TODO(moby) update to include reputation values
		newNodes, err := oc.Choose(ctx, Options{
			Amount:       v.limit,
			Space:        v.space,
			Uptime:       v.uptime,
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
	n1 := &pb.Node{Id: "n1"}
	n2 := &pb.Node{Id: "n2"}
	n3 := &pb.Node{Id: "n3"}
	nodes := []*pb.Node{n1, n2, n3}
	for _, n := range nodes {
		assert.NoError(t, s.cache.Put(ctx, n.Id, *n))
	}

	cases := []struct {
		testID    string
		nodeIDs   []dht.NodeID
		responses []*pb.Node
		errors    *errs.Class
	}{
		{testID: "empty id",
			nodeIDs:   []dht.NodeID{},
			responses: nil,
			errors:    &ClientError,
		},
		{testID: "valid ids",
			nodeIDs: func() []dht.NodeID {
				id1 := node.IDFromString("n1")
				id2 := node.IDFromString("n2")
				id3 := node.IDFromString("n3")
				return []dht.NodeID{id1, id2, id3}
			}(),
			responses: nodes,
			errors:    nil,
		},
		{testID: "missing ids",
			nodeIDs: func() []dht.NodeID {
				id1 := node.IDFromString("n4")
				id2 := node.IDFromString("n5")
				return []dht.NodeID{id1, id2}
			}(),
			responses: []*pb.Node{nil, nil},
			errors:    nil,
		},
		{testID: "random order and nil",
			nodeIDs: func() []dht.NodeID {
				id1 := node.IDFromString("n1")
				id2 := node.IDFromString("n2")
				id3 := node.IDFromString("n3")
				id4 := node.IDFromString("n4")
				return []dht.NodeID{id2, id1, id3, id4}
			}(),
			responses: func() []*pb.Node {
				return []*pb.Node{nodes[1], nodes[0], nodes[2], nil}
			}(),
			errors: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ns, err := oc.BulkLookup(ctx, c.nodeIDs)
			assertErrClass(t, c.errors, err)
			assert.EqualValues(t, len(c.responses), len(ns))
			for i, n := range ns {
				if c.responses[i] == nil {
					assert.Equal(t, n, c.responses[i])
				} else {
					assert.Equal(t, c.responses[i].Id, n.Id)
				}
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

	statdb := sdbclient.NewMockClient()
	grpcServer := grpc.NewServer(identOpt)
	cache, err := NewRedisOverlayCache(redisAddr, "", 1, nil, statdb)
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
