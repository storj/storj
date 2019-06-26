// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls_test

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/gob"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

func TestNewCert_CA(t *testing.T) {
	caKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.CreateSelfSignedCertificate(caKey, caTemplate)
	assert.NoError(t, err)

	assert.NotEmpty(t, caKey)
	assert.NotEmpty(t, caCert)
	assert.NotEmpty(t, caCert.PublicKey)

	err = caCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
}

func TestNewCert_Leaf(t *testing.T) {
	caKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.CreateSelfSignedCertificate(caKey, caTemplate)
	assert.NoError(t, err)

	leafKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := peertls.CreateCertificate(pkcrypto.PublicKeyFromPrivate(leafKey), caKey, leafTemplate, caCert)
	assert.NoError(t, err)

	assert.NotEmpty(t, caKey)
	assert.NotEmpty(t, leafCert)
	assert.NotEmpty(t, leafCert.PublicKey)

	err = caCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
	err = leafCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
}

func TestVerifyPeerFunc(t *testing.T) {
	_, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leafCert, caCert := chain[peertls.LeafIndex], chain[peertls.CAIndex]

	testFunc := func(chain [][]byte, parsedChains [][]*x509.Certificate) error {
		switch {
		case !bytes.Equal(chain[peertls.CAIndex], caCert.Raw):
			return errs.New("CA cert doesn't match")
		case !bytes.Equal(chain[peertls.LeafIndex], leafCert.Raw):
			return errs.New("leaf's CA cert doesn't match")
		case !pkcrypto.PublicKeyEqual(leafCert.PublicKey, parsedChains[0][0].PublicKey):
			return errs.New("leaf public key doesn't match")
		case !bytes.Equal(parsedChains[0][peertls.CAIndex].Raw, caCert.Raw):
			return errs.New("parsed CA cert doesn't match")
		case !bytes.Equal(parsedChains[0][peertls.LeafIndex].Raw, leafCert.Raw):
			return errs.New("parsed leaf cert doesn't match")
		}
		return nil
	}

	err = peertls.VerifyPeerFunc(testFunc)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)
}

func TestVerifyPeerCertChains(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leafKey, leafCert, caCert := keys[peertls.LeafIndex], chain[peertls.LeafIndex], chain[peertls.CAIndex]

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	wrongKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	leafCert, err = peertls.CreateCertificate(pkcrypto.PublicKeyFromPrivate(leafKey), wrongKey, leafCert, caCert)
	assert.NoError(t, err)

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	nonTempErr, ok := err.(peertls.NonTemporaryError)
	require.True(t, ok)
	assert.True(t, peertls.ErrVerifyPeerCert.Has(nonTempErr.Err()))
	assert.True(t, peertls.ErrVerifyCertificateChain.Has(nonTempErr.Err()))
}

func TestVerifyCAWhitelist(t *testing.T) {
	_, chain2, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leafCert, caCert := chain2[0], chain2[1]

	t.Run("empty whitelist", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist(nil))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	t.Run("whitelist contains ca", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{caCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	_, unrelatedChain, err := testpeertls.NewCertChain(1, storj.LatestIDVersion().Number)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	unrelatedCert := unrelatedChain[0]

	t.Run("no valid signed extension, non-empty whitelist", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{unrelatedCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		nonTempErr, ok := err.(peertls.NonTemporaryError)
		require.True(t, ok)
		assert.True(t, peertls.ErrVerifyCAWhitelist.Has(nonTempErr.Err()))
	})

	t.Run("last cert in whitelist is signer", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{unrelatedCert, caCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	t.Run("first cert in whitelist is signer", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{caCert, unrelatedCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	_, chain3, err := testpeertls.NewCertChain(3, storj.LatestIDVersion().Number)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leaf2Cert, ca2Cert, rootCert := chain3[0], chain3[1], chain3[2]

	t.Run("length 3 chain - first cert in whitelist is signer", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{rootCert, unrelatedCert}))([][]byte{leaf2Cert.Raw, ca2Cert.Raw, unrelatedCert.Raw}, nil)
		assert.NoError(t, err)
	})

	t.Run("length 3 chain - last cert in whitelist is signer", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{unrelatedCert, rootCert}))([][]byte{leaf2Cert.Raw, ca2Cert.Raw, unrelatedCert.Raw}, nil)
		assert.NoError(t, err)
	})
}

