// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/gob"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	assert.NotEmpty(t, caKey.(*ecdsa.PrivateKey))
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

	assert.NotEmpty(t, caKey.(*ecdsa.PrivateKey))
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

func TestRevocationDB_Get(t *testing.T) {
	tmp, err := ioutil.TempDir("", "TestRevocationDB_Get")
	defer func() { _ = os.RemoveAll(tmp) }()

	keys, chain, err := testpeertls.NewCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	ext, err := peertls.NewRevocationExt(keys[0], chain[0])
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	revDB, err := peertls.NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	var rev *peertls.Revocation
	t.Run("missing key", func(t *testing.T) {
		rev, err = revDB.Get(chain)
		assert.NoError(t, err)
		assert.Nil(t, rev)
	})

	caHash, err := peertls.SHA256Hash(chain[1].Raw)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = revDB.DB.Put(caHash, ext.Value)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	t.Run("existing key", func(t *testing.T) {
		rev, err = revDB.Get(chain)
		assert.NoError(t, err)

		revBytes, err := rev.Marshal()
		assert.NoError(t, err)
		assert.True(t, bytes.Equal(ext.Value, revBytes))
	})
}

func TestRevocationDB_Put(t *testing.T) {
	tmp, err := ioutil.TempDir("", "TestRevocationDB_Put")
	defer func() { _ = os.RemoveAll(tmp) }()

	keys, chain, err := testpeertls.NewCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	olderExt, err := peertls.NewRevocationExt(keys[0], chain[0])
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)
	ext, err := peertls.NewRevocationExt(keys[0], chain[0])
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	time.Sleep(1 * time.Second)
	newerExt, err := peertls.NewRevocationExt(keys[0], chain[0])
	assert.NoError(t, err)

	revDB, err := peertls.NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	cases := []struct {
		testID   string
		ext      pkix.Extension
		errClass *errs.Class
		err      error
	}{
		{
			"new key",
			ext,
			nil,
			nil,
		},
		{
			"existing key - older timestamp",
			olderExt,
			&peertls.ErrExtension,
			peertls.ErrRevocationTimestamp,
		},
		{
			"existing key - newer timestamp",
			newerExt,
			nil,
			nil,
		},
		// TODO(bryanchriswhite): test empty/garbage cert/timestamp/sig
	}

	for _, c := range cases {
		t.Run(c.testID, func(t2 *testing.T) {
			if !assert.NotNil(t, c.ext) {
				t2.Fail()
				t.FailNow()
			}
			err = revDB.Put(chain, c.ext)
			if c.errClass != nil {
				assert.True(t, c.errClass.Has(err))
			}
			if c.err != nil {
				assert.Equal(t, c.err, err)
			}

			if c.err == nil && c.errClass == nil {
				if !assert.NoError(t2, err) {
					t2.Fail()
					t.FailNow()
				}
				func(t2 *testing.T, ext pkix.Extension) {
					caHash, err := peertls.SHA256Hash(chain[1].Raw)
					if !assert.NoError(t2, err) {
						t2.FailNow()
					}

					revBytes, err := revDB.DB.Get(caHash)
					if !assert.NoError(t2, err) {
						t2.FailNow()
					}

					rev := new(peertls.Revocation)
					err = rev.Unmarshal(revBytes)
					assert.NoError(t2, err)
					assert.True(t2, bytes.Equal(ext.Value, revBytes))
				}(t2, c.ext)
			}
		})
	}
}

type extensionHandlerMock struct {
	mock.Mock
}

func (m *extensionHandlerMock) verify(ext pkix.Extension, chain [][]*x509.Certificate) error {
	args := m.Called(ext, chain)
	return args.Error(0)
}

func TestExtensionHandlers_VerifyFunc(t *testing.T) {
	keys, chain, err := newRevokedLeafChain()
	chains := [][]*x509.Certificate{chain}
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	err = peertls.AddSignedCertExt(keys[0], chain[0])
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	extMock := new(extensionHandlerMock)
	verify := func(ext pkix.Extension, chain [][]*x509.Certificate) error {
		return extMock.verify(ext, chain)
	}

	handlers := peertls.ExtensionHandlers{
		{
			ID:     peertls.ExtensionIDs[peertls.RevocationExtID],
			Verify: verify,
		},
		{
			ID:     peertls.ExtensionIDs[peertls.SignedCertExtID],
			Verify: verify,
		},
	}

	extMock.On("verify", chains[0][peertls.LeafIndex].ExtraExtensions[0], chains).Return(nil)
	extMock.On("verify", chains[0][peertls.LeafIndex].ExtraExtensions[1], chains).Return(nil)

	err = handlers.VerifyFunc()(nil, chains)
	assert.NoError(t, err)
	extMock.AssertCalled(t, "verify", chains[0][peertls.LeafIndex].ExtraExtensions[0], chains)
	extMock.AssertCalled(t, "verify", chains[0][peertls.LeafIndex].ExtraExtensions[1], chains)
	extMock.AssertExpectations(t)

	// TODO: test error scenario(s)
}

