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
)

type tlsCredFilesTestCase struct {
	tlsCredFiles *TlsCredFiles
	before func (*tlsCredFilesTestCase) (error)
	after func (*tlsCredFilesTestCase) (error)
}

func ensureRemoved(c *tlsCredFilesTestCase) (_ error) {
	creds := c.tlsCredFiles
	err := creds.ensureAbsPaths(); if err != nil {
		return err
	}

	fPaths := []string{creds.certAbsPath, creds.keyAbsPath}
	for _, fPath := range fPaths {
		err := os.Remove(fPath); if err != nil {
			return errs.New(err.Error())
		}
	}

	return nil
}

func TestTlsCredFiles(t *testing.T) {
	cases := []tlsCredFilesTestCase{
		{
			// generate cert/key with given filename
			tlsCredFiles: &TlsCredFiles{
				certRelPath: "./non-existent.cert",
				keyRelPath:  "./non-existent.key",
			},
			before: ensureRemoved,
			after: ensureRemoved,
		},
		{
			// use defaults
			tlsCredFiles: &TlsCredFiles{},
			after: ensureRemoved,
		},
	}

	for _, c := range cases {
		err := c.tlsCredFiles.ensureExists(); if err != nil {
			assert.NoError(t, err)
		}

		assert.NotEqual(t, c.tlsCredFiles.certAbsPath, "certAbsPath is an empty string")
		assert.NotEqual(t, c.tlsCredFiles.keyAbsPath, "keyAbsPath is an empty string")

		fPaths := []string{c.tlsCredFiles.certAbsPath, c.tlsCredFiles.keyAbsPath}
		for _, fPath := range fPaths {
			_, err := os.Stat(fPath)
			assert.NoError(t, err)
		}
	}
}

func TestNewServerGeneratesCerts(t *testing.T) {
	testCertPath := "./generate-me.cert"
	testKeyPath := "./generate-me.key"
	
	tlsCredFiles := &TlsCredFiles{
		certRelPath: testCertPath,
		keyRelPath: testKeyPath,
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
