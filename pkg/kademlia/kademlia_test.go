// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	// testIDA   = teststorj.NodeIDFromString("AAAAA")
	// testIDB   = teststorj.NodeIDFromString("BBBBB")
	// testIDC   = teststorj.NodeIDFromString("CCCCC")
	testIDD   = teststorj.NodeIDFromString("DDDDD")
)

// helper function to generate new node identities with
// correct difficulty and concurrency
func newTestIdentity() (*provider.FullIdentity, error) {
	fid, err := node.NewFullIdentity(context.Background(), 12, 4)
	return fid, err
}

func TestNewKademlia(t *testing.T) {
	rootdir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	cases := []struct {
		id          storj.NodeID
		bn          []storj.Node
		addr        string
		expectedErr error
	}{
		{
			id: func() storj.NodeID {
				id, err := newTestIdentity()
				assert.NoError(t, err)
				return id.ID
			}(),
			bn:   []storj.Node{nodeFoo},
			addr: "127.0.0.1:8080",
		},
		{
			id: func() storj.NodeID {
				id, err := newTestIdentity()
				assert.NoError(t, err)
				return id.ID
			}(),
			bn:   []storj.Node{nodeFoo},
			addr: "127.0.0.1:8080",
		},
	}

	for i, v := range cases {
		dir := filepath.Join(rootdir, strconv.Itoa(i))

		ca, err := provider.NewTestCA(context.Background())
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		kad, err := NewKademlia(v.id, v.bn, v.addr, identity, dir, defaultAlpha)
		assert.NoError(t, err)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, kad.bootstrapNodes, v.bn)
		assert.NotNil(t, kad.nodeClient)
		assert.NotNil(t, kad.routingTable)
		assert.NoError(t, kad.Disconnect())
	}

}

func TestPeerDiscovery(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	addr := lis.Addr().String()

	assert.NoError(t, err)

	srv, mns := newTestServer([]storj.Node{nodeFoo})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	dir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	k := func() *Kademlia {
		// create two new unique identities
		fid, err := newTestIdentity()
		assert.NoError(t, err)
		fid2, err := newTestIdentity()
		assert.NoError(t, err)
		assert.NotEqual(t, fid.ID, fid2.ID)

		k, err := NewKademlia(fid.ID, []storj.Node{
			storj.NewNodeWithID(
				fid2.ID,
				&pb.Node{
					Address: &pb.NodeAddress{Address: lis.Addr().String()},
				},
			),
		}, lis.Addr().String(), fid, dir, defaultAlpha)
		assert.NoError(t, err)
		return k
	}()

	defer func() {
		assert.NoError(t, k.Disconnect())
	}()

	cases := []struct {
		target      storj.NodeID
		opts        discoveryOptions
		expected    storj.Node
		expectedErr error
	}{
		{target: func() storj.NodeID {
			fid, err := newTestIdentity()
			assert.NoError(t, err)
			mns.returnValue = []storj.Node{
				storj.NewNodeWithID(fid.ID, &pb.Node{Address: &pb.NodeAddress{Address: addr}}),
			}
			return fid.ID
		}(),
			opts:        discoveryOptions{concurrency: 3, bootstrap: true, retries: 1},
			expected:    storj.Node{},
			expectedErr: nil,
		},
		{target: func() storj.NodeID {
			id, err := newTestIdentity()
			assert.NoError(t, err)
			return id.ID
		}(),
			opts:        discoveryOptions{concurrency: 3, bootstrap: true, retries: 1},
			expected:    storj.Node{},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		err := k.lookup(context.Background(), v.target, v.opts)
		assert.Equal(t, v.expectedErr, err)
	}
}

func TestBootstrap(t *testing.T) {
	bn, s, clean := testNode(t, []storj.Node{})
	defer clean()
	defer s.Stop()

	n1, s1, clean1 := testNode(t, []storj.Node{bn.routingTable.self})
	defer clean1()
	defer s1.Stop()

	err := n1.Bootstrap(context.Background())
	assert.NoError(t, err)

	n2, s2, clean2 := testNode(t, []storj.Node{bn.routingTable.self})
	defer clean2()
	defer s2.Stop()

	err = n2.Bootstrap(context.Background())
	assert.NoError(t, err)

	nodeIDs, err := n2.routingTable.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Len(t, nodeIDs, 3)
}

func testNode(t *testing.T, bn []storj.Node) (*Kademlia, *grpc.Server, func()) {
	// new address
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	// new config
	// new identity
	fid, err := newTestIdentity()
	assert.NoError(t, err)
	// new kademlia
	dir, cleanup := mktempdir(t, "kademlia")

	k, err := NewKademlia(fid.ID, bn, lis.Addr().String(), fid, dir, defaultAlpha)
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
	lis, err := net.Listen("tcp", "127.0.0.1:0")

	assert.NoError(t, err)

	srv, _ := newTestServer([]storj.Node{nodeFoo})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	// make new identity
	fid, err := newTestIdentity()
	assert.NoError(t, err)
	fid2, err := newTestIdentity()
	assert.NoError(t, err)
	fid.ID = testIDA
	fid2.ID = testIDB
	// create two new unique identities
	assert.NotEqual(t, fid.ID, fid2.ID)

	dir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	k, err := NewKademlia(fid.ID, []storj.Node{
		storj.NewNodeWithID(
			fid2.ID,
			&pb.Node{Address: &pb.NodeAddress{Address: lis.Addr().String()}},
	)}, lis.Addr().String(), fid, dir, defaultAlpha)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, k.Disconnect())
	}()

	// add nodes
	ids := storj.NodeIDList{
		testIDA,
		testIDB,
		testIDC,
		testIDD,
	}
	bw := []int64{1, 2, 3, 4}
	disk := []int64{4, 3, 2, 1}
	var nodes []storj.Node
	for i, v := range ids {
		n := storj.NewNodeWithID(
			v,
			&pb.Node{
				Restrictions: &pb.NodeRestrictions{
					FreeBandwidth: bw[i],
					FreeDisk:      disk[i],
				},
			},
		)
		nodes = append(nodes, n)
		err = k.routingTable.ConnectionSuccess(n)
		assert.NoError(t, err)
	}

	cases := []struct {
		testID       string
		start        storj.NodeID
		limit        int
		restrictions []pb.Restriction
		expected     []storj.Node
	}{
		{testID: "one",
			start: testIDB,
			limit: 2,
			restrictions: []pb.Restriction{
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
			},
			expected: nodes[2:],
		},
		{testID: "two",
			start: testIDA,
			limit: 3,
			restrictions: []pb.Restriction{
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_LT,
					Operand:  pb.Restriction_freeDisk,
					Value:    int64(2),
				},
			},
			expected: nodes[3:],
		},
		{testID: "three",
			start:        testIDA,
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
				assert.True(t, proto.Equal(c.expected[i].Node, n.Node))
			}
		})
	}
}

func TestMeetsRestrictions(t *testing.T) {
	cases := []struct {
		testID string
		r      []pb.Restriction
		n      *pb.Node
		expect bool
	}{
		{testID: "pass one",
			r: []pb.Restriction{
				{
					Operator: pb.Restriction_EQ,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(1),
				},
			},
			n: &pb.Node{
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
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_GTE,
					Operand:  pb.Restriction_freeDisk,
					Value:    int64(2),
				},
			},
			n: &pb.Node{
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
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeDisk,
					Value:    int64(2),
				},
			},
			n: &pb.Node{
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
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeDisk,
					Value:    int64(2),
				},
			},
			n: &pb.Node{
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
			result := meetsRestrictions(c.r, storj.Node{Node: c.n})
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
