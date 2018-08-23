// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/dht/mocks"
	proto "storj.io/storj/protos/overlay"
)

func TestQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDHT := mock_dht.NewMockDHT(ctrl)
	mockRT := mock_dht.NewMockRoutingTable(ctrl)
	s := &Server{dht: mockDHT}
	sender := &proto.Node{Id: "A"}
	target := &proto.Node{Id: "B"}
	node := &proto.Node{Id: "C"}
	cases := []struct {
		caseName   string
		rt         dht.RoutingTable
		getRTErr   error
		pingNode   proto.Node
		pingErr    error
		successErr error
		failErr    error
		findNear   []*proto.Node
		limit      int
		nearErr    error
		res        proto.QueryResponse
		err        error
	}{
		{caseName: "ping success, return sender",
			rt:         mockRT,
			getRTErr:   nil,
			pingNode:   *sender,
			pingErr:    nil,
			successErr: nil,
			failErr:    nil,
			findNear:   []*proto.Node{target},
			limit:      2,
			nearErr:    nil,
			res:        proto.QueryResponse{Sender: sender, Response: []*proto.Node{target}},
			err:        nil,
		},
		{caseName: "ping success, return nearest",
			rt:         mockRT,
			getRTErr:   nil,
			pingNode:   *sender,
			pingErr:    nil,
			successErr: nil,
			failErr:    nil,
			findNear:   []*proto.Node{sender, node},
			limit:      2,
			nearErr:    nil,
			res:        proto.QueryResponse{Sender: sender, Response: []*proto.Node{sender, node}},
			err:        nil,
		},
		{caseName: "ping success, connectionSuccess errors",
			rt:         mockRT,
			getRTErr:   nil,
			pingNode:   *sender,
			pingErr:    nil,
			successErr: errors.New("connection fails error"),
			failErr:    nil,
			findNear:   []*proto.Node{},
			limit:      2,
			nearErr:    nil,
			res:        proto.QueryResponse{},
			err:        errors.New("query error"),
		},
		{caseName: "ping fails, return error",
			rt:         mockRT,
			getRTErr:   nil,
			pingNode:   proto.Node{},
			pingErr:    errors.New("ping err"),
			successErr: nil,
			failErr:    nil,
			findNear:   []*proto.Node{},
			limit:      2,
			nearErr:    nil,
			res:        proto.QueryResponse{},
			err:        errors.New("query error"),
		},
		{caseName: "ping fails, connectionFailed errors",
			rt:         mockRT,
			getRTErr:   nil,
			pingNode:   proto.Node{},
			pingErr:    errors.New("ping err"),
			successErr: nil,
			failErr:    errors.New("connection fails error"),
			findNear:   []*proto.Node{},
			limit:      2,
			nearErr:    nil,
			res:        proto.QueryResponse{},
			err:        errors.New("query error"),
		},
	}
	for i, v := range cases {
		req := proto.QueryRequest{Sender: sender, Target: &proto.Node{Id: "B"}, Limit: int64(2)}
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
		res, err := s.Query(context.Background(), req)
		if !assert.Equal(t, v.res, res) {
			fmt.Printf("case %s (%v) failed\n", v.caseName, i)
		}
		if v.err == nil && !assert.NoError(t, err) {
			fmt.Printf("query errored at case %v\n", i)
		}
	}
}
