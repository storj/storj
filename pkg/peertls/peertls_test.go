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
	ct, err := CATemplate()
	assert.NoError(t, err)

	c, err := Generate(ct, nil, nil, nil)
	assert.NoError(t, err)

	assert.NotEmpty(t, c)
	assert.NotEmpty(t, c.PrivateKey.(*ecdsa.PrivateKey))
	assert.NotEmpty(t, c.Leaf)
	assert.NotEmpty(t, c.Leaf.PublicKey.(*ecdsa.PublicKey))
}

func TestGenerate_Leaf(t *testing.T) {
	ct, err := CATemplate()
	assert.NoError(t, err)

	c, err := Generate(ct, nil, nil, nil)
	assert.NoError(t, err)

	lt, err := CATemplate()
	assert.NoError(t, err)

	l, err := Generate(lt, ct, c, c)
	assert.NoError(t, err)

	ca, err := x509.ParseCertificate(l.Certificate[1])
	assert.Equal(t, c.Certificate[0], ca.Raw)
	assert.Equal(t, c.Leaf.PublicKey, ca.PublicKey)

	lf, err := x509.ParseCertificate(l.Certificate[0])
	assert.Equal(t, ca.Raw, l.Certificate[1])
	assert.Equal(t, l.Certificate[0], lf.Raw)
	assert.Equal(t, l.Leaf.PublicKey, lf.PublicKey)
}

func TestVerifyPeerFunc(t *testing.T) {
	ct, err := CATemplate()
	assert.NoError(t, err)

	c, err := Generate(ct, nil, nil, nil)
	assert.NoError(t, err)

	lt, err := CATemplate()
	assert.NoError(t, err)

	l, err := Generate(lt, ct, c, c)
	assert.NoError(t, err)

	testFunc := func(chain [][]byte, certs [][]*x509.Certificate) error {
		ca, err := x509.ParseCertificate(l.Certificate[1])
		if err != nil {
			return err
		}
		lf, err := x509.ParseCertificate(l.Certificate[0])
		if err != nil {
			return err
		}

		switch true {
		case bytes.Compare(c.Certificate[0], ca.Raw) != 0:
			return errs.New("CA cert doesn't match")
		case bytes.Compare(l.Certificate[1], ca.Raw) != 0:
			return errs.New("leaf's CA cert doesn't match")
		case l.Leaf.PublicKey.(*ecdsa.PublicKey).Curve != lf.PublicKey.(*ecdsa.PublicKey).Curve:
			return errs.New("leaf public key doesn't match")
		case l.Leaf.PublicKey.(*ecdsa.PublicKey).X.Cmp(lf.PublicKey.(*ecdsa.PublicKey).X) != 0:
			return errs.New("leaf public key doesn't match")
		case l.Leaf.PublicKey.(*ecdsa.PublicKey).Y.Cmp(lf.PublicKey.(*ecdsa.PublicKey).Y) != 0:
			return errs.New("leaf public key doesn't match")
		case bytes.Compare(l.Certificate[0], l.Leaf.Raw) != 0:
			return errs.New("leaf cert doesn't match")
		case bytes.Compare(certs[0][1].Raw, ca.Raw) != 0:
			return errs.New("parsed CA cert doesn't match")
		case bytes.Compare(certs[0][0].Raw, lf.Raw) != 0:
			return errs.New("parsed leaf cert doesn't match")
		}
		return nil
	}

	err = VerifyPeerFunc(testFunc)(l.Certificate, nil)
	assert.NoError(t, err)
}
