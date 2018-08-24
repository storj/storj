// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"storj.io/storj/internal/test"
	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

func TestGetWork(t *testing.T) {
	cases := []struct {
		name     string
		worker   *worker
		expected *proto.Node
	}{
		{
			name:     "test valid chore returned",
			worker:   newWorker(context.Background(), nil, []*proto.Node{&proto.Node{Id: "1001"}}, nil, node.StringToID("1000"), 5),
			expected: &proto.Node{Id: "1001"},
		},
		{
			name: "test no chore left",
			worker: func() *worker {
				w := newWorker(context.Background(), nil, []*proto.Node{&proto.Node{Id: "foo"}}, nil, node.StringToID("foo"), 5)
				w.maxResponse = 0
				w.pq.Pop()
				assert.Len(t, w.pq, 0)
				w.cancel = func() {}
				return w
			}(),
			expected: nil,
		},
	}

	for _, v := range cases {
		actual := v.worker.getWork()
		if v.expected != nil {
			assert.Equal(t, v.expected, actual)
		} else {
			assert.Nil(t, actual)
		}
	}
}

func TestWorkerLookup(t *testing.T) {
	cases := []struct {
		name     string
		worker   *worker
		work     *proto.Node
		expected []*proto.Node
	}{
		{
			name: "test valid chore returned",
			worker: func() *worker {
				nc, err := node.NewNodeClient(proto.Node{Id: "foo", Address: &proto.NodeAddress{Address: ":7070"}})
				assert.NoError(t, err)
				return newWorker(context.Background(), nil, []*proto.Node{&proto.Node{Id: "foo"}}, nc, node.StringToID("foo"), 5)
			}(),
			work:     &proto.Node{Id: "foo", Address: &proto.NodeAddress{Address: ":8080"}},
			expected: []*proto.Node{&proto.Node{Id: "foo"}},
		},
	}

	for _, v := range cases {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
		assert.NoError(t, err)

		srv, mock := newTestServer(nil)
		go srv.Serve(lis)
		defer srv.Stop()
		actual := v.worker.lookup(context.Background(), v.work)
		assert.Equal(t, v.expected, actual)
		assert.Equal(t, 1, mock.queryCalled)
	}
}

func TestUpdate(t *testing.T) {
	cases := []struct {
		name                string
		worker              *worker
		input               []*proto.Node
		expectedQueueLength int
		expected            []*proto.Node
		expectedErr         error
	}{
		{
			name: "test nil nodes",
			worker: func() *worker {
				nc, err := node.NewNodeClient(proto.Node{Id: "foo", Address: &proto.NodeAddress{Address: ":7070"}})
				assert.NoError(t, err)
				return newWorker(context.Background(), nil, []*proto.Node{&proto.Node{Id: "0000"}}, nc, node.StringToID("foo"), 2)
			}(),
			expectedQueueLength: 1,
			input:               nil,
			expectedErr:         WorkerError.New("nodes must not be empty"),
			expected:            []*proto.Node{&proto.Node{Id: "0000"}},
		},
		{
			name: "test combined less than k",
			worker: func() *worker {
				nc, err := node.NewNodeClient(proto.Node{Id: "foo", Address: &proto.NodeAddress{Address: ":7070"}})
				assert.NoError(t, err)
				return newWorker(context.Background(), nil, []*proto.Node{&proto.Node{Id: "0001"}}, nc, node.StringToID("1100"), 2)
			}(),
			expectedQueueLength: 2,
			expected:            []*proto.Node{&proto.Node{Id: "0001"}, &proto.Node{Id: "1001"}},
			input:               []*proto.Node{&proto.Node{Id: "1001"}, &proto.Node{Id: "0100"}},
			expectedErr:         nil,
		},
	}

	for _, v := range cases {
		err := v.worker.update(v.input)
		if v.expectedErr != nil || err != nil {
			assert.Equal(t, v.expectedErr.Error(), err.Error())
		}

		assert.Len(t, v.worker.pq, v.expectedQueueLength)

		i := 0
		for v.worker.pq.Len() > 0 {
			assert.Equal(t, v.expected[i], v.worker.pq.Pop().(*Item).value)
			i++
		}
	}
}

func newTestServer(nn []*proto.Node) (*grpc.Server, *mockNodeServer) {
	grpcServer := grpc.NewServer()
	mn := &mockNodeServer{queryCalled: 0}

	proto.RegisterNodesServer(grpcServer, mn)
	proto.RegisterOverlayServer(grpcServer, test.NewMockOverlay(nn))

	return grpcServer, mn
}

type mockNodeServer struct {
	queryCalled int
	returnValue []*proto.Node
}

func (mn *mockNodeServer) Query(ctx context.Context, req *proto.QueryRequest) (*proto.QueryResponse, error) {
	mn.queryCalled++

	return &proto.QueryResponse{Response: mn.returnValue}, nil

}
