// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/dht/mocks"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	fooID    = teststorj.NodeIDFromString("foo")
	one001ID = teststorj.NodeIDFromBytes([]byte{1, 0, 0, 1})
	one000ID = teststorj.NodeIDFromBytes([]byte{1, 0, 0, 0})
	zeroID   = teststorj.NodeIDFromBytes([]byte{0, 0, 0, 0})
	node1001 = storj.NewNodeWithID(one001ID, &pb.Node{})
	node1000 = storj.NewNodeWithID(one001ID, &pb.Node{})
	nodeFoo  = storj.NewNodeWithID(one001ID, &pb.Node{})
	nodeZero = storj.NewNodeWithID(zeroID, &pb.Node{})
	aID      = teststorj.NodeIDFromString("a")
	fID      = teststorj.NodeIDFromString("f")
	gID      = teststorj.NodeIDFromString("g")
	hID      = teststorj.NodeIDFromString("h")
	nodeA = storj.NewNodeWithID(aID, &pb.Node{})
	nodeF = storj.NewNodeWithID(fID, &pb.Node{})
	nodeG = storj.NewNodeWithID(gID, &pb.Node{})
	nodeH = storj.NewNodeWithID(hID, &pb.Node{})
)

func TestGetWork(t *testing.T) {
	cases := []struct {
		name     string
		worker   *worker
		expected storj.Node
		ch       chan storj.Node
	}{
		{
			name:     "test valid chore returned",
			worker:   func() *worker {
				w := newWorker(context.Background(), nil, []storj.Node{node1001}, nil, node1001.Id, 5)
				return w
			}(),
			expected: node1001,
			ch:       make(chan storj.Node, 2),
		},
		{
			name: "test no chore left",
			worker: func() *worker {
				w := newWorker(context.Background(), nil, []storj.Node{nodeFoo}, nil, nodeFoo.Id, 5)
				w.maxResponse = 0
				w.pq.Closest()
				assert.Equal(t, w.pq.Len(), 0)
				return w
			}(),
			expected: storj.Node{},
			ch:       make(chan storj.Node, 2),
		},
	}

	for _, v := range cases {
		ctx, cf := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cf()

		v.worker.cancel = cf
		v.worker.getWork(ctx, v.ch)

		if v.expected != (storj.Node{}) {
			actual := <-v.ch
			assert.Equal(t, v.expected, actual)
		} else {
			assert.Len(t, v.ch, 0)
		}
	}
}

func TestWorkCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	worker := newWorker(ctx, nil, []storj.Node{node1001}, nil, node1000.Id, 5)
	// TODO: ensure this also works when running
	cancel()
	worker.work(ctx, make(chan storj.Node))
}

func TestWorkerLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDHT := mock_dht.NewMockDHT(ctrl)
	mockRT := mock_dht.NewMockRoutingTable(ctrl)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	srv, mock := newTestServer(nil)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()
	cases := []struct {
		name     string
		worker   *worker
		work     storj.Node
		expected []storj.Node
	}{
		{
			name: "test valid chore returned",
			worker: func() *worker {
				ca, err := provider.NewTestCA(context.Background())
				assert.NoError(t, err)

				identity, err := ca.NewIdentity()
				assert.NoError(t, err)

				n := storj.NewNodeWithID(fooID, &pb.Node{
					Address: &pb.NodeAddress{Address: "127.0.0.1:0"},
				})
				nc, err := node.NewNodeClient(identity, n, mockDHT)
				assert.NoError(t, err)

				mock.returnValue = []storj.Node{nodeFoo}
				return newWorker(context.Background(), nil, []storj.Node{nodeFoo}, nc, fooID, 5)
			}(),
			work:     storj.NewNodeWithID(fooID, &pb.Node{
				Address: &pb.NodeAddress{Address: lis.Addr().String()},
			}),
			expected: []storj.Node{nodeFoo},
		},
	}

	for _, v := range cases {
		mockDHT.EXPECT().GetRoutingTable(gomock.Any()).Return(mockRT, nil)
		mockRT.EXPECT().ConnectionSuccess(gomock.Any()).Return(nil)
		actual := v.worker.lookup(context.Background(), v.work)
		assert.Equal(t, v.expected, actual)
		assert.Equal(t, int32(1), mock.queryCalled)
	}
}

func TestUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDHT := mock_dht.NewMockDHT(ctrl)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	srv, _ := newTestServer(nil)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	cases := []struct {
		name                string
		worker              *worker
		input               []storj.Node
		expectedQueueLength int
		expected            []storj.Node
		expectedErr         error
	}{
		{
			name: "test nil nodes",
			worker: func() *worker {
				ca, err := provider.NewTestCA(context.Background())
				assert.NoError(t, err)
				identity, err := ca.NewIdentity()
				assert.NoError(t, err)
				nc, err := node.NewNodeClient(identity, storj.NewNodeWithID(fooID, &pb.Node{
					Address: &pb.NodeAddress{Address: ":7070"},
				}), mockDHT)
				assert.NoError(t, err)
				return newWorker(context.Background(), nil, []storj.Node{nodeZero}, nc, fooID, 2)
			}(),
			expectedQueueLength: 1,
			input:               nil,
			expectedErr:         WorkerError.New("nodes must not be empty"),
			expected:            []storj.Node{nodeZero},
		},
		{
			name: "test combined less than k",
			worker: func() *worker {
				ca, err := provider.NewTestCA(context.Background())
				assert.NoError(t, err)
				identity, err := ca.NewIdentity()
				assert.NoError(t, err)
				nc, err := node.NewNodeClient(identity, storj.NewNodeWithID(aID, &pb.Node{
					Address: &pb.NodeAddress{Address: ":7070"},
				}), mockDHT)
				assert.NoError(t, err)
				return newWorker(context.Background(), nil, []storj.Node{nodeH}, nc, aID, 2)
			}(),
			expectedQueueLength: 2,
			expected:            []storj.Node{nodeG, nodeF},
			input:               []storj.Node{nodeF, nodeG},
			expectedErr:         nil,
		},
	}

	for _, v := range cases {
		v.worker.update(v.input)
		assert.Equal(t, v.expectedQueueLength, v.worker.pq.Len())
		i := 0
		for v.worker.pq.Len() > 0 {
			node, _ := v.worker.pq.Closest()
			assert.Equal(t, v.expected[i], node)
			i++
		}
	}
}

func newTestServer(nn []storj.Node) (*grpc.Server, *mockNodeServer) {
	ca, err := provider.NewTestCA(context.Background())
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
	mn := &mockNodeServer{queryCalled: 0}

	pb.RegisterNodesServer(grpcServer, mn)

	return grpcServer, mn
}

type mockNodeServer struct {
	queryCalled int32
	pingCalled  int32
	returnValue []storj.Node
}

func (mn *mockNodeServer) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	atomic.AddInt32(&mn.queryCalled, 1)
	return &pb.QueryResponse{Response: storj.ProtoNodes(mn.returnValue)}, nil
}

func (mn *mockNodeServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	atomic.AddInt32(&mn.pingCalled, 1)
	return &pb.PingResponse{}, nil
}
