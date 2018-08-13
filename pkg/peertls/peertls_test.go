// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
)

func TestGenerate_CA(t *testing.T) {
	k, err := NewKey()
	assert.NoError(t, err)

	ct, err := CATemplate()
	assert.NoError(t, err)

	c, err := NewCert(ct, nil, k)
	assert.NoError(t, err)

	assert.NotEmpty(t, k.(*ecdsa.PrivateKey))
	assert.NotEmpty(t, c)
	assert.NotEmpty(t, c.PublicKey.(*ecdsa.PublicKey))

	err = c.CheckSignatureFrom(c)
	assert.NoError(t, err)
}

func TestGenerate_Leaf(t *testing.T) {
	k, err := NewKey()
	assert.NoError(t, err)

	ct, err := CATemplate()
	assert.NoError(t, err)

	c, err := NewCert(ct, nil, k)
	assert.NoError(t, err)

	lt, err := LeafTemplate()
	assert.NoError(t, err)

	l, err := NewCert(lt, ct, k)
	assert.NoError(t, err)

	assert.NotEmpty(t, k.(*ecdsa.PrivateKey))
	assert.NotEmpty(t, l)
	assert.NotEmpty(t, l.PublicKey.(*ecdsa.PublicKey))

	err = l.CheckSignatureFrom(c)
	assert.NoError(t, err)
}

func TestVerifyPeerFunc(t *testing.T) {
	k, err := NewKey()
	assert.NoError(t, err)

	ct, err := CATemplate()
	assert.NoError(t, err)

	c, err := NewCert(ct, nil, k)
	assert.NoError(t, err)

	lt, err := LeafTemplate()
	assert.NoError(t, err)

	l, err := NewCert(lt, ct, k)
	assert.NoError(t, err)

	testFunc := func(chain [][]byte, parsedChains [][]*x509.Certificate) error {
		switch {
		case bytes.Compare(chain[1], c.Raw) != 0:
			return errs.New("CA cert doesn't match")
		case bytes.Compare(chain[0], l.Raw) != 0:
			return errs.New("leaf's CA cert doesn't match")
		case l.PublicKey.(*ecdsa.PublicKey).Curve != parsedChains[0][0].PublicKey.(*ecdsa.PublicKey).Curve:
			return errs.New("leaf public key doesn't match")
		case l.PublicKey.(*ecdsa.PublicKey).X.Cmp(parsedChains[0][0].PublicKey.(*ecdsa.PublicKey).X) != 0:
			return errs.New("leaf public key doesn't match")
		case l.PublicKey.(*ecdsa.PublicKey).Y.Cmp(parsedChains[0][0].PublicKey.(*ecdsa.PublicKey).Y) != 0:
			return errs.New("leaf public key doesn't match")
		case bytes.Compare(parsedChains[0][1].Raw, c.Raw) != 0:
			return errs.New("parsed CA cert doesn't match")
		case bytes.Compare(parsedChains[0][0].Raw, l.Raw) != 0:
			return errs.New("parsed leaf cert doesn't match")
		}
		return nil
	}

	err = VerifyPeerFunc(testFunc)([][]byte{l.Raw, c.Raw}, nil)
	assert.NoError(t, err)
}
