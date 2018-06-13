// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/test"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

func TestNewServer(t *testing.T) {
	t.SkipNow()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv := NewServer(nil, nil, nil, nil)
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient(t *testing.T) {
	//a := "35.232.202.229:8080"
	//c, err := NewClient(&a, grpc.WithInsecure())
	t.SkipNow()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv := NewServer(nil, nil, nil, nil)
	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestProcess(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	o := Service{}
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	err := o.Process(ctx)
	assert.NoError(t, err)
}
