// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

// helper function to get kademlia base configs without root Config struct
func kadconfig() KadConfig {
	return KadConfig{
		Alpha:                       5,
		DefaultIDLength:             256,
		DefaultBucketSize:           20,
		DefaultReplacementCacheSize: 5,
	}
}

// helper function to generate new node identities with
// correct difficulty and concurrency
func newTestIdentity() (*provider.FullIdentity, error) {
	fid, err := node.NewFullIdentity(context.Background(), 12, 4)
	return fid, err
}

func TestNewKademlia(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "kad_test")
	assert.NoError(t, err)
	cases := []struct {
		id          dht.NodeID
		bn          []pb.Node
		addr        string
		expectedErr error
		setup       func() error
	}{
		{
			id: func() *node.ID {
				id, err := newTestIdentity()
				assert.NoError(t, err)
				n := node.ID(id.ID)
				return &n
			}(),
			bn:    []pb.Node{pb.Node{Id: "foo"}},
			addr:  "127.0.0.1:8080",
			setup: func() error { return nil },
		},
		{
			id: func() *node.ID {
				id, err := newTestIdentity()
				assert.NoError(t, err)
				n := node.ID(id.ID)
				return &n
			}(),
			bn:    []pb.Node{pb.Node{Id: "foo"}},
			addr:  "127.0.0.1:8080",
			setup: func() error { return os.RemoveAll(dir) },
		},
	}
	var actual *Kademlia
	for _, v := range cases {
		assert.NoError(t, v.setup())
		kc := kadconfig()
		ca, err := provider.NewCA(context.Background(), 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)
		actual, err = NewKademlia(v.id, v.bn, v.addr, identity, dir, kc)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, actual.bootstrapNodes, v.bn)
		assert.NotNil(t, actual.nodeClient)
		assert.NotNil(t, actual.routingTable)
	}
	defer cleanup(ctx, t, actual, dir)
}

