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
	tlsH, err := NewTLSHelper(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, tlsH.cert)
	assert.NotEmpty(t, tlsH.cert.PrivateKey)
	assert.NotNil(t, tlsH.cert.Leaf)
	assert.NotNil(t, tlsH.cert.Leaf.PublicKey.(*ecdsa.PublicKey))
}

func TestGenerate(t *testing.T) {
	tlsH := &TLSHelper{}

	cert, rootKey, err := generateTLS()
	assert.NoError(t, err)

	tlsH.cert = cert

	assert.NotNil(t, rootKey)
	assert.NotEmpty(t, *rootKey)
	assert.NotNil(t, tlsH.cert)
	assert.NotNil(t, tlsH.cert.PrivateKey)
	assert.NotNil(t, tlsH.cert.Leaf)
	assert.NotNil(t, tlsH.cert.Leaf.PublicKey.(*ecdsa.PublicKey))
	assert.NotEmpty(t, *tlsH.cert.Leaf.PublicKey.(*ecdsa.PublicKey))

	err = VerifyPeerFunc(nil)(tlsH.cert.Certificate, nil)
	assert.NoError(t, err)
}

func TestNewTLSConfig(t *testing.T) {
	opts, err := NewTLSHelper(nil)
	assert.NoError(t, err)

	config := opts.NewTLSConfig(nil)
	assert.Equal(t, opts.cert, config.Certificates[0])
}