func TestParseExtensions(t *testing.T) {
	revokedLeafKeys, revokedLeafChain, err := newRevokedLeafChain()
	assert.NoError(t, err)

	whitelistSignedKeys, whitelistSignedChain, err := testpeertls.NewCertChain(3)
	assert.NoError(t, err)

	err = peertls.AddSignedCertExt(whitelistSignedKeys[0], whitelistSignedChain[0])
	assert.NoError(t, err)

	_, unrelatedChain, err := testpeertls.NewCertChain(1)
	assert.NoError(t, err)

	tmp, err := ioutil.TempDir("", "TestParseExtensions")
	if err != nil {
		t.FailNow()
	}

	defer func() { _ = os.RemoveAll(tmp) }()
	revDB, err := peertls.NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
	assert.NoError(t, err)

	cases := []struct {
		testID    string
		config    peertls.TLSExtConfig
		extLen    int
		certChain []*x509.Certificate
		whitelist []*x509.Certificate
		errClass  *errs.Class
		err       error
	}{
		{
			"leaf whitelist signature - success",
			peertls.TLSExtConfig{WhitelistSignedLeaf: true},
			1,
			whitelistSignedChain,
			[]*x509.Certificate{whitelistSignedChain[2]},
			nil,
			nil,
		},
		{
			"leaf whitelist signature - failure (empty whitelist)",
			peertls.TLSExtConfig{WhitelistSignedLeaf: true},
			1,
			whitelistSignedChain,
			nil,
			&peertls.ErrVerifyCAWhitelist,
			nil,
		},
		{
			"leaf whitelist signature - failure",
			peertls.TLSExtConfig{WhitelistSignedLeaf: true},
			1,
			whitelistSignedChain,
			unrelatedChain,
			&peertls.ErrVerifyCAWhitelist,
			nil,
		},
		{
			"certificate revocation - single revocation ",
			peertls.TLSExtConfig{Revocation: true},
			1,
			revokedLeafChain,
			nil,
			nil,
			nil,
		},
		{
			"certificate revocation - serial revocations",
			peertls.TLSExtConfig{Revocation: true},
			1,
			func() []*x509.Certificate {
				rev := new(peertls.Revocation)
				time.Sleep(1 * time.Second)
				_, chain, err := revokeLeaf(revokedLeafKeys, revokedLeafChain)
				assert.NoError(t, err)

				err = rev.Unmarshal(chain[0].ExtraExtensions[0].Value)
				assert.NoError(t, err)

				return chain
			}(),
			nil,
			nil,
			nil,
		},
		{
			"certificate revocation - serial revocations error (older timestamp)",
			peertls.TLSExtConfig{Revocation: true},
			1,
			func() []*x509.Certificate {
				keys, chain, err := newRevokedLeafChain()
				assert.NoError(t, err)

				rev := new(peertls.Revocation)
				err = rev.Unmarshal(chain[0].ExtraExtensions[0].Value)
				assert.NoError(t, err)

				rev.Timestamp = rev.Timestamp + 300
				err = rev.Sign(keys[0])
				assert.NoError(t, err)

				revBytes, err := rev.Marshal()
				assert.NoError(t, err)

				err = revDB.Put(chain, pkix.Extension{
					Id:    peertls.ExtensionIDs[peertls.RevocationExtID],
					Value: revBytes,
				})
				assert.NoError(t, err)
				return chain
			}(),
			nil,
			&peertls.ErrExtension,
			peertls.ErrRevocationTimestamp,
		},
		{
			"certificate revocation and leaf whitelist signature",
			peertls.TLSExtConfig{Revocation: true, WhitelistSignedLeaf: true},
			2,
			func() []*x509.Certificate {
				_, chain, err := newRevokedLeafChain()
				assert.NoError(t, err)

				err = peertls.AddSignedCertExt(whitelistSignedKeys[0], chain[0])
				assert.NoError(t, err)

				return chain
			}(),
			[]*x509.Certificate{whitelistSignedChain[2]},
			nil,
			nil,
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			opts := peertls.ParseExtOptions{
				CAWhitelist: c.whitelist,
				RevDB:       revDB,
			}

			handlers := peertls.ParseExtensions(c.config, opts)
			assert.Equal(t, c.extLen, len(handlers))
			err := handlers.VerifyFunc()(nil, [][]*x509.Certificate{c.certChain})
			if c.errClass != nil {
				assert.True(t, c.errClass.Has(err))
			}
			if c.err != nil {
				assert.NotNil(t, err)
			}
			if c.errClass == nil && c.err == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func revokeLeaf(keys []crypto.PrivateKey, chain []*x509.Certificate) ([]crypto.PrivateKey, []*x509.Certificate, error) {
	revokingKey, err := peertls.NewKey()
	if err != nil {
		return nil, nil, err
	}

	revokingTemplate, err := peertls.LeafTemplate()
	if err != nil {
		return nil, nil, err
	}

	revokingCert, err := peertls.NewCert(revokingKey, keys[0], revokingTemplate, chain[1])
	if err != nil {
		return nil, nil, err
	}

	err = peertls.AddRevocationExt(keys[0], chain[0], revokingCert)
	if err != nil {
		return nil, nil, err
	}

	return keys, append([]*x509.Certificate{revokingCert}, chain[1:]...), nil
}

func newRevokedLeafChain() ([]crypto.PrivateKey, []*x509.Certificate, error) {
	keys2, certs2, err := testpeertls.NewCertChain(2)
	if err != nil {
		return nil, nil, err
	}

	return revokeLeaf(keys2, certs2)
}
