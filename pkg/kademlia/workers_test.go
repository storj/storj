// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

func TestUpdate(t *testing.T) {
	cases := []struct {
		name        string
		worker      *worker
		input       []*proto.Node
		self        proto.Node
		ctx         context.Context
		expected    map[string]*chore
		expectedErr error
	}{
		{
			name: "test nil nodes",
		},
		{
			name: "test combined less than k",
		},
		{
			name: "test proper node removed from working set",
		},
		{
			name: "test no node removed from working set",
		},
	}

	for _, v := range cases {
		v.worker.update(v.input)
	}
}

func TestWork(t *testing.T) {
	mu := &sync.Mutex{}
	ctx, cf := context.WithCancel(context.Background())
	cases := []struct {
		worker      *worker
		self        proto.Node
		ctx         context.Context
		expected    map[string]*chore
		expectedErr error
	}{
		{
			ctx:  ctx,
			self: proto.Node{Id: "hello", Address: &proto.NodeAddress{Address: ":7070"}},
			worker: &worker{
				workingSet: map[string]*chore{
					"foo": &chore{status: uncontacted, node: &proto.Node{Id: "foo", Address: &proto.NodeAddress{Address: ":8080"}}},
				},
				mu:          mu,
				maxResponse: 1 * time.Second,
				cancel:      cf,
				find:        proto.Node{Id: "foo"},
				k:           5,
			},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		nc, err := node.NewNodeClient(v.self)
		assert.NoError(t, err)
		v.worker.nodeClient = nc

		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
		assert.NoError(t, err)

		srv, mock := newTestServer()
		go srv.Serve(lis)
		defer srv.Stop()

		if err := v.worker.work(v.ctx); err != nil || v.expectedErr != nil {
			fmt.Printf("ERROR = %#v\n", err)
			assert.Equal(t, v.expectedErr, err)
		}

		assert.Equal(t, 1, mock.queryCalled)

	}
}

func newTestServer() (*grpc.Server, *mockNodeServer) {
	grpcServer := grpc.NewServer()
	mn := &mockNodeServer{queryCalled: 0}

	proto.RegisterNodesServer(grpcServer, mn)

	return grpcServer, mn
}

type mockNodeServer struct {
	queryCalled int
}

func (mn *mockNodeServer) Query(ctx context.Context, req *proto.QueryRequest) (*proto.QueryResponse, error) {
	mn.queryCalled++
	return &proto.QueryResponse{}, nil
}
