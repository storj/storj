// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"testing"

	"crypto/ecdsa"

	"github.com/stretchr/testify/assert"
)

type tlsFileOptionsTestCase struct {
	tlsFileOptions *TLSHelper
	before         func(*tlsFileOptionsTestCase) error
	after          func(*tlsFileOptionsTestCase) error
}

func TestNewTLSHelper(t *testing.T) {
	opts, err := NewTLSHelper(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, opts.cert)
	assert.NotEmpty(t, opts.cert.PrivateKey)
	assert.NotNil(t, opts.cert.Leaf)
	assert.NotNil(t, opts.cert.Leaf.PublicKey.(*ecdsa.PublicKey))
}

func TestGenerate(t *testing.T) {
	opts := &TLSHelper{}

	cert, rootKey, err := generateTLS()
	assert.NoError(t, err)

	opts.cert = cert

	assert.NotNil(t, rootKey)
	assert.NotEmpty(t, *rootKey)
	assert.NotNil(t, opts.cert)
	assert.NotNil(t, opts.cert.PrivateKey)
	assert.NotNil(t, opts.cert.Leaf)
	assert.NotNil(t, opts.cert.Leaf.PublicKey.(*ecdsa.PublicKey))
	assert.NotEmpty(t, *opts.cert.Leaf.PublicKey.(*ecdsa.PublicKey))

	err = VerifyPeerCertificate(opts.cert.Certificate, nil)
	assert.NoError(t, err)
}

func TestNewTLSConfig(t *testing.T) {
	opts, err := NewTLSHelper(nil)
	assert.NoError(t, err)

	config := opts.NewTLSConfig(nil)
	assert.Equal(t, opts.cert, config.Certificates[0])
}
