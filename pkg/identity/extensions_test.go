// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
)

type extensionHandlerMock struct {
	mock.Mock
}

func (m *extensionHandlerMock) verify(ext pkix.Extension, chain [][]*x509.Certificate) error {
	args := m.Called(ext, chain)
	return args.Error(0)
}

func TestExtensionHandlers_VerifyFunc(t *testing.T) {
	keys, chain, err := newRevokedLeafChain()
	require.NoError(t, err)
	err = peertls.AddSignedCertExt(keys[0], chain[0])
	require.NoError(t, err)

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

	chains := [][]*x509.Certificate{chain}
	extMock.On("verify", chains[0][peertls.LeafIndex].ExtraExtensions[0], chains).Return(nil)
	extMock.On("verify", chains[0][peertls.LeafIndex].ExtraExtensions[1], chains).Return(nil)

	err = handlers.VerifyFunc()(nil, chains)
	assert.NoError(t, err)
	extMock.AssertCalled(t, "verify", chains[0][peertls.LeafIndex].ExtraExtensions[0], chains)
	extMock.AssertCalled(t, "verify", chains[0][peertls.LeafIndex].ExtraExtensions[1], chains)
	assert.True(t, extMock.AssertExpectations(t))

	// TODO: test error scenario(s)
}

func TestParseExtensions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	revokedLeafKeys, revokedLeafChain, err := newRevokedLeafChain()
	assert.NoError(t, err)

	whitelistSignedKeys, whitelistSignedChain, err := testpeertls.NewCertChain(3)
	assert.NoError(t, err)

	err = peertls.AddSignedCertExt(whitelistSignedKeys[0], whitelistSignedChain[0])
	assert.NoError(t, err)

	_, unrelatedChain, err := testpeertls.NewCertChain(1)
	assert.NoError(t, err)

	revDB, err := NewRevocationDBBolt(ctx.File("revocations.db"))
	assert.NoError(t, err)
	defer ctx.Check(revDB.Close)

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
	revokingKey, err := pkcrypto.GeneratePrivateKey()
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
