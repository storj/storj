// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/identity"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
)

func TestNewKademlia(t *testing.T) {
	rootdir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	cases := []struct {
		id          storj.NodeID
		bn          []pb.Node
		addr        string
		expectedErr error
	}{
		{
			id: func() storj.NodeID {
				id, err := testidentity.NewTestIdentity()
				assert.NoError(t, err)
				return id.ID
			}(),
			bn:   []pb.Node{{Id: teststorj.NodeIDFromString("foo")}},
			addr: "127.0.0.1:8080",
		},
		{
			id: func() storj.NodeID {
				id, err := testidentity.NewTestIdentity()
				assert.NoError(t, err)
				return id.ID
			}(),
			bn:   []pb.Node{{Id: teststorj.NodeIDFromString("foo")}},
			addr: "127.0.0.1:8080",
		},
	}

	for i, v := range cases {
		dir := filepath.Join(rootdir, strconv.Itoa(i))

		ca, err := testidentity.NewTestCA(context.Background())
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		kad, err := NewKademlia(v.id, pb.NodeType_STORAGE, v.bn, v.addr, nil, identity, dir, defaultAlpha)
		assert.NoError(t, err)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, kad.bootstrapNodes, v.bn)
		assert.NotNil(t, kad.nodeClient)
		assert.NotNil(t, kad.routingTable)
		assert.NoError(t, kad.Disconnect())
	}

}

func TestPeerDiscovery(t *testing.T) {
	dir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	// make new identity
	bootServer, mockBootServer, bootID, bootAddress := startTestNodeServer()
	defer bootServer.Stop()
	testServer, _, testID, testAddress := startTestNodeServer()
	defer testServer.Stop()
	targetServer, _, targetID, targetAddress := startTestNodeServer()
	defer targetServer.Stop()

	bootstrapNodes := []pb.Node{{Id: bootID.ID, Address: &pb.NodeAddress{Address: bootAddress}}}
	metadata := &pb.NodeMetadata{
		Email:  "foo@bar.com",
		Wallet: "FarmerWallet",
	}
	k, err := NewKademlia(testID.ID, pb.NodeType_STORAGE, bootstrapNodes, testAddress, metadata, testID, dir, defaultAlpha)
	assert.NoError(t, err)
	rt, err := k.GetRoutingTable(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, rt.Local().Metadata.Email, "foo@bar.com")
	assert.Equal(t, rt.Local().Metadata.Wallet, "FarmerWallet")

	defer func() {
		assert.NoError(t, k.Disconnect())
	}()

	cases := []struct {
		target      storj.NodeID
		opts        discoveryOptions
		expected    *pb.Node
		expectedErr error
	}{
		{target: func() storj.NodeID {
			// this is what the bootstrap node returns
			mockBootServer.returnValue = []*pb.Node{{Id: targetID.ID, Address: &pb.NodeAddress{Address: targetAddress}}}
			return targetID.ID
		}(),
			opts:        discoveryOptions{concurrency: 3, bootstrap: true, retries: 1},
			expected:    &pb.Node{},
			expectedErr: nil,
		},
		{target: bootID.ID,
			opts:        discoveryOptions{concurrency: 3, bootstrap: true, retries: 1},
			expected:    nil,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		err := k.lookup(context.Background(), v.target, v.opts)
		assert.Equal(t, v.expectedErr, err)
	}
}

func TestBootstrap(t *testing.T) {
	bn, s, clean := testNode(t, []pb.Node{})
	defer clean()
	defer s.Stop()

	n1, s1, clean1 := testNode(t, []pb.Node{bn.routingTable.self})
	defer clean1()
	defer s1.Stop()

	err := n1.Bootstrap(context.Background())
	assert.NoError(t, err)

	n2, s2, clean2 := testNode(t, []pb.Node{bn.routingTable.self})
	defer clean2()
	defer s2.Stop()

	err = n2.Bootstrap(context.Background())
	assert.NoError(t, err)

	nodeIDs, err := n2.routingTable.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Len(t, nodeIDs, 3)
}

func testNode(t *testing.T, bn []pb.Node) (*Kademlia, *grpc.Server, func()) {
	// new address
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	// new config
	// new identity
	fid, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)
	// new kademlia
	dir, cleanup := mktempdir(t, "kademlia")

	k, err := NewKademlia(fid.ID, pb.NodeType_STORAGE, bn, lis.Addr().String(), nil, fid, dir, defaultAlpha)
	assert.NoError(t, err)
	s := node.NewServer(k)
	// new ident opts
	identOpt, err := fid.ServerOption()
	assert.NoError(t, err)

	grpcServer := grpc.NewServer(identOpt)

	pb.RegisterNodesServer(grpcServer, s)
	go func() { assert.NoError(t, grpcServer.Serve(lis)) }()

	return k, grpcServer, func() {
		defer cleanup()
		assert.NoError(t, k.Disconnect())
	}

}