func TestAddExtraExtension(t *testing.T) {
	_, chain, err := testpeertls.NewCertChain(1, storj.LatestIDVersion().Number)
	require.NoError(t, err)

	cert := chain[0]
	extLen := len(cert.Extensions)

	randBytes := testrand.Bytes(10)
	ext := pkix.Extension{
		Id:    asn1.ObjectIdentifier{2, 999, int(randBytes[0])},
		Value: randBytes,
	}

	err = extensions.AddExtraExtension(cert, ext)
	assert.NoError(t, err)
	assert.Len(t, cert.ExtraExtensions, 1)
	require.Len(t, cert.Extensions, extLen)
	assert.Equal(t, ext, cert.ExtraExtensions[0])
}

func TestRevocation_Sign(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	assert.NoError(t, err)
	leafCert, caKey := chain[peertls.LeafIndex], keys[peertls.CAIndex]

	leafKeyHash, err := peertls.DoubleSHA256PublicKey(leafCert.PublicKey)
	require.NoError(t, err)

	rev := extensions.Revocation{
		Timestamp: time.Now().Unix(),
		KeyHash:   make([]byte, len(leafKeyHash)),
	}
	copy(rev.KeyHash, leafKeyHash[:])
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)
}

func TestRevocation_Verify(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	assert.NoError(t, err)
	leafCert, caCert, caKey := chain[peertls.LeafIndex], chain[peertls.CAIndex], keys[peertls.CAIndex]

	leafKeyHash, err := peertls.DoubleSHA256PublicKey(leafCert.PublicKey)
	require.NoError(t, err)

	rev := extensions.Revocation{
		Timestamp: time.Now().Unix(),
		KeyHash:   make([]byte, len(leafKeyHash)),
	}
	copy(rev.KeyHash, leafKeyHash[:])
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)

	err = rev.Verify(caCert)
	assert.NoError(t, err)
}

func TestRevocation_Marshal(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	assert.NoError(t, err)
	leafCert, caKey := chain[peertls.LeafIndex], keys[peertls.CAIndex]

	leafKeyHash, err := peertls.DoubleSHA256PublicKey(leafCert.PublicKey)
	require.NoError(t, err)

	rev := extensions.Revocation{
		Timestamp: time.Now().Unix(),
		KeyHash:   make([]byte, len(leafKeyHash)),
	}
	copy(rev.KeyHash, leafKeyHash[:])
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)

	revBytes, err := rev.Marshal()
	assert.NoError(t, err)
	assert.NotEmpty(t, revBytes)

	decodedRev := new(extensions.Revocation)
	decoder := gob.NewDecoder(bytes.NewBuffer(revBytes))
	err = decoder.Decode(decodedRev)
	assert.NoError(t, err)
	assert.Equal(t, rev, *decodedRev)
}

func TestRevocation_Unmarshal(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	assert.NoError(t, err)
	leafCert, caKey := chain[peertls.LeafIndex], keys[peertls.CAIndex]

	leafKeyHash, err := peertls.DoubleSHA256PublicKey(leafCert.PublicKey)
	require.NoError(t, err)

	rev := extensions.Revocation{
		Timestamp: time.Now().Unix(),
		KeyHash:   make([]byte, len(leafKeyHash)),
	}
	copy(rev.KeyHash, leafKeyHash[:])
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)

	encodedRev := new(bytes.Buffer)
	encoder := gob.NewEncoder(encodedRev)
	err = encoder.Encode(rev)
	assert.NoError(t, err)

	unmarshaledRev := new(extensions.Revocation)
	err = unmarshaledRev.Unmarshal(encodedRev.Bytes())
	assert.NoError(t, err)
	assert.NotNil(t, rev)
	assert.Equal(t, rev, *unmarshaledRev)
}

func TestNewRevocationExt(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	assert.NoError(t, err)

	ext, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
	assert.NoError(t, err)

	var rev extensions.Revocation
	err = rev.Unmarshal(ext.Value)
	assert.NoError(t, err)

	err = rev.Verify(chain[peertls.CAIndex])
	assert.NoError(t, err)
}
