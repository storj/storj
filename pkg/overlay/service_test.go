// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/peertls"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

func TestNewServer(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv := newMockServer()
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient_SingleIdentity(t *testing.T) {
	var err error

	tmpPath, err := ioutil.TempDir("", "TestNewClient")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpPath)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	basePath := filepath.Join(tmpPath, "TestNewClient_CreateTLS")
	srv, tlsOpts := newMockTLSServer(t, basePath, true)
	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(address, tlsOpts.DialOption())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewClient_IndependentIdentities(t *testing.T) {
	var err error

	tmpPath, err := ioutil.TempDir("", "TestNewClient_IndependentTLS")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpPath)

	serverBasePath := filepath.Join(tmpPath, "server")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv, _ := newMockTLSServer(t, serverBasePath, true)

	go srv.Serve(lis)
	defer srv.Stop()

	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	address := lis.Addr().String()
	c, err := NewClient(address, tlsH.DialOption())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func newMockServer(opts ...grpc.ServerOption) *grpc.Server {
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterOverlayServer(grpcServer, &MockOverlay{})

	return grpcServer
}

func newMockTLSServer(t *testing.T, tlsBasePath string, create bool) (*grpc.Server, *peertls.TLSHelper) {
	tlsOpts, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)
	assert.NotNil(t, tlsOpts)

	grpcServer := newMockServer(tlsOpts.ServerOption())
	return grpcServer, tlsOpts
}

type MockOverlay struct{}

func (o *MockOverlay) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	return &proto.FindStorageNodesResponse{}, nil
}

func (o *MockOverlay) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	return &proto.LookupResponse{}, nil
}