func TestGetNodes(t *testing.T) {
	var (
		nodeIDA = teststorj.NodeIDFromString("AAAAA")
		nodeIDB = teststorj.NodeIDFromString("BBBBB")
		nodeIDC = teststorj.NodeIDFromString("CCCCC")
		nodeIDD = teststorj.NodeIDFromString("DDDDD")
	)

	lis, err := net.Listen("tcp", "127.0.0.1:0")

	assert.NoError(t, err)

	srv, _ := newTestServer([]*pb.Node{{Id: teststorj.NodeIDFromString("foo")}})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	// make new identity
	fid, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)
	fid2, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)
	fid.ID = nodeIDA
	fid2.ID = nodeIDB
	// create two new unique identities
	assert.NotEqual(t, fid.ID, fid2.ID)
	dir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	k, err := NewKademlia(fid.ID, pb.NodeType_STORAGE, []pb.Node{{Id: fid2.ID, Address: &pb.NodeAddress{Address: lis.Addr().String()}}}, lis.Addr().String(), nil, fid, dir, defaultAlpha)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, k.Disconnect())
	}()

	// add nodes
	ids := storj.NodeIDList{nodeIDA, nodeIDB, nodeIDC, nodeIDD}
	bw := []int64{1, 2, 3, 4}
	disk := []int64{4, 3, 2, 1}
	nodes := []*pb.Node{}
	for i, v := range ids {
		n := &pb.Node{
			Id: v,
			Restrictions: &pb.NodeRestrictions{
				FreeBandwidth: bw[i],
				FreeDisk:      disk[i],
			},
		}
		nodes = append(nodes, n)
		err = k.routingTable.ConnectionSuccess(n)
		assert.NoError(t, err)
	}

	cases := []struct {
		testID       string
		start        storj.NodeID
		limit        int
		restrictions []pb.Restriction
		expected     []*pb.Node
	}{
		{testID: "one",
			start: nodeIDB,
			limit: 2,
			restrictions: []pb.Restriction{
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_FREE_BANDWIDTH,
					Value:    int64(2),
				},
			},
			expected: nodes[2:],
		},
		{testID: "two",
			start: nodeIDA,
			limit: 3,
			restrictions: []pb.Restriction{
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_FREE_BANDWIDTH,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_LT,
					Operand:  pb.Restriction_FREE_DISK,
					Value:    int64(2),
				},
			},
			expected: nodes[3:],
		},
		{testID: "three",
			start:        nodeIDA,
			limit:        4,
			restrictions: []pb.Restriction{},
			expected:     nodes,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ns, err := k.GetNodes(context.Background(), c.start, c.limit, c.restrictions...)
			assert.NoError(t, err)
			assert.Equal(t, len(c.expected), len(ns))
			for i, n := range ns {
				assert.True(t, bytes.Equal(c.expected[i].Id.Bytes(), n.Id.Bytes()))
			}
		})
	}
}

