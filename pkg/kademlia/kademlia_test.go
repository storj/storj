// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage/teststore"
)

const (
	defaultAlpha = 5
)

func TestNewKademlia(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	rootdir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	cases := []struct {
		id          *identity.FullIdentity
		bn          []pb.Node
		addr        string
		expectedErr error
	}{
		{
			id: func() *identity.FullIdentity {
				id, err := testidentity.NewTestIdentity(ctx)
				require.NoError(t, err)
				return id
			}(),
			bn:   []pb.Node{{Id: teststorj.NodeIDFromString("foo")}},
			addr: "127.0.0.1:8080",
		},
		{
			id: func() *identity.FullIdentity {
				id, err := testidentity.NewTestIdentity(ctx)
				require.NoError(t, err)
				return id
			}(),
			bn:   []pb.Node{{Id: teststorj.NodeIDFromString("foo")}},
			addr: "127.0.0.1:8080",
		},
	}

	for i, v := range cases {
		dir := filepath.Join(rootdir, strconv.Itoa(i))

		kad, err := newKademlia(zaptest.NewLogger(t), pb.NodeType_STORAGE, v.bn, v.addr, nil, v.id, dir, defaultAlpha)
		require.NoError(t, err)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, kad.bootstrapNodes, v.bn)
		assert.NotNil(t, kad.dialer)
		assert.NotNil(t, kad.routingTable)
		assert.NoError(t, kad.Close())
	}

}

func TestPeerDiscovery(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	// make new identity
	bootServer, mockBootServer, bootID, bootAddress := startTestNodeServer(ctx)
	defer bootServer.Stop()
	testServer, _, testID, testAddress := startTestNodeServer(ctx)
	defer testServer.Stop()
	targetServer, _, targetID, targetAddress := startTestNodeServer(ctx)
	defer targetServer.Stop()

	bootstrapNodes := []pb.Node{{Id: bootID.ID, Address: &pb.NodeAddress{Address: bootAddress}, Type: pb.NodeType_STORAGE}}
	metadata := &pb.NodeMetadata{
		Email:  "foo@bar.com",
		Wallet: "OperatorWallet",
	}
	k, err := newKademlia(zaptest.NewLogger(t), pb.NodeType_STORAGE, bootstrapNodes, testAddress, metadata, testID, dir, defaultAlpha)
	assert.NoError(t, err)
	rt := k.GetRoutingTable()
	assert.Equal(t, rt.Local().Metadata.Email, "foo@bar.com")
	assert.Equal(t, rt.Local().Metadata.Wallet, "OperatorWallet")

	defer ctx.Check(k.Close)

	cases := []struct {
		target      storj.NodeID
		expected    *pb.Node
		expectedErr error
	}{
		{target: func() storj.NodeID {
			mockBootServer.returnValue = []*pb.Node{{Id: targetID.ID, Type: pb.NodeType_STORAGE, Address: &pb.NodeAddress{Address: targetAddress}}}
			return targetID.ID
		}(),
			expected:    &pb.Node{},
			expectedErr: nil,
		},
		{target: bootID.ID,
			expected:    nil,
			expectedErr: nil,
		},
	}
	for _, v := range cases {
		_, err := k.lookup(ctx, v.target, true)
		assert.Equal(t, v.expectedErr, err)
	}
}

func TestBootstrap(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	bn, s, clean := testNode(t, []pb.Node{})
	defer clean()
	defer s.Stop()

	n1, s1, clean1 := testNode(t, []pb.Node{bn.routingTable.self})
	defer clean1()
	defer s1.Stop()

	err := n1.Bootstrap(ctx)
	assert.NoError(t, err)

	n2, s2, clean2 := testNode(t, []pb.Node{bn.routingTable.self})
	defer clean2()
	defer s2.Stop()

	err = n2.Bootstrap(ctx)
	assert.NoError(t, err)

	nodeIDs, err := n2.routingTable.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Len(t, nodeIDs, 3)
}

func testNode(t *testing.T, bn []pb.Node) (*Kademlia, *grpc.Server, func()) {
	ctx := testcontext.New(t)
	// new address
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	// new config
	// new identity
	fid, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	// new kademlia
	dir, cleanup := mktempdir(t, "kademlia")

	logger := zaptest.NewLogger(t)
	k, err := newKademlia(logger, pb.NodeType_STORAGE, bn, lis.Addr().String(), nil, fid, dir, defaultAlpha)
	assert.NoError(t, err)
	s := NewEndpoint(logger, k, k.GetRoutingTable().(*RoutingTable))
	// new ident opts
	identOpt, err := fid.ServerOption()
	assert.NoError(t, err)

	grpcServer := grpc.NewServer(identOpt)

	pb.RegisterNodesServer(grpcServer, s)
	go func() { assert.NoError(t, grpcServer.Serve(lis)) }()

	return k, grpcServer, func() {
		defer cleanup()
		assert.NoError(t, k.Close())
	}
}

