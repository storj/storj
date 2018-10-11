// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/dht/mocks"
	"storj.io/storj/pkg/pb"
)

func TestQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDHT := mock_dht.NewMockDHT(ctrl)
	mockRT := mock_dht.NewMockRoutingTable(ctrl)
	s := &Server{dht: mockDHT}
	sender := &pb.Node{Id: "A"}
	target := &pb.Node{Id: "B"}
	node := &pb.Node{Id: "C"}
	cases := []struct {
		caseName   string
		rt         dht.RoutingTable
		getRTErr   error
		pingNode   pb.Node
		pingErr    error
		successErr error
		failErr    error
		findNear   []*pb.Node
		limit      int
		nearErr    error
		res        *pb.QueryResponse
		err        error
	}{
		{caseName: "ping success, return sender",
			rt:         mockRT,
			getRTErr:   nil,
			pingNode:   *sender,
			pingErr:    nil,
			successErr: nil,
			failErr:    nil,
			findNear:   []*pb.Node{target},
			limit:      2,
			nearErr:    nil,
			res:        &pb.QueryResponse{Sender: sender, Response: []*pb.Node{target}},
			err:        nil,
		},
		{caseName: "ping success, return nearest",
			rt:         mockRT,
			getRTErr:   nil,
			pingNode:   *sender,
			pingErr:    nil,
			successErr: nil,
			failErr:    nil,
			findNear:   []*pb.Node{sender, node},
			limit:      2,
			nearErr:    nil,
			res:        &pb.QueryResponse{Sender: sender, Response: []*pb.Node{sender, node}},
			err:        nil,
		},
	}
	for i, v := range cases {
		req := pb.QueryRequest{Pingback: true, Sender: sender, Target: &pb.Node{Id: "B"}, Limit: int64(2)}
		mockDHT.EXPECT().GetRoutingTable(gomock.Any()).Return(v.rt, v.getRTErr)
		mockDHT.EXPECT().Ping(gomock.Any(), gomock.Any()).Return(v.pingNode, v.pingErr)
		if v.pingErr != nil {
			mockRT.EXPECT().ConnectionFailed(gomock.Any()).Return(v.failErr)
		} else {
			mockRT.EXPECT().ConnectionSuccess(gomock.Any()).Return(v.successErr)
			if v.successErr == nil {
				mockRT.EXPECT().FindNear(gomock.Any(), v.limit).Return(v.findNear, v.nearErr)
			}
		}
		res, err := s.Query(context.Background(), &req)
		if !assert.Equal(t, v.res, res) {
			fmt.Printf("case %s (%v) failed\n", v.caseName, i)
		}
		if v.err == nil && !assert.NoError(t, err) {
			fmt.Printf("query errored at case %v\n", i)
		}
	}
}
