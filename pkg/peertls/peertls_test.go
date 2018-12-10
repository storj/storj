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

func TestNewCert_CA(t *testing.T) {
	caKey, err := NewKey()
	assert.NoError(t, err)

	caTemplate, err := CATemplate()
	assert.NoError(t, err)

	caCert, err := NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	assert.NotEmpty(t, caKey.(*ecdsa.PrivateKey))
	assert.NotEmpty(t, caCert)
	assert.NotEmpty(t, caCert.PublicKey.(*ecdsa.PublicKey))

	err = caCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
}

func TestNewCert_Leaf(t *testing.T) {
	caKey, err := NewKey()
	assert.NoError(t, err)

	caTemplate, err := CATemplate()
	assert.NoError(t, err)

	caCert, err := NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	leafKey, err := NewKey()
	assert.NoError(t, err)

	leafTemplate, err := LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := NewCert(leafKey, caKey, leafTemplate, caCert)
	assert.NoError(t, err)

	assert.NotEmpty(t, caKey.(*ecdsa.PrivateKey))
	assert.NotEmpty(t, leafCert)
	assert.NotEmpty(t, leafCert.PublicKey.(*ecdsa.PublicKey))

	err = caCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
	err = leafCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
}

func TestVerifyPeerFunc(t *testing.T) {
	caKey, err := NewKey()
	assert.NoError(t, err)

	caTemplate, err := CATemplate()
	assert.NoError(t, err)

	caCert, err := NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	leafKey, err := NewKey()
	assert.NoError(t, err)

	leafTemplate, err := LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := NewCert(leafKey, caKey, leafTemplate, caCert)
	assert.NoError(t, err)

	testFunc := func(chain [][]byte, parsedChains [][]*x509.Certificate) error {
		switch {
		case !bytes.Equal(chain[1], caCert.Raw):
			return errs.New("CA cert doesn't match")
		case !bytes.Equal(chain[0], leafCert.Raw):
			return errs.New("leaf's CA cert doesn't match")
		case leafCert.PublicKey.(*ecdsa.PublicKey).Curve != parsedChains[0][0].PublicKey.(*ecdsa.PublicKey).Curve:
			return errs.New("leaf public key doesn't match")
		case leafCert.PublicKey.(*ecdsa.PublicKey).X.Cmp(parsedChains[0][0].PublicKey.(*ecdsa.PublicKey).X) != 0:
			return errs.New("leaf public key doesn't match")
		case leafCert.PublicKey.(*ecdsa.PublicKey).Y.Cmp(parsedChains[0][0].PublicKey.(*ecdsa.PublicKey).Y) != 0:
			return errs.New("leaf public key doesn't match")
		case !bytes.Equal(parsedChains[0][1].Raw, caCert.Raw):
			return errs.New("parsed CA cert doesn't match")
		case !bytes.Equal(parsedChains[0][0].Raw, leafCert.Raw):
			return errs.New("parsed leaf cert doesn't match")
		}
		return nil
	}

	err = VerifyPeerFunc(testFunc)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)
}

