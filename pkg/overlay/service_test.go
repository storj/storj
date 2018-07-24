// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/test"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

func newTestService(t *testing.T) Service {
	return Service{
		logger:  zap.NewNop(),
		metrics: monkit.Default,
	}
}

func TestNewServer(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv := newMockServer()
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient_CreateTLS(t *testing.T) {
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

func TestNewClient_LoadTLS(t *testing.T) {
	var err error

	tmpPath, err := ioutil.TempDir("", "TestNewClient")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpPath)

	basePath := filepath.Join(tmpPath, "TestNewClient_LoadTLS")
	_, err = peertls.NewTLSFileOptions(
		basePath,
		basePath,
		true,
		false,
	)

	assert.NoError(t, err)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	// NB: do NOT create a cert, it should be loaded from disk
	srv, tlsOpts := newMockTLSServer(t, basePath, false)

	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(address, tlsOpts.DialOption())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewClient_IndependentTLS(t *testing.T) {
	var err error

	tmpPath, err := ioutil.TempDir("", "TestNewClient_IndependentTLS")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpPath)

	clientBasePath := filepath.Join(tmpPath, "client")
	serverBasePath := filepath.Join(tmpPath, "server")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv, _ := newMockTLSServer(t, serverBasePath, true)

	go srv.Serve(lis)
	defer srv.Stop()

	clientTLSOps, err := peertls.NewTLSFileOptions(
		clientBasePath,
		clientBasePath,
		true,
		false,
	)

	assert.NoError(t, err)

	address := lis.Addr().String()
	c, err := NewClient(address, clientTLSOps.DialOption())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestProcess_redis(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestProcess_redis")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	viper.Set("localPort", "0")
	viper.Set("redisaddress", "127.0.0.1:6379")
	defer viper.Set("redisaddress", "")

	done := test.EnsureRedis(t)
	defer done()

	o := newTestService(t)
	ctx, _ := context.WithTimeout(context.Background(), 500*time.Millisecond)

	err = o.Process(ctx, nil, nil)
	assert.NoError(t, err)
}

func TestProcess_bolt(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestProcess_bolt")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	boltdbPath, err := filepath.Abs("test_bolt.db")
	viper.Set("localport", "0")
	viper.Set("boltdbpath", boltdbPath)
	defer viper.Set("boltdbpath", "")
	defer viper.Set("redisaddress", "")

	assert.NoError(t, err)

	if err != nil {
		defer func() {
			if err := os.Remove(boltdbPath); err != nil {
				log.Printf("%s\n", errs.New("error while removing test bolt db: %s", err))
			}
		}()
	}

	o := newTestService(t)
	ctx, _ := context.WithTimeout(context.Background(), 500*time.Millisecond)
	process.ConfigEnvironment()
	err = o.Process(ctx, nil, nil)
	assert.NoError(t, err)
}

func TestProcess_default(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestProcess_error")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	viper.Set("localPort", "0")
	viper.Set("boltdbpath", defaultBoltDBPath())
	defer viper.Set("boltdbpath", "")
	defer viper.Set("redisaddress", "")

	o := newTestService(t)
	ctx, _ := context.WithTimeout(context.Background(), 500*time.Millisecond)
	err = o.Process(ctx, nil, nil)
	assert.Nil(t, err)
}

func newMockServer(opts ...grpc.ServerOption) *grpc.Server {
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterOverlayServer(grpcServer, &MockOverlay{})

	return grpcServer
}

func newMockTLSServer(t *testing.T, tlsBasePath string, create bool) (*grpc.Server, *peertls.TLSFileOptions) {
	tlsOpts, err := peertls.NewTLSFileOptions(
		tlsBasePath,
		tlsBasePath,
		create,
		false,
	)
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
