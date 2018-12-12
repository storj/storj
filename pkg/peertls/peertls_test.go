// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/gob"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	_, chain, err := newCertChain(2)
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

	err = VerifyPeerFunc(testFunc)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)
}

func TestVerifyPeerCertChains(t *testing.T) {
	keys, chain, err := newCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leafKey, leafCert, caCert := keys[1], chain[0], chain[1]

	err = VerifyPeerFunc(VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.NoError(t, err)

	wrongKey, err := NewKey()
	assert.NoError(t, err)

	leafCert, err = NewCert(leafKey, wrongKey, leafCert, caCert)
	assert.NoError(t, err)

	err = VerifyPeerFunc(VerifyPeerCertChains)([][]byte{leafCert.Raw, caCert.Raw}, nil)
	assert.True(t, ErrVerifyPeerCert.Has(err))
	assert.True(t, ErrVerifyCertificateChain.Has(err))
}

func TestVerifyCAWhitelist(t *testing.T) {
	_, chain2, err := newCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leafCert, caCert := chain2[0], chain2[1]

	t.Run("empty whitelist", func(t *testing.T) {
		err = VerifyPeerFunc(VerifyCAWhitelist(nil))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	t.Run("whitelist contains ca", func(t *testing.T) {
		err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{caCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	_, unrelatedChain, err := newCertChain(1)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	unrelatedCert := unrelatedChain[0]

	t.Run("no valid signed extension, non-empty whitelist", func(t *testing.T) {
		err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{unrelatedCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.True(t, ErrVerifyCAWhitelist.Has(err))
	})

	t.Run("last cert in whitelist is signer", func(t *testing.T) {
		err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{unrelatedCert, caCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	t.Run("first cert in whitelist is signer", func(t *testing.T) {
		err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{caCert, unrelatedCert}))([][]byte{leafCert.Raw, caCert.Raw}, nil)
		assert.NoError(t, err)
	})

	_, chain3, err := newCertChain(3)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	leaf2Cert, ca2Cert, rootCert := chain3[0], chain3[1], chain3[2]

	t.Run("length 3 chain - first cert in whitelist is signer", func(t *testing.T) {
		err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{rootCert, unrelatedCert}))([][]byte{leaf2Cert.Raw, ca2Cert.Raw, unrelatedCert.Raw}, nil)
		assert.NoError(t, err)
	})

	t.Run("length 3 chain - last cert in whitelist is signer", func(t *testing.T) {
		err = VerifyPeerFunc(VerifyCAWhitelist([]*x509.Certificate{unrelatedCert, rootCert}))([][]byte{leaf2Cert.Raw, ca2Cert.Raw, unrelatedCert.Raw}, nil)
		assert.NoError(t, err)
	})
}

func TestAddExtension(t *testing.T) {
	t.SkipNow()
}

func TestAddSignedCertExt(t *testing.T) {
	t.SkipNow()
}

func TestSignLeafExt(t *testing.T) {
	keys, chain, err := newCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	caKey, leafCert := keys[0], chain[0]

	err = AddSignedCertExt(caKey, leafCert)
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

func TestRevocation_Sign(t *testing.T) {
	keys, chain, err := newCertChain(2)
	assert.NoError(t, err)
	leafCert, caKey := chain[0], keys[0]

	leafHash, err := hashBytes(leafCert.Raw)
	assert.NoError(t, err)

	rev := Revocation{
		Timestamp: time.Now().Unix(),
		CertHash:  make([]byte, len(leafHash)),
	}
	copy(rev.CertHash, leafHash)
	err = rev.Sign(caKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, rev.Signature)
}

func TestRevocation_Verify(t *testing.T) {
	keys, chain, err := newCertChain(2)
	assert.NoError(t, err)
	leafCert, caCert, caKey := chain[0], chain[1], keys[0]

	leafHash, err := hashBytes(leafCert.Raw)
	assert.NoError(t, err)

	rev := Revocation{
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
	keys, chain, err := newCertChain(2)
	assert.NoError(t, err)
	leafCert, caKey := chain[0], keys[0]

	leafHash, err := hashBytes(leafCert.Raw)
	assert.NoError(t, err)

	rev := Revocation{
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

	decodedRev := new(Revocation)
	decoder := gob.NewDecoder(bytes.NewBuffer(revBytes))
	err = decoder.Decode(decodedRev)
	assert.NoError(t, err)
	assert.Equal(t, rev, *decodedRev)
}

func TestRevocation_Unmarshal(t *testing.T) {
	keys, chain, err := newCertChain(2)
	assert.NoError(t, err)
	leafCert, caKey := chain[0], keys[0]

	leafHash, err := hashBytes(leafCert.Raw)
	assert.NoError(t, err)

	rev := Revocation{
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

	unmarshaledRev := new(Revocation)
	err = unmarshaledRev.Unmarshal(encodedRev.Bytes())
	assert.NoError(t, err)
	assert.NotNil(t, rev)
	assert.Equal(t, rev, *unmarshaledRev)
}

func TestNewRevocationExt(t *testing.T) {
	keys, chain, err := newCertChain(2)
	assert.NoError(t, err)

	ext, err := NewRevocationExt(keys[0], chain[0])
	assert.NoError(t, err)

	var rev Revocation
	err = rev.Unmarshal(ext.Value)
	assert.NoError(t, err)

	err = rev.Verify(chain[1])
	assert.NoError(t, err)
}

func TestRevocationDB_Get(t *testing.T) {
	tmp, err := ioutil.TempDir("", os.TempDir())
	defer func() { _ = os.RemoveAll(tmp) }()

	keys, chain, err := newCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	ext, err := NewRevocationExt(keys[0], chain[0])
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	revDB, err := NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	var rev *Revocation
	t.Run("missing key", func(t *testing.T) {
		rev, err = revDB.Get(chain)
		assert.NoError(t, err)
		assert.Nil(t, rev)
	})

	caHash, err := hashBytes(chain[1].Raw)
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
	tmp, err := ioutil.TempDir("", os.TempDir())
	defer func() { _ = os.RemoveAll(tmp) }()

	keys, chain, err := newCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	olderExt, err := NewRevocationExt(keys[0], chain[0])
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)
	ext, err := NewRevocationExt(keys[0], chain[0])
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	time.Sleep(1 * time.Second)
	newerExt, err := NewRevocationExt(keys[0], chain[0])
	assert.NoError(t, err)

	revDB, err := NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
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
			&ErrExtension,
			ErrRevocationTimestamp,
		},
		{
			"existing key - newer timestamp",
			newerExt,
			nil,
			nil,
		},
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
					caHash, err := hashBytes(chain[1].Raw)
					if !assert.NoError(t2, err) {
						t2.FailNow()
					}

					revBytes, err := revDB.DB.Get(caHash)
					if !assert.NoError(t2, err) {
						t2.FailNow()
					}

					rev := new(Revocation)
					err = rev.Unmarshal(revBytes)
					assert.NoError(t2, err)
					assert.True(t2, bytes.Equal(ext.Value, revBytes))
				}(t2, c.ext)
			}
		})
	}
}

func TestParseExtensions(t *testing.T) {
	revokedLeafKeys, revokedLeafChain, err := newRevokedLeafChain()
	assert.NoError(t, err)
	_ = revokedLeafKeys

	whitelistSignedKeys, whitelistSignedChain, err := newCertChain(3)
	assert.NoError(t, err)

	err = AddSignedCertExt(whitelistSignedKeys[0], whitelistSignedChain[0])
	assert.NoError(t, err)

	_, unrelatedChain, err := newCertChain(1)
	assert.NoError(t, err)

	tmp, err := ioutil.TempDir("", os.TempDir())
	if err != nil {
		t.FailNow()
	}

	defer func() { _ = os.RemoveAll(tmp) }()
	revDB, err := NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
	assert.NoError(t, err)

	cases := []struct {
		testID    string
		config    TLSExtConfig
		certChain []*x509.Certificate
		whitelist []*x509.Certificate
		errClass  *errs.Class
		err       error
	}{
		{
			"leaf whitelist signature - success",
			TLSExtConfig{WhitelistSignedLeaf: true},
			whitelistSignedChain,
			[]*x509.Certificate{whitelistSignedChain[2]},
			nil,
			nil,
		},
		{
			"leaf whitelist signature - failure (empty whitelist)",
			TLSExtConfig{WhitelistSignedLeaf: true},
			whitelistSignedChain,
			nil,
			&ErrVerifyCAWhitelist,
			nil,
		},
		{
			"leaf whitelist signature - failure",
			TLSExtConfig{WhitelistSignedLeaf: true},
			whitelistSignedChain,
			unrelatedChain,
			&ErrVerifyCAWhitelist,
			nil,
		},
		{
			"certificate revocation - single revocation ",
			TLSExtConfig{Revocation: true},
			revokedLeafChain,
			nil,
			nil,
			nil,
		},
		{
			"certificate revocation - multiple, serial revocations",
			TLSExtConfig{Revocation: true},
			func() []*x509.Certificate {
				rev := new(Revocation)
				err = rev.Unmarshal(revokedLeafChain[0].ExtraExtensions[0].Value)
				assert.NoError(t, err)

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
		// TODO: test multiple simultaneous extensions
		// TODO:
		// 	{
		// 		"certificate revocation - error (older timestamp)",
		// 		TLSExtConfig{Revocation: true},
		// 		func() []*x509.Certificate {
		// 			rev := new(Revocation)
		// 			err = rev.Unmarshal(revokedLeafChain[0].ExtraExtensions[0].Value)
		// 			assert.NoError(t, err)
		//
		// 			time.Sleep(1 * time.Second)
		// 			_, chain, err := revokeLeaf(revokedLeafKeys, revokedLeafChain)
		// 			assert.NoError(t, err)
		//
		// 			err = rev.Unmarshal(chain[0].ExtraExtensions[0].Value)
		// 			assert.NoError(t, err)
		//
		// 			err = revDB.Put(revokedLeafChain, chain[0].ExtraExtensions[0])
		// 			return chain
		// 		}(),
		// 		nil,
		// 		&ErrExtension,
		// 		ErrRevocationTimestamp,
		// 	},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			opts := ParseExtOptions{
				CAWhitelist: c.whitelist,
				RevDB:       revDB,
			}

			exts := ParseExtensions(c.config, opts)
			assert.Equal(t, 1, len(exts))
			for _, e := range exts {
				err := e.verify(c.certChain[0].ExtraExtensions[0], [][]*x509.Certificate{c.certChain})
				if c.errClass != nil {
					assert.True(t, c.errClass.Has(err))
				}
				if c.err != nil {
					assert.NotNil(t, err)
				}
				if c.errClass == nil && c.err == nil {
					assert.NoError(t, err)
				}
			}
		})
	}
}

// NB: keys are in the reverse order compared to certs (i.e. first key belongs to last cert)!
func newCertChain(length int) (keys []crypto.PrivateKey, certs []*x509.Certificate, _ error) {
	for i := 0; i < length; i++ {
		key, err := NewKey()
		if err != nil {
			return nil, nil, err
		}
		keys = append(keys, key)

		var template *x509.Certificate
		if i == length-1 {
			template, err = CATemplate()
		} else {
			template, err = LeafTemplate()
		}
		if err != nil {
			return nil, nil, err
		}

		var cert *x509.Certificate
		if i == 0 {
			cert, err = NewCert(key, nil, template, nil)
		} else {
			cert, err = NewCert(key, keys[i-1], template, certs[i-1:][0])
		}
		if err != nil {
			return nil, nil, err
		}

		certs = append([]*x509.Certificate{cert}, certs...)
	}
	return keys, certs, nil
}

func revokeLeaf(keys []crypto.PrivateKey, chain []*x509.Certificate) ([]crypto.PrivateKey, []*x509.Certificate, error) {
	revokingKey, err := NewKey()
	if err != nil {
		return nil, nil, err
	}

	revokingTemplate, err := LeafTemplate()
	if err != nil {
		return nil, nil, err
	}

	revokingCert, err := NewCert(revokingKey, keys[0], revokingTemplate, chain[1])
	if err != nil {
		return nil, nil, err
	}

	err = AddRevocationExt(keys[0], chain[0], revokingCert)
	if err != nil {
		return nil, nil, err
	}

	return keys, append([]*x509.Certificate{revokingCert}, chain[1:]...), nil
}

func newRevokedLeafChain() ([]crypto.PrivateKey, []*x509.Certificate, error) {
	keys2, certs2, err := newCertChain(2)
	if err != nil {
		return nil, nil, err
	}

	return revokeLeaf(keys2, certs2)
}
