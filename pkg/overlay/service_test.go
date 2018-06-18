// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/test"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/utils"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

func setTLSFlags(basePath string, create bool) {
	var createString string
	if create {
		createString = "true"
	} else {
		createString = "false"
	}

	flag.Set("tlsCertPath", fmt.Sprintf("%s.crt", basePath))
	flag.Set("tlsKeyPath", fmt.Sprintf("%s.key", basePath))
	flag.Set("tlsCreate", createString)
	flag.Set("tlsHosts", "localhost,127.0.0.1,::")
}

func setPortFlags(t *testing.T) {
	availablePort, err := test.NewPort()
	assert.NoError(t, err)

	flag.Set("localPort", strconv.Itoa(availablePort))
}

func newTestService(t *testing.T) Service {
	return Service{
		logger:  zap.NewNop(),
		metrics: monkit.Default,
	}
}

func TestNewServer(t *testing.T) {
	t.SkipNow()
	var err error

	tempPath, err := ioutil.TempDir("", "TestNewServer")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempPath)

	setTLSFlags(filepath.Join(tempPath, "TestNewServer"), true)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv, err := NewServer(nil, nil, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient_CreateTLS(t *testing.T) {
	t.SkipNow()
	var err error

	tmpPath, err := ioutil.TempDir("", "TestNewClient")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpPath)

	setTLSFlags(filepath.Join(tmpPath, "TestNewClient_CreateTLS"), true)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv, err := NewServer(nil, nil, nil, nil)
	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address)
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewClient_LoadTLS(t *testing.T) {
	t.SkipNow()
	var err error

	tmpPath, err := ioutil.TempDir("", "TestNewClient")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpPath)

	basePath := filepath.Join(tmpPath, "TestNewClient_LoadTLS")
	tlsOpts := &utils.TLSFileOptions{
		CertRelPath: fmt.Sprintf("%s.crt", basePath),
		KeyRelPath:  fmt.Sprintf("%s.key", basePath),
		Hosts:       "localhost,127.0.0.1,::",
		Create:      true,
	}

	// Ensure cert/key have been generated
	err = tlsOpts.EnsureExists()
	assert.NoError(t, err)

	// NB: do NOT create a cert, it should be loaded from disk
	setTLSFlags(basePath, false)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv, err := NewServer(nil, nil, nil, nil)
	assert.NoError(t, err)

	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address)
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestProcess_redis(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestProcess_redis")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	setTLSFlags(filepath.Join(tempPath, "TestProcess_redis"), true)
	setPortFlags(t)
	done := test.EnsureRedis(t)
	defer done()

	o := newTestService(t)
	ctx, _ := context.WithTimeout(context.Background(), 500*time.Millisecond)
	err = o.Process(ctx)
	assert.NoError(t, err)
}

func TestProcess_bolt(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestProcess_bolt")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	setTLSFlags(filepath.Join(tempPath, "TestProcess_bolt"), true)
	setPortFlags(t)
	flag.Set("redisAddress", "")
	boltdbPath, err := filepath.Abs("test_bolt.db")
	assert.NoError(t, err)

	if err != nil {
		defer func() {
			if err := os.Remove(boltdbPath); err != nil {
				log.Printf("%s\n", errs.New("error while removing test bolt db: %s", err))
			}
		}()
	}

	flag.Set("boltdbPath", boltdbPath)

	o := newTestService(t)
	ctx, _ := context.WithTimeout(context.Background(), 500*time.Millisecond)
	err = o.Process(ctx)
	assert.NoError(t, err)
}

func TestProcess_error(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestProcess_error")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	setTLSFlags(filepath.Join(tempPath, "TestProcess_error"), true)
	setPortFlags(t)
	flag.Set("boltdbPath", "")
	flag.Set("redisAddress", "")

	o := newTestService(t)
	ctx, _ := context.WithTimeout(context.Background(), 500*time.Millisecond)
	err = o.Process(ctx)
	assert.True(t, process.ErrUsage.Has(err))
}
