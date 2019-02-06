// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/gob"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/peertls"
)

func TestNewCert_CA(t *testing.T) {
	caKey, err := peertls.NewKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	assert.NotEmpty(t, caKey)
	assert.NotEmpty(t, caCert)
	assert.NotEmpty(t, caCert.PublicKey.(*ecdsa.PublicKey))

	err = caCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
}

func TestNewCert_Leaf(t *testing.T) {
	caKey, err := peertls.NewKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, nil, caTemplate, nil)
	assert.NoError(t, err)

	leafKey, err := peertls.NewKey()
	assert.NoError(t, err)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(leafKey, caKey, leafTemplate, caCert)
	assert.NoError(t, err)

	assert.NotEmpty(t, caKey)
	assert.NotEmpty(t, leafCert)
	assert.NotEmpty(t, leafCert.PublicKey.(*ecdsa.PublicKey))

	err = caCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
	err = leafCert.CheckSignatureFrom(caCert)
	assert.NoError(t, err)
}

func TestVerifyPeerFunc(t *testing.T) {
	_, chain, err := testpeertls.NewCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leafCert, caCert := chain[0], chain[1]

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

	err = peertls.VerifyPeerFunc(testFunc)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)
}

func TestVerifyPeerCertChains(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leafKey, leafCert, caCert := keys[1], chain[0], chain[1]

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	wrongKey, err := peertls.NewKey()
	assert.NoError(t, err)

	leafCert, err = peertls.NewCert(leafKey, wrongKey, leafCert, caCert)
	assert.NoError(t, err)

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.True(t, peertls.ErrVerifyPeerCert.Has(err))
	assert.True(t, peertls.ErrVerifyCertificateChain.Has(err))
}

func TestVerifyCAWhitelist(t *testing.T) {
	_, chain2, err := testpeertls.NewCertChain(2)
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

	_, unrelatedChain, err := testpeertls.NewCertChain(1)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	unrelatedCert := unrelatedChain[0]

	t.Run("no valid signed extension, non-empty whitelist", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{unrelatedCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.True(t, peertls.ErrVerifyCAWhitelist.Has(err))
	})

	t.Run("last cert in whitelist is signer", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{unrelatedCert, caCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	t.Run("first cert in whitelist is signer", func(t *testing.T) {
		err = peertls.VerifyPeerFunc(peertls.VerifyCAWhitelist([]*x509.Certificate{caCert, unrelatedCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	_, chain3, err := testpeertls.NewCertChain(3)
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

func TestAddExtension(t *testing.T) {
	_, chain, err := testpeertls.NewCertChain(1)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// NB: there's nothing special about length 32
	randBytes := make([]byte, 32)
	exampleID := asn1.ObjectIdentifier{2, 999}
	i, err := rand.Read(randBytes)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, 32, i)

	ext := pkix.Extension{
		Id:    exampleID,
		Value: randBytes,
	}

	err = peertls.AddExtension(chain[0], ext)
	assert.NoError(t, err)
	assert.Len(t, chain[0].ExtraExtensions, 1)
	assert.Equal(t, ext, chain[0].ExtraExtensions[0])
}

func TestAddSignedCertExt(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(1)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = peertls.AddSignedCertExt(keys[0], chain[0])
	assert.NoError(t, err)

	assert.Len(t, chain[0].ExtraExtensions, 1)
	assert.Equal(t, peertls.ExtensionIDs[peertls.SignedCertExtID], chain[0].ExtraExtensions[0].Id)

	ecKey, ok := keys[0].(*ecdsa.PrivateKey)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	err = peertls.VerifySignature(
		chain[0].ExtraExtensions[0].Value,
		chain[0].RawTBSCertificate,
		&ecKey.PublicKey,
	)
	assert.NoError(t, err)
}

func TestSignLeafExt(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	caKey, leafCert := keys[0], chain[0]

	err = peertls.AddSignedCertExt(caKey, leafCert)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(leafCert.ExtraExtensions))
	assert.True(t, peertls.ExtensionIDs[peertls.SignedCertExtID].Equal(leafCert.ExtraExtensions[0].Id))

	caECKey, ok := caKey.(*ecdsa.PrivateKey)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	err = peertls.VerifySignature(leafCert.ExtraExtensions[0].Value, leafCert.RawTBSCertificate, &caECKey.PublicKey)
	assert.NoError(t, err)
}

func TestRevocation_Sign(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2)
	assert.NoError(t, err)
	leafCert, caKey := chain[0], keys[0]

	leafHash, err := peertls.SHA256Hash(leafCert.Raw)
	assert.NoError(t, err)

	rev := peertls.Revocation{
		Timestamp: time.Now().Unix(),
		CertHash:  make([]byte, len(leafHash)),
	}
	copy(rev.CertHash, leafHash)
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)
}

func TestRevocation_Verify(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2)
	assert.NoError(t, err)
	leafCert, caCert, caKey := chain[0], chain[1], keys[0]

	leafHash, err := peertls.SHA256Hash(leafCert.Raw)
	assert.NoError(t, err)

	rev := peertls.Revocation{
		Timestamp: time.Now().Unix(),
		CertHash:  make([]byte, len(leafHash)),
	}
	copy(rev.CertHash, leafHash)
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)

	err = rev.Verify(caCert)
	assert.NoError(t, err)
}

func TestRevocation_Marshal(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2)
	assert.NoError(t, err)
	leafCert, caKey := chain[0], keys[0]

	leafHash, err := peertls.SHA256Hash(leafCert.Raw)
	assert.NoError(t, err)

	rev := peertls.Revocation{
		Timestamp: time.Now().Unix(),
		CertHash:  make([]byte, len(leafHash)),
	}
	copy(rev.CertHash, leafHash)
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)

	revBytes, err := rev.Marshal()
	assert.NoError(t, err)
	assert.NotEmpty(t, revBytes)

	decodedRev := new(peertls.Revocation)
	decoder := gob.NewDecoder(bytes.NewBuffer(revBytes))
	err = decoder.Decode(decodedRev)
	assert.NoError(t, err)
	assert.Equal(t, rev, *decodedRev)
}

func TestRevocation_Unmarshal(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2)
	assert.NoError(t, err)
	leafCert, caKey := chain[0], keys[0]

	leafHash, err := peertls.SHA256Hash(leafCert.Raw)
	assert.NoError(t, err)

	rev := peertls.Revocation{
		Timestamp: time.Now().Unix(),
		CertHash:  make([]byte, len(leafHash)),
	}
	copy(rev.CertHash, leafHash)
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)

	encodedRev := new(bytes.Buffer)
	encoder := gob.NewEncoder(encodedRev)
	err = encoder.Encode(rev)
	assert.NoError(t, err)

	unmarshaledRev := new(peertls.Revocation)
	err = unmarshaledRev.Unmarshal(encodedRev.Bytes())
	assert.NoError(t, err)
	assert.NotNil(t, rev)
	assert.Equal(t, rev, *unmarshaledRev)
}

func TestNewRevocationExt(t *testing.T) {
	keys, chain, err := testpeertls.NewCertChain(2)
	assert.NoError(t, err)

	ext, err := peertls.NewRevocationExt(keys[0], chain[0])
	assert.NoError(t, err)

	var rev peertls.Revocation
	err = rev.Unmarshal(ext.Value)
	assert.NoError(t, err)

	err = rev.Verify(chain[1])
	assert.NoError(t, err)
}
