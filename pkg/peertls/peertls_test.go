// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"testing"

	"crypto/ecdsa"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	leaf, ca, err := Generate()
	assert.NoError(t, err)

	assert.NotEmpty(t, leaf)
	assert.NotEmpty(t, leaf.PrivateKey.(*ecdsa.PrivateKey))
	assert.NotEmpty(t, leaf.Leaf)
	assert.NotEmpty(t, leaf.Leaf.PublicKey.(*ecdsa.PublicKey))

	assert.NotEmpty(t, ca)
	assert.NotEmpty(t, ca.PrivateKey.(*ecdsa.PrivateKey))
	assert.NotEmpty(t, ca.Leaf)
	assert.NotEmpty(t, ca.Leaf.PublicKey.(*ecdsa.PublicKey))

	err = VerifyPeerFunc(nil)(leaf.Certificate, nil)
	assert.NoError(t, err)
}
