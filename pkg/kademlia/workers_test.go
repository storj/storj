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
	"storj.io/storj/pkg/dht/mocks"
	"storj.io/storj/pkg/node"
	mock "storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

func TestGetWork(t *testing.T) {
	cases := []struct {
		name     string
		worker   *worker
		expected *pb.Node
		ch       chan *pb.Node
	}{
		{
			name:     "test valid chore returned",
			worker:   newWorker(context.Background(), nil, []*pb.Node{&pb.Node{Id: "1001"}}, nil, node.IDFromString("1000"), 5),
			expected: &pb.Node{Id: "1001"},
			ch:       make(chan *pb.Node, 2),
		},
		{
			name: "test no chore left",
			worker: func() *worker {
				w := newWorker(context.Background(), nil, []*pb.Node{&pb.Node{Id: "foo"}}, nil, node.IDFromString("foo"), 5)
				w.maxResponse = 0
				w.pq.Pop()
				assert.Len(t, w.pq, 0)
				return w
			}(),
			expected: nil,
			ch:       make(chan *pb.Node, 2),
		},
	}

	for _, v := range cases {
		ctx, cf := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cf()

		v.worker.cancel = cf
		v.worker.getWork(ctx, v.ch)

		if v.expected != nil {
			actual := <-v.ch
			assert.Equal(t, v.expected, actual)
		} else {
			assert.Len(t, v.ch, 0)
		}
	}
}

func TestWorkCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	worker := newWorker(ctx, nil, []*pb.Node{&pb.Node{Id: "1001"}}, nil, node.IDFromString("1000"), 5)
	// TODO: ensure this also works when running
	cancel()
	worker.work(ctx, make(chan *pb.Node))
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
		work     *pb.Node
		expected []*pb.Node
	}{
		{
			name: "test valid chore returned",
			worker: func() *worker {
				ca, err := provider.NewCA(context.Background(), 12, 4)
				assert.NoError(t, err)
				identity, err := ca.NewIdentity()
				assert.NoError(t, err)
				nc, err := node.NewNodeClient(identity, pb.Node{Id: "foo", Address: &pb.NodeAddress{Address: "127.0.0.1:0"}}, mockDHT)
				assert.NoError(t, err)
				mock.returnValue = []*pb.Node{&pb.Node{Id: "foo"}}
				return newWorker(context.Background(), nil, []*pb.Node{&pb.Node{Id: "foo"}}, nc, node.IDFromString("foo"), 5)
			}(),
			work:     &pb.Node{Id: "foo", Address: &pb.NodeAddress{Address: lis.Addr().String()}},
			expected: []*pb.Node{&pb.Node{Id: "foo"}},
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
		input               []*pb.Node
		expectedQueueLength int
		expected            []*pb.Node
		expectedErr         error
	}{
		{
			name: "test nil nodes",
			worker: func() *worker {
				ca, err := provider.NewCA(context.Background(), 12, 4)
				assert.NoError(t, err)
				identity, err := ca.NewIdentity()
				assert.NoError(t, err)
				nc, err := node.NewNodeClient(identity, pb.Node{Id: "foo", Address: &pb.NodeAddress{Address: ":7070"}}, mockDHT)
				assert.NoError(t, err)
				return newWorker(context.Background(), nil, []*pb.Node{&pb.Node{Id: "0000"}}, nc, node.IDFromString("foo"), 2)
			}(),
			expectedQueueLength: 1,
			input:               nil,
			expectedErr:         WorkerError.New("nodes must not be empty"),
			expected:            []*pb.Node{&pb.Node{Id: "0000"}},
		},
		{
			name: "test combined less than k",
			worker: func() *worker {
				ca, err := provider.NewCA(context.Background(), 12, 4)
				assert.NoError(t, err)
				identity, err := ca.NewIdentity()
				assert.NoError(t, err)
				nc, err := node.NewNodeClient(identity, pb.Node{Id: "foo", Address: &pb.NodeAddress{Address: ":7070"}}, mockDHT)
				assert.NoError(t, err)
				return newWorker(context.Background(), nil, []*pb.Node{&pb.Node{Id: "0001"}}, nc, node.IDFromString("1100"), 2)
			}(),
			expectedQueueLength: 2,
			expected:            []*pb.Node{&pb.Node{Id: "0100"}, &pb.Node{Id: "1001"}},
			input:               []*pb.Node{&pb.Node{Id: "1001"}, &pb.Node{Id: "0100"}},
			expectedErr:         nil,
		},
	}

	for _, v := range cases {
		v.worker.update(v.input)

		assert.Len(t, v.worker.pq, v.expectedQueueLength)

		i := 0
		for v.worker.pq.Len() > 0 {
			assert.Equal(t, v.expected[i], v.worker.pq.Pop().(*Item).value)
			i++
		}
	}
}

func newTestServer(nn []*pb.Node) (*grpc.Server, *mockNodeServer) {
	ca, err := provider.NewCA(context.Background(), 12, 4)
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
	pb.RegisterOverlayServer(grpcServer, mock.NewOverlay(nn))

	return grpcServer, mn
}

type mockNodeServer struct {
	queryCalled int32
	returnValue []*pb.Node
}

func (mn *mockNodeServer) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	atomic.AddInt32(&mn.queryCalled, 1)
	return &pb.QueryResponse{Response: mn.returnValue}, nil

}