func TestRefresh(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	k, s, clean := testNode(t, []pb.Node{})
	defer clean()
	defer s.Stop()
	//turn back time for only bucket
	rt := k.routingTable
	now := time.Now().UTC()
	bID := firstBucketID //always exists
	err := rt.SetBucketTimestamp(bID[:], now.Add(-2*time.Hour))
	assert.NoError(t, err)
	//refresh should  call FindNode, updating the time
	err = k.refresh(ctx, time.Minute)
	assert.NoError(t, err)
	ts1, err := rt.GetBucketTimestamp(bID[:])
	assert.NoError(t, err)
	assert.True(t, now.Add(-5*time.Minute).Before(ts1))
	//refresh should not call FindNode, leaving the previous time
	err = k.refresh(ctx, time.Minute)
	assert.NoError(t, err)
	ts2, err := rt.GetBucketTimestamp(bID[:])
	assert.NoError(t, err)
	assert.True(t, ts1.Equal(ts2))
}

func TestFindNear(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var (
		nodeIDA = teststorj.NodeIDFromString("AAAAA")
		nodeIDB = teststorj.NodeIDFromString("BBBBB")
		nodeIDC = teststorj.NodeIDFromString("CCCCC")
		nodeIDD = teststorj.NodeIDFromString("DDDDD")
	)

	lis, err := net.Listen("tcp", "127.0.0.1:0")

	assert.NoError(t, err)

	srv, _ := newTestServer(ctx, []*pb.Node{{Id: teststorj.NodeIDFromString("foo")}})
	go func() { assert.NoError(t, srv.Serve(lis)) }()
	defer srv.Stop()

	// make new identity
	fid, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	fid2, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	fid.ID = nodeIDA
	fid2.ID = nodeIDB
	// create two new unique identities
	assert.NotEqual(t, fid.ID, fid2.ID)
	dir, cleanup := mktempdir(t, "kademlia")
	defer cleanup()
	k, err := newKademlia(zaptest.NewLogger(t), pb.NodeType_STORAGE, []pb.Node{{Id: fid2.ID, Address: &pb.NodeAddress{Address: lis.Addr().String()}}}, lis.Addr().String(), nil, fid, dir, defaultAlpha)
	assert.NoError(t, err)
	defer ctx.Check(k.Close)

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
			Type: pb.NodeType_STORAGE,
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
			ns, err := k.FindNear(ctx, c.start, c.limit, c.restrictions...)
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

func startTestNodeServer(ctx context.Context) (*grpc.Server, *mockNodesServer, *identity.FullIdentity, string) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, nil, ""
	}

	ca, err := testidentity.NewTestCA(ctx)
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

func newTestServer(ctx context.Context, nn []*pb.Node) (*grpc.Server, *mockNodesServer) {

	ca, err := testidentity.NewTestCA(ctx)
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

// TestRandomIds makes sure finds a random node ID is within a range (start..end]
func TestRandomIds(t *testing.T) {
	for x := 0; x < 1000; x++ {
		var start, end bucketID
		// many valid options
		rand.Read(start[:])
		rand.Read(end[:])
		if bytes.Compare(start[:], end[:]) > 0 {
			start, end = end, start
		}
		id, err := randomIDInRange(start, end)
		assert.NoError(t, err, "Unexpected err in randomIDInRange")
		assert.True(t, bytes.Compare(id[:], start[:]) > 0, "Random id was less than starting id")
		assert.True(t, bytes.Compare(id[:], end[:]) <= 0, "Random id was greater than end id")
		//invalid range
		_, err = randomIDInRange(end, start)
		assert.Error(t, err, "Missing expected err in invalid randomIDInRange")
		//no valid options
		end = start
		_, err = randomIDInRange(start, end)
		assert.Error(t, err, "Missing expected err in empty randomIDInRange")
		// one valid option
		if start[31] == 255 {
			start[31] = 254
		} else {
			end[31] = start[31] + 1
		}
		id, err = randomIDInRange(start, end)
		assert.NoError(t, err, "Unexpected err in randomIDInRange")
		assert.True(t, bytes.Equal(id[:], end[:]), "Not-so-random id was incorrect")
	}
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

// newKademlia returns a newly configured Kademlia instance
func newKademlia(log *zap.Logger, nodeType pb.NodeType, bootstrapNodes []pb.Node, address string, metadata *pb.NodeMetadata, identity *identity.FullIdentity, path string, alpha int) (*Kademlia, error) {
	self := pb.Node{
		Id:       identity.ID,
		Type:     nodeType,
		Address:  &pb.NodeAddress{Address: address},
		Metadata: metadata,
	}

	rt, err := NewRoutingTable(log, self, teststore.New(), teststore.New(), nil)
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}

	return NewService(log, self, bootstrapNodes, transport.NewClient(identity, rt), alpha, rt)
}