func TestLookup(t *testing.T) {
	ctx := context.Background()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	addr := lis.Addr().String()

	assert.NoError(t, err)
	kc := kadconfig()

	srv, mns := newTestServer([]*pb.Node{&pb.Node{Id: "foo"}})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()
	dir, err := ioutil.TempDir("", "kad_test")
	assert.NoError(t, err)
	k := func() *Kademlia {
		// make new identity
		fid, err := newTestIdentity()
		assert.NoError(t, err)
		fid2, err := newTestIdentity()
		assert.NoError(t, err)

		// create two new unique identities
		id := node.ID(fid.ID)
		id2 := node.ID(fid2.ID)
		assert.NotEqual(t, id, id2)

		kid := dht.NodeID(fid.ID)
		k, err := NewKademlia(kid, []pb.Node{pb.Node{Id: id2.String(), Address: &pb.NodeAddress{Address: lis.Addr().String()}}}, lis.Addr().String(), fid, dir, kc)

		assert.NoError(t, err)
		return k
	}()

	cases := []struct {
		k           *Kademlia
		target      dht.NodeID
		opts        lookupOpts
		expected    *pb.Node
		expectedErr error
	}{
		{
			k: k,
			target: func() *node.ID {
				fid, err := newTestIdentity()
				id := dht.NodeID(fid.ID)
				nid := node.ID(fid.ID)
				assert.NoError(t, err)
				mns.returnValue = []*pb.Node{&pb.Node{Id: id.String(), Address: &pb.NodeAddress{Address: addr}}}
				return &nid
			}(),
			opts:        lookupOpts{amount: 5},
			expected:    &pb.Node{},
			expectedErr: nil,
		},
		{
			k: k,
			target: func() *node.ID {
				id, err := newTestIdentity()
				assert.NoError(t, err)
				n := node.ID(id.ID)
				return &n
			}(),
			opts:        lookupOpts{amount: 5},
			expected:    nil,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		err := v.k.lookup(context.Background(), v.target, v.opts)
		assert.Equal(t, v.expectedErr, err)
	}
	defer cleanup(ctx, t, k, dir)
}

func TestBootstrap(t *testing.T) {
	ctx := context.Background()

	bn, s := testNode(t, []pb.Node{})
	defer s.Stop()

	n1, s1 := testNode(t, []pb.Node{*bn.routingTable.self})
	defer s1.Stop()

	err := n1.Bootstrap(context.Background())
	assert.NoError(t, err)

	n2, s2 := testNode(t, []pb.Node{*bn.routingTable.self})
	defer s2.Stop()

	err = n2.Bootstrap(context.Background())
	assert.NoError(t, err)

	nodeIDs, err := n2.routingTable.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Len(t, nodeIDs, 3)

	defer disconnect(ctx, t, bn)
}

func testNode(t *testing.T, bn []pb.Node) (*Kademlia, *grpc.Server) {
	// new address
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	// new config
	kc := kadconfig()
	// new identity
	fid, err := newTestIdentity()
	id := dht.NodeID(fid.ID)
	assert.NoError(t, err)
	// new kademlia

	dir, err := ioutil.TempDir("", "kad_test")
	assert.NoError(t, err)
	k, err := NewKademlia(id, bn, lis.Addr().String(), fid, dir, kc)
	assert.NoError(t, err)
	s := node.NewServer(k)
	// new ident opts
	identOpt, err := fid.ServerOption()
	assert.NoError(t, err)

	grpcServer := grpc.NewServer(identOpt)

	pb.RegisterNodesServer(grpcServer, s)
	go func() { _ = grpcServer.Serve(lis) }()

	return k, grpcServer

}

func TestGetNodes(t *testing.T) {
	ctx := context.Background()

	lis, err := net.Listen("tcp", "127.0.0.1:0")

	assert.NoError(t, err)
	kc := kadconfig()

	srv, _ := newTestServer([]*pb.Node{&pb.Node{Id: "foo"}})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	// make new identity
	fid, err := newTestIdentity()
	assert.NoError(t, err)
	fid2, err := newTestIdentity()
	assert.NoError(t, err)
	fid.ID = "AAAAA"
	fid2.ID = "BBBBB"
	// create two new unique identities
	id := node.ID(fid.ID)
	id2 := node.ID(fid2.ID)
	assert.NotEqual(t, id, id2)
	kid := dht.NodeID(fid.ID)
	dir, err := ioutil.TempDir("", "kad_test")
	assert.NoError(t, err)
	k, err := NewKademlia(kid, []pb.Node{pb.Node{Id: id2.String(), Address: &pb.NodeAddress{Address: lis.Addr().String()}}}, lis.Addr().String(), fid, dir, kc)
	assert.NoError(t, err)
	// add nodes
	ids := []string{"AAAAA", "BBBBB", "CCCCC", "DDDDD"}
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
		start        string
		limit        int
		restrictions []pb.Restriction
		expected     []*pb.Node
	}{
		{testID: "one",
			start: "BBBBB",
			limit: 2,
			restrictions: []pb.Restriction{
				pb.Restriction{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
			},
			expected: nodes[2:],
		},
		{testID: "two",
			start: "AAAAA",
			limit: 3,
			restrictions: []pb.Restriction{
				pb.Restriction{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				pb.Restriction{
					Operator: pb.Restriction_LT,
					Operand:  pb.Restriction_freeDisk,
					Value:    int64(2),
				},
			},
			expected: nodes[3:],
		},
		{testID: "three",
			start:        "AAAAA",
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
				assert.True(t, proto.Equal(c.expected[i], n))
			}
		})
	}
	defer cleanup(ctx, t, k, dir)
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
				pb.Restriction{
					Operator: pb.Restriction_EQ,
					Operand:  pb.Restriction_freeBandwidth,
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
				pb.Restriction{
					Operator: pb.Restriction_LTE,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				pb.Restriction{
					Operator: pb.Restriction_GTE,
					Operand:  pb.Restriction_freeDisk,
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
				pb.Restriction{
					Operator: pb.Restriction_LT,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				pb.Restriction{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeDisk,
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
				pb.Restriction{
					Operator: pb.Restriction_LT,
					Operand:  pb.Restriction_freeBandwidth,
					Value:    int64(2),
				},
				pb.Restriction{
					Operator: pb.Restriction_GT,
					Operand:  pb.Restriction_freeDisk,
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

func cleanup(ctx context.Context, t *testing.T, k *Kademlia, dir string) {
	disconnect(ctx, t, k)
	removeAll(t, dir)
}

func disconnect(ctx context.Context, t *testing.T, k *Kademlia) {
	assert.NoError(t, k.Disconnect(ctx))
}

func removeAll(t *testing.T, dir string) {
	assert.NoError(t, os.RemoveAll(dir))
}
