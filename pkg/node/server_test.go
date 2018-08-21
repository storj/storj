// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"testing"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/dht/mocks"
	proto "storj.io/storj/protos/overlay"
)

func TestQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dht := mock_dht.NewMockDHT(ctrl)
	s := &Server{dht: dht}

	cases := []struct {
		QR proto.QueryRequest
		getRT bool
		pingNode proto.Node
		pingError error
		success error
		failed error
		findNear []*proto.Node
		Res proto.QueryResponse

	}{
		{QR: proto.QueryRequest{Sender: , Target: , Limit: },
			getRT: ,
			pingNode: ,
			pingErr:, 
			success:,
			failed:,
			findNear: 
			Res: 
		},
		{QR: proto.QueryRequest{Sender: , Target: , Limit: },
			getRT: ,
			ping: ,
			success:,
			failed:,
			findNear: 
		},
		{QR: proto.QueryRequest{Sender: , Target: , Limit: },
			getRT: ,
			ping: ,
			success:,
			failed:,
			findNear: 
		},
		{QR: proto.QueryRequest{Sender: , Target: , Limit: },
			getRT: ,
			ping: ,
			success:,
			failed:,
			findNear: 
		},
		{QR: proto.QueryRequest{Sender: , Target: , Limit: },
			getRT: ,
			ping: ,
			success:,
			failed:,
			findNear: 
		},
	}
	for i, v := range cases {
		dht.EXPECT().Ping(gomock.Any(), gomock.Any()).Return()			
	}


	req := proto.QueryRequest{}
	res, err := s.Query(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
}