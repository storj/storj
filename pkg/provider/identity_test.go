// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
)

func TestPeerIdentityFromCertChain(t *testing.T) {
	caKey, err := peertls.NewKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, caKey, caTemplate, nil)
	assert.NoError(t, err)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := peertls.NewKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(leafKey, caKey, leafTemplate, caTemplate)
	assert.NoError(t, err)

	peerIdent, err := provider.PeerIdentityFromCerts(leafCert, caCert, nil)
	assert.NoError(t, err)
	assert.Equal(t, caCert, peerIdent.CA)
	assert.Equal(t, leafCert, peerIdent.Leaf)
	assert.NotEmpty(t, peerIdent.ID)
}

func TestFullIdentityFromPEM(t *testing.T) {
	caKey, err := peertls.NewKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, caKey, caTemplate, nil)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.NotEmpty(t, caCert)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := peertls.NewKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(leafKey, caKey, leafTemplate, caTemplate)
	assert.NoError(t, err)
	assert.NotEmpty(t, leafCert)

	chainPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(leafCert.Raw)))
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(caCert.Raw)))

	leafECKey, ok := leafKey.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, leafECKey)

	leafKeyBytes, err := x509.MarshalECPrivateKey(leafECKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, leafKeyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(keyPEM, peertls.NewKeyBlock(leafKeyBytes)))

	fullIdent, err := provider.FullIdentityFromPEM(chainPEM.Bytes(), keyPEM.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, leafCert.Raw, fullIdent.Leaf.Raw)
	assert.Equal(t, caCert.Raw, fullIdent.CA.Raw)
	assert.Equal(t, leafKey, fullIdent.Key)
}

func TestIdentityConfig_SaveIdentity(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ic := &provider.IdentityConfig{
		CertPath: ctx.File("chain.pem"),
		KeyPath:  ctx.File("key.pem"),
	}
	fi := pregeneratedIdentity(t)

	chainPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(fi.Leaf.Raw)))
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(fi.CA.Raw)))

	privateKey, ok := fi.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, privateKey)

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, keyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(keyPEM, peertls.NewKeyBlock(keyBytes)))

	{ // test saving
		err = ic.Save(fi)
		assert.NoError(t, err)

		certInfo, err := os.Stat(ic.CertPath)
		assert.NoError(t, err)

		keyInfo, err := os.Stat(ic.KeyPath)
		assert.NoError(t, err)

		// TODO (windows): ignoring for windows due to different default permissions
		if runtime.GOOS != "windows" {
			assert.Equal(t, os.FileMode(0644), certInfo.Mode())
			assert.Equal(t, os.FileMode(0600), keyInfo.Mode())
		}
	}

	{ // test loading
		loadedFi, err := ic.Load()
		assert.NoError(t, err)
		assert.Equal(t, fi.Key, loadedFi.Key)
		assert.Equal(t, fi.Leaf, loadedFi.Leaf)
		assert.Equal(t, fi.CA, loadedFi.CA)
		assert.Equal(t, fi.ID, loadedFi.ID)
	}
}

func TestVerifyPeer(t *testing.T) {
	ca, err := provider.NewCA(context.Background(), provider.NewCAOptions{
		Difficulty:  12,
		Concurrency: 4,
	})
	assert.NoError(t, err)

	fi, err := ca.NewIdentity()
	assert.NoError(t, err)

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{fi.Leaf.Raw, fi.CA.Raw}, nil)
	assert.NoError(t, err)
}

func TestNewServerOptions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fi := pregeneratedIdentity(t)

	whitelistPath := ctx.File("whitelist.pem")

	chainData, err := peertls.ChainBytes(fi.CA)
	assert.NoError(t, err)

	err = ioutil.WriteFile(whitelistPath, chainData, 0644)
	assert.NoError(t, err)

	cases := []struct {
		testID      string
		config      provider.ServerConfig
		pcvFuncsLen int
	}{
		{
			"default",
			provider.ServerConfig{},
			0,
		},
		{
			"revocation processing",
			provider.ServerConfig{
				RevocationDBURL: "bolt://" + ctx.File("revocation1.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation: true,
				},
			},
			2,
		},
		{
			"ca whitelist verification",
			provider.ServerConfig{
				PeerCAWhitelistPath: whitelistPath,
			},
			1,
		},
		{
			"ca whitelist verification and whitelist signed leaf verification",
			provider.ServerConfig{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				Extensions: peertls.TLSExtConfig{
					WhitelistSignedLeaf: true,
				},
			},
			2,
		},
		{
			"revocation processing and whitelist verification",
			provider.ServerConfig{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				RevocationDBURL:     "bolt://" + ctx.File("revocation2.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation: true,
				},
			},
			3,
		},
		{
			"revocation processing, whitelist, and signed leaf verification",
			provider.ServerConfig{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				RevocationDBURL:     "bolt://" + ctx.File("revocation3.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation:          true,
					WhitelistSignedLeaf: true,
				},
			},
			3,
		},
	}

	for _, c := range cases {
		t.Log(c.testID)
		opts, err := provider.NewServerOptions(fi, c.config)
		assert.NoError(t, err)
		assert.True(t, reflect.DeepEqual(fi, opts.Ident))
		assert.Equal(t, c.config, opts.Config)
		assert.Len(t, opts.PCVFuncs, c.pcvFuncsLen)
	}
}

func pregeneratedIdentity(t *testing.T) *provider.FullIdentity {
	const chain = `-----BEGIN CERTIFICATE-----
MIIBQDCB56ADAgECAhB+u3d03qyW/ROgwy/ZsPccMAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABIZrEPV/ExEkF0qUF0fJ3qSeGt5oFUX231v02NSUywcQ/Ve0
v3nHbmcJdjWBis2AkfL25mYDVC25jLl4tylMKumjPzA9MA4GA1UdDwEB/wQEAwIF
oDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAK
BggqhkjOPQQDAgNIADBFAiEA2ZvsR0ncw4mHRIg2Isavd+XVEoMo/etXQRAkDy9n
wyoCIDykUsqjshc9kCrXOvPSN8GuO2bNoLu5C7K1GlE/HI2X
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBODCB4KADAgECAhAOcvhKe5TWT44LqFfgA1f8MAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABIZrEPV/ExEkF0qUF0fJ3qSeGt5oFUX231v02NSUywcQ/Ve0
v3nHbmcJdjWBis2AkfL25mYDVC25jLl4tylMKumjODA2MA4GA1UdDwEB/wQEAwIC
BDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49
BAMCA0cAMEQCIGAZfPT1qvlnkTacojTtP20ZWf6XbnSztJHIKlUw6AE+AiB5Vcjj
awRaC5l1KBPGqiKB0coVXDwhW+K70l326MPUcg==
-----END CERTIFICATE-----`

	const key = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIKGjEetrxKrzl+AL1E5LXke+1ElyAdjAmr88/1Kx09+doAoGCCqGSM49
AwEHoUQDQgAEoLy/0hs5deTXZunRumsMkiHpF0g8wAc58aXANmr7Mxx9tzoIYFnx
0YN4VDKdCtUJa29yA6TIz1MiIDUAcB5YCA==
-----END EC PRIVATE KEY-----`

	fi, err := provider.FullIdentityFromPEM([]byte(chain), []byte(key))
	assert.NoError(t, err)

	return fi
}