func TestMeetsRestrictions(t *testing.T) {
	cases := []struct {
		testID string
		r      []pb.Restriction
		n      pb.Node
		expect bool
	}{
		{testID: "pass one",
			r: []pb.Restriction{
				{
					Operator: pb.Restriction_EQ,
					Operand:  pb.Restriction_FREE_BANDWIDTH,
					Value:    int64(1),
				},
			},
			n: pb.Node{
				Restrictions: &pb.NodeRestrictions{
					FreeBandwidth: int64(1),
				},
			},
			expect: true,
		},
		{testID: "pass multiple",
			r: []pb.Restriction{
				{
					Operator: pb.Restriction_LTE,
					Operand:  pb.Restriction_FREE_BANDWIDTH,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_GTE,
					Operand:  pb.Restriction_FREE_DISK,
					Value:    int64(2),
				},
			},
			n: pb.Node{
				Restrictions: &pb.NodeRestrictions{
					FreeBandwidth: int64(1),
					FreeDisk:      int64(3),
				},
			},
			expect: true,
		},
		{testID: "fail one",
			r: []pb.Restriction{
				{
					Operator: pb.Restriction_LT,
					Operand:  pb.Restriction_FREE_BANDWIDTH,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_FREE_DISK,
					Value:    int64(2),
				},
			},
			n: pb.Node{
				Restrictions: &pb.NodeRestrictions{
					FreeBandwidth: int64(2),
					FreeDisk:      int64(3),
				},
			},
			expect: false,
		},
		{testID: "fail multiple",
			r: []pb.Restriction{
				{
					Operator: pb.Restriction_LT,
					Operand:  pb.Restriction_FREE_BANDWIDTH,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_FREE_DISK,
					Value:    int64(2),
				},
			},
			n: pb.Node{
				Restrictions: &pb.NodeRestrictions{
					FreeBandwidth: int64(2),
					FreeDisk:      int64(2),
				},
			},
			expect: false,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			result := meetsRestrictions(c.r, c.n)
			assert.Equal(t, c.expect, result)
		})
	}
}

func mktempdir(t *testing.T, dir string) (string, func()) {
	rootdir, err := ioutil.TempDir("", dir)
	assert.NoError(t, err)
	cleanup := func() {
		assert.NoError(t, os.RemoveAll(rootdir))
	}
	return rootdir, cleanup
}

func startTestNodeServer() (*grpc.Server, *mockNodesServer, *provider.FullIdentity, string) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, nil, ""
	}

	ca, err := testidentity.NewTestCA(context.Background())
	if err != nil {
		return nil, nil, nil, ""
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, nil, nil, ""
	}
	identOpt, err := identity.ServerOption()
	if err != nil {
		return nil, nil, nil, ""
	}
	grpcServer := grpc.NewServer(identOpt)
	mn := &mockNodesServer{queryCalled: 0}

	pb.RegisterNodesServer(grpcServer, mn)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			return
		}
	}()

	return grpcServer, mn, identity, lis.Addr().String()
}

func newTestServer(nn []*pb.Node) (*grpc.Server, *mockNodesServer) {
	ca, err := testidentity.NewTestCA(context.Background())
	if err != nil {
		return nil, nil
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, nil
	}
	identOpt, err := identity.ServerOption()
	if err != nil {
		return nil, nil
	}
	grpcServer := grpc.NewServer(identOpt)
	mn := &mockNodesServer{queryCalled: 0}

	pb.RegisterNodesServer(grpcServer, mn)

	return grpcServer, mn
}

type mockNodesServer struct {
	queryCalled int32
	pingCalled  int32
	returnValue []*pb.Node
}

func (mn *mockNodesServer) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	atomic.AddInt32(&mn.queryCalled, 1)
	return &pb.QueryResponse{Response: mn.returnValue}, nil
}

func (mn *mockNodesServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	atomic.AddInt32(&mn.pingCalled, 1)
	return &pb.PingResponse{}, nil
}