func TestVerifyPeerCertChains(t *testing.T) {
	caKey, err := NewKey()
	assert.NoError(t, err)

	caTemplate, err := CATemplate()
	assert.NoError(t, err)

	caCert, err := NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	leafKey, err := NewKey()
	assert.NoError(t, err)

	leafTemplate, err := LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := NewCert(leafKey, caKey, leafTemplate, caCert)
	assert.NoError(t, err)

	err = VerifyPeerFunc(VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	wrongKey, err := NewKey()
	assert.NoError(t, err)

	leafCert, err = NewCert(leafKey, wrongKey, leafTemplate, caCert)
	assert.NoError(t, err)

	err = VerifyPeerFunc(VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.True(t, ErrVerifyPeerCert.Has(err))
	assert.True(t, ErrVerifyCertificateChain.Has(err))
}

func TestVerifyCAWhitelist(t *testing.T) {
	caKey, err := NewKey()
	assert.NoError(t, err)

	caTemplate, err := CATemplate()
	assert.NoError(t, err)

	caCert, err := NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	leafKey, err := NewKey()
	assert.NoError(t, err)

	leafTemplate, err := LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := NewCert(leafKey, caKey, leafTemplate, caCert)
	assert.NoError(t, err)

	// empty whitelist
	err = VerifyPeerFunc(VerifyCAWhitelist(nil))([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	// whitelist contains ca
	err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{caCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	rootKey, err := NewKey()
	assert.NoError(t, err)

	rootTemplate, err := CATemplate()
	assert.NoError(t, err)

	rootCert, err := NewCert(rootKey, nil, rootTemplate, nil)
	assert.NoError(t, err)

	// no valid signed extension, non-empty whitelist
	err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{rootCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.True(t, ErrVerifyCAWhitelist.Has(err))

	// last cert in whitelist is signer
	err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{rootCert, caCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	// first cert in whitelist is signer
	err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{caCert, rootCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	ca2Cert, err := NewCert(caKey, rootKey, caTemplate, rootCert)
	assert.NoError(t, err)

	leaf2Cert, err := NewCert(leafKey, caKey, leafTemplate, ca2Cert)
	assert.NoError(t, err)

	// length 3 chain; first cert in whitelist is signer
	err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{rootCert, caCert}))([][]byte{leaf2Cert.Raw, ca2Cert.Raw, rootCert.Raw}, nil)
	assert.NoError(t, err)

	// length 3 chain; last cert in whitelist is signer
	err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{caCert, rootCert}))([][]byte{leaf2Cert.Raw, ca2Cert.Raw, rootCert.Raw}, nil)
	assert.NoError(t, err)
}

func TestSignLeafExt(t *testing.T) {
	caKey, err := NewKey()
	assert.NoError(t, err)

	caTemplate, err := CATemplate()
	assert.NoError(t, err)

	caCert, err := NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	leafKey, err := NewKey()
	assert.NoError(t, err)

	leafTemplate, err := LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := NewCert(leafKey, caKey, leafTemplate, caCert)
	assert.NoError(t, err)

	err = AddSignedLeafExt(caKey, leafCert)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(leafCert.ExtraExtensions))
	assert.True(t, ExtensionIDs[SignedCertExtID].Equal(leafCert.ExtraExtensions[0].Id))

	caECKey, ok := caKey.(*ecdsa.PrivateKey)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	err = VerifySignature(leafCert.ExtraExtensions[0].Value, leafCert.RawTBSCertificate, &caECKey.PublicKey)
	assert.NoError(t, err)
}

func TestParseExtensions(t *testing.T) {
	type result struct {
		ok  bool
		err error
	}

	rootKey, err := NewKey()
	assert.NoError(t, err)

	caKey, err := NewKey()
	assert.NoError(t, err)

	caTemplate, err := CATemplate()
	assert.NoError(t, err)

	rootCert, err := NewCert(rootKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	caCert, err := NewCert(caKey, rootKey, caTemplate, rootCert)
	assert.NoError(t, err)

	leafKey, err := NewKey()
	assert.NoError(t, err)

	leafTemplate, err := LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := NewCert(leafKey, rootKey, leafTemplate, caCert)
	assert.NoError(t, err)

	err = AddSignedLeafExt(rootKey, leafCert)
	assert.NoError(t, err)

	whitelist := []*x509.Certificate{rootCert}

	cases := []struct {
		testID    string
		config    TLSExtConfig
		whitelist []*x509.Certificate
		expected  []result
	}{
		{
			"leaf whitelist signature",
			TLSExtConfig{WhitelistSignedLeaf: true},
			whitelist,
			[]result{{true, nil}},
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			exts := ParseExtensions(c.config, c.whitelist)
			assert.Equal(t, 1, len(exts))
			for i, e := range exts {
				ok, err := e.f(leafCert.ExtraExtensions[0], [][]*x509.Certificate{{leafCert, caCert, rootCert}})
				assert.Equal(t, c.expected[i].err, err)
				assert.Equal(t, c.expected[i].ok, ok)
			}
		})
	}
}
