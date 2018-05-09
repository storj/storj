// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	proto "storj.io/storj/protos/netstate"
)

func TestNetStateClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 9000))
	assert.NoError(t, err)

	srv := NewServer(logger, mockDB{
		timesCalled: 0,
	})
	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address, grpc.WithInsecure())
	assert.NoError(t, err)

	ctx := context.Background()

	// example file path to put/get
	fp := proto.FilePath{
		Path:       "here/is/a/path",
		SmallValue: "oatmeal",
	}

	// Tests NetState.Put
	c.Put(ctx, &fp)
}
