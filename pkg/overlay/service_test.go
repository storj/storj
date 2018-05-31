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
)

var (
  tempPath string
  basePath string
)

func setFlags() {
  basePath = filepath.Join(tempPath, "x509")

  flag.Set("tlsCertPath", fmt.Sprintf("%s.crt", basePath))
  flag.Set("tlsKeyPath", fmt.Sprintf("%s.key", basePath))
  flag.Set("tlsCreate", "true")
  flag.Set("tlsHosts", "localhost,127.0.0.1,::")
}

func TestNewServer(t *testing.T) {
  var err error

  tempPath, err = ioutil.TempDir("", "TestNewServer")
  if err != nil {
    panic(err)
  }
  defer os.RemoveAll(tempPath)

  setFlags()

  lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
  assert.NoError(t, err)

  srv, err := NewServer()
  assert.NoError(t, err)
  assert.NotNil(t, srv)

  go srv.Serve(lis)
  srv.Stop()
}

func TestNewClient(t *testing.T) {
  var err error

  tempPath, err = ioutil.TempDir("", "TestNewClient")
  if err != nil {
    panic(err)
  }
  defer os.RemoveAll(tempPath)

  setFlags()

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
