// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage/teststore"
)

const (
	defaultAlpha = 5
)

func TestNewKademlia(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

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

	for _, v := range cases {
		kad, err := newKademlia(zaptest.NewLogger(t), pb.NodeType_STORAGE, v.bn, v.addr, pb.NodeOperator{}, v.id, defaultAlpha)
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

	// make new identity
	mockBootServer, bootID, bootAddress, cancel := startTestNodeServer(t, ctx)
	defer cancel()
	_, testID, testAddress, cancel := startTestNodeServer(t, ctx)
	defer cancel()
	_, targetID, targetAddress, cancel := startTestNodeServer(t, ctx)
	defer cancel()

	bootstrapNodes := []pb.Node{{Id: bootID.ID, Address: &pb.NodeAddress{Address: bootAddress}}}
	operator := pb.NodeOperator{
		Wallet: "OperatorWallet",
	}
	k, err := newKademlia(zaptest.NewLogger(t), pb.NodeType_STORAGE, bootstrapNodes, testAddress, operator, testID, defaultAlpha)
	require.NoError(t, err)
	rt := k.routingTable
	assert.Equal(t, rt.Local().Operator.Wallet, "OperatorWallet")

	defer ctx.Check(k.Close)

	cases := []struct {
		target      storj.NodeID
		expected    *pb.Node
		expectedErr error
	}{
		{target: func() storj.NodeID {
			mockBootServer.returnValue = []*pb.Node{{Id: targetID.ID, Address: &pb.NodeAddress{Address: targetAddress}}}
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
		_, err := k.lookup(ctx, v.target)
		assert.Equal(t, v.expectedErr, err)
	}
}

func TestBootstrap(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	bn, clean := testNode(t, ctx, "1", []pb.Node{})
	defer clean()

	n1, clean := testNode(t, ctx, "2", []pb.Node{bn.routingTable.self.Node})
	defer clean()

	err := n1.Bootstrap(ctx)
	require.NoError(t, err)

	n2, clean := testNode(t, ctx, "3", []pb.Node{bn.routingTable.self.Node})
	defer clean()

	err = n2.Bootstrap(ctx)
	require.NoError(t, err)

	nodeIDs, err := n2.routingTable.nodeBucketDB.List(ctx, nil, 0)
	require.NoError(t, err)
	assert.Len(t, nodeIDs, 3)
}

func TestRefresh(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	k, clean := testNode(t, ctx, "refresh", []pb.Node{})
	defer clean()
	//turn back time for only bucket
	rt := k.routingTable
	now := time.Now().UTC()
	bID := firstBucketID //always exists
	err := rt.SetBucketTimestamp(ctx, bID[:], now.Add(-2*time.Hour))
	require.NoError(t, err)
	//refresh should  call FindNode, updating the time
	err = k.refresh(ctx, time.Minute)
	require.NoError(t, err)
	ts1, err := rt.GetBucketTimestamp(ctx, bID[:])
	require.NoError(t, err)
	assert.True(t, now.Add(-5*time.Minute).Before(ts1))
	//refresh should not call FindNode, leaving the previous time
	err = k.refresh(ctx, time.Minute)
	require.NoError(t, err)
	ts2, err := rt.GetBucketTimestamp(ctx, bID[:])
	require.NoError(t, err)
	assert.True(t, ts1.Equal(ts2))
}

func TestFindNear(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// make new identity
	fid, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	fid2, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, fid.ID, fid2.ID)

	//start kademlia
	lis, lisCancel := newListener(t, ctx, "127.0.0.1:0")
	defer lisCancel()
	_, cancel := newTestServer(t, ctx, lis)
	defer cancel()

	bootstrap := []pb.Node{{Id: fid2.ID, Address: &pb.NodeAddress{Address: lis.Addr().String()}}}
	k, err := newKademlia(zaptest.NewLogger(t), pb.NodeType_STORAGE, bootstrap,
		lis.Addr().String(), pb.NodeOperator{}, fid, defaultAlpha)
	require.NoError(t, err)
	defer ctx.Check(k.Close)

	// add nodes
	var nodes []*pb.Node
	newNode := func(id string, bw, disk int64) pb.Node {
		nodeID := teststorj.NodeIDFromString(id)
		n := &pb.Node{Id: nodeID}
		nodes = append(nodes, n)
		err = k.routingTable.ConnectionSuccess(ctx, n)
		require.NoError(t, err)
		return *n
	}
	nodeIDA := newNode("AAAAA", 1, 4)
	newNode("BBBBB", 2, 3)
	newNode("CCCCC", 3, 2)
	newNode("DDDDD", 4, 1)
	require.Len(t, nodes, 4)

	cases := []struct {
		testID       string
		target       storj.NodeID
		limit        int
		restrictions []pb.Restriction
		expected     []*pb.Node
	}{
		{testID: "three", target: nodeIDA.Id, limit: 4, expected: nodes, restrictions: []pb.Restriction{}},
	}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {

			ns, err := k.FindNear(ctx, testCase.target, testCase.limit)
			require.NoError(t, err)
			assert.Equal(t, len(testCase.expected), len(ns))
			for _, e := range testCase.expected {
				found := false
				for _, n := range ns {
					if e.Id == n.Id {
						found = true
					}
				}
				assert.True(t, found, e.String())
			}
		})
	}
}

// TestRandomIds makes sure finds a random node ID is within a range (start..end]
func TestRandomIds(t *testing.T) {
	for x := 0; x < 1000; x++ {
		var start, end bucketID
		// many valid options
		start = testrand.NodeID()
		end = testrand.NodeID()
		if bytes.Compare(start[:], end[:]) > 0 {
			start, end = end, start
		}
		id, err := randomIDInRange(start, end)
		require.NoError(t, err, "Unexpected err in randomIDInRange")
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
		require.NoError(t, err, "Unexpected err in randomIDInRange")
		assert.True(t, bytes.Equal(id[:], end[:]), "Not-so-random id was incorrect")
	}
}

type mockNodesServer struct {
	queryCalled int32
	pingCalled  int32
	infoCalled  int32
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

func (mn *mockNodesServer) RequestInfo(ctx context.Context, req *pb.InfoRequest) (*pb.InfoResponse, error) {
	atomic.AddInt32(&mn.infoCalled, 1)
	return &pb.InfoResponse{}, nil
}

// newKademlia returns a newly configured Kademlia instance
func newKademlia(log *zap.Logger, nodeType pb.NodeType, bootstrapNodes []pb.Node, address string, operator pb.NodeOperator, identity *identity.FullIdentity, alpha int) (*Kademlia, error) {
	self := &overlay.NodeDossier{
		Node: pb.Node{
			Id:      identity.ID,
			Address: &pb.NodeAddress{Address: address},
		},
		Type:     nodeType,
		Operator: operator,
	}

	rt, err := NewRoutingTable(log, self, teststore.New(), teststore.New(), teststore.New(), nil)
	if err != nil {
		return nil, err
	}

	tlsOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{PeerIDVersions: "*"}, nil)
	if err != nil {
		return nil, err
	}

	kadConfig := Config{
		BootstrapBackoffMax:  10 * time.Second,
		BootstrapBackoffBase: 1 * time.Second,
		Alpha:                alpha,
	}

	kad, err := NewService(log, rpc.NewDefaultDialer(tlsOptions), rt, kadConfig)
	if err != nil {
		return nil, err
	}
	kad.bootstrapNodes = bootstrapNodes

	return kad, nil
}
