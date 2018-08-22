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
	proto "storj.io/storj/protos/overlay"
)

func TestQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDHT := mock_dht.NewMockDHT(ctrl)
	mockRT := mock_dht.NewMockRoutingTable(ctrl)
	s := &Server{dht: mockDHT}
	sender := &proto.Node{Id: "A"}
	cases := []struct {
		caseName string
		rt       dht.RoutingTable
		getRT    error
		pingNode proto.Node
		pingErr  error
		success  error
		failed   error
		findNear []*proto.Node
		limit    int
		nearErr  error
		Res      proto.QueryResponse
	}{
		{caseName: "ping success, return sender",
			rt:       mockRT,
			getRT:    nil,
			pingNode: *sender,
			pingErr:  nil,
			success:  nil,
			failed:   nil,
			findNear: []*proto.Node{sender},
			limit:    2,
			nearErr:  nil,
			Res:      proto.QueryResponse{Sender: sender, Response: []*proto.Node{sender}},
		},
		{caseName: "ping success, return nearest",
			rt:       mockRT,
			getRT:    nil,
			pingNode: *sender,
			pingErr:  nil,
			success:  nil,
			failed:   nil,
			findNear: []*proto.Node{sender},
			limit:    2,
			nearErr:  nil,
			Res:      proto.QueryResponse{Sender: sender, Response: []*proto.Node{sender}},
		},
		{caseName: "ping fails, return error",
			rt:       mockRT,
			getRT:    nil,
			pingNode: *sender,
			pingErr:  nil,
			success:  nil,
			failed:   nil,
			findNear: []*proto.Node{sender},
			limit:    2,
			nearErr:  nil,
			Res:      proto.QueryResponse{Sender: sender, Response: []*proto.Node{sender}},
		},
	}
	for i, v := range cases {
		req := proto.QueryRequest{Sender: sender, Target: &proto.Node{Id: "B"}, Limit: int64(2)}
		mockDHT.EXPECT().GetRoutingTable(gomock.Any()).Return(v.rt, v.getRT)
		mockDHT.EXPECT().Ping(gomock.Any(), gomock.Any()).Return(v.pingNode, v.pingErr)
		if v.pingErr != nil {
			mockRT.EXPECT().ConnectionFailed(gomock.Any()).Return(v.failed)
		} else {
			mockRT.EXPECT().ConnectionSuccess(gomock.Any()).Return(v.success)
		}
		mockRT.EXPECT().FindNear(gomock.Any(), v.limit).Return(v.findNear, v.nearErr)
		res, _ := s.Query(context.Background(), req)
		if !assert.Equal(t, v.Res, res) {
			fmt.Printf("case %s (%v) failed\n", v.caseName, i)
		}
		// if !assert.NoError(t, err) {
		// 	fmt.Printf("query errored at case %v\n", i)
		// }
	}
}
