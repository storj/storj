// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/zeebo/errs"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
	"storj.io/storj/pkg/utils"
)


func TestNewServerGeneratesCerts(t *testing.T) {
	testCertPath := "./generate-me.cert"
	testKeyPath := "./generate-me.key"
	
	tlsCredFiles := &utils.TlsFileOpions{
		CertRelPath: testCertPath,
		KeyRelPath: testKeyPath,
	}
	
	srv, err := NewServer(tlsCredFiles)
	assert.NoError(t, err)
	assert.NotNil(t, srv)

}

func TestNewServer(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv, err := NewServer(nil)
	assert.NoError(t, err)
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient(t *testing.T) {

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv, err := NewServer(nil)
	assert.NoError(t, err)

	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}
