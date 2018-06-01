// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
  "context"
  "fmt"
  "net"
  "flag"
  "testing"
  "io/ioutil"
  "os"
  "path/filepath"

  "github.com/stretchr/testify/assert"

  proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
  "storj.io/storj/pkg/utils"
)

func setFlags(basePath string, create bool) {
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

func TestNewServer(t *testing.T) {
  var err error

  tempPath, err := ioutil.TempDir("", "TestNewServer")
  if err != nil {
    panic(err)
  }
  defer os.RemoveAll(tempPath)

  setFlags(filepath.Join(tempPath, "TestNewServer"), true)

  lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
  assert.NoError(t, err)

  srv, err := NewServer()
  assert.NoError(t, err)
  assert.NotNil(t, srv)

  go srv.Serve(lis)
  srv.Stop()
}

func TestNewClient_CreateTLS(t *testing.T) {
  var err error

  tmpPath, err := ioutil.TempDir("", "TestNewClient")
  if err != nil {
    panic(err)
  }
  defer os.RemoveAll(tmpPath)

  setFlags(filepath.Join(tmpPath, "TestNewClient_CreateTLS"), true)

  lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
  assert.NoError(t, err)
  srv, err := NewServer()
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

func TestNewClient_LoadTLS(t *testing.T) {
  var err error

  tmpPath, err := ioutil.TempDir("", "TestNewClient")
  if err != nil {
    panic(err)
  }
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
  setFlags(basePath, false)

  lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
  assert.NoError(t, err)
  srv, err := NewServer()
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
