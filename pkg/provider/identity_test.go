// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/peertls"
)

func TestPeerIdentityFromCertChain(t *testing.T) {
	caT, err := peertls.CATemplate()
	assert.NoError(t, err)

	caC, err := peertls.Generate(caT, nil, nil, nil)
	assert.NoError(t, err)

	lT, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	lC, err := peertls.Generate(lT, caT, caC, caC)
	assert.NoError(t, err)

	pi, err := PeerIdentityFromCerts(lC.Leaf, caC.Leaf)
	assert.NoError(t, err)
	assert.Equal(t, caC.Leaf, pi.CA.Cert)
	assert.Equal(t, lC.Leaf, pi.Leaf)
	assert.NotEmpty(t, pi.ID)
}

func TestFullIdentityFromPEM(t *testing.T) {
	caT, err := peertls.CATemplate()
	assert.NoError(t, err)

	caC, err := peertls.Generate(caT, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.NotEmpty(t, caC)

	lT, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	lC, err := peertls.Generate(lT, caT, caC, caC)
	assert.NoError(t, err)
	assert.NotEmpty(t, lC)
	assert.NotEmpty(t, lC.PrivateKey)

	chainPEM := bytes.NewBuffer([]byte{})
	for _, c := range lC.Certificate {
		pem.Encode(chainPEM, peertls.NewCertBlock(c))
	}

	privateKey, ok := lC.PrivateKey.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, privateKey)

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, keyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	pem.Encode(keyPEM, peertls.NewKeyBlock(keyBytes))

	fi, err := FullIdentityFromPEM(chainPEM.Bytes(), keyPEM.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, lC.Certificate[0], fi.Leaf.Raw)
	assert.Equal(t, lC.Certificate[1], fi.CA.Cert.Raw)
	assert.Equal(t, lC.PrivateKey, fi.PrivateKey)
}

func TestIdentityConfig_SaveIdentity(t *testing.T) {
	done, ic, fi, _ := tempIdentity(t)
	defer done()

	chainPEM := bytes.NewBuffer([]byte{})
	pem.Encode(chainPEM, peertls.NewCertBlock(fi.Leaf.Raw))
	pem.Encode(chainPEM, peertls.NewCertBlock(fi.CA.Cert.Raw))

	privateKey, ok := fi.PrivateKey.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, privateKey)

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, keyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	pem.Encode(keyPEM, peertls.NewKeyBlock(keyBytes))

	err = ic.Save(fi)
	assert.NoError(t, err)

	certInfo, err := os.Stat(ic.CertPath)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), certInfo.Mode())

	keyInfo, err := os.Stat(ic.KeyPath)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), keyInfo.Mode())

	savedChainPEM, err := ioutil.ReadFile(ic.CertPath)
	assert.NoError(t, err)

	savedKeyPEM, err := ioutil.ReadFile(ic.KeyPath)
	assert.NoError(t, err)

	assert.Equal(t, chainPEM.Bytes(), savedChainPEM)
	assert.Equal(t, keyPEM.Bytes(), savedKeyPEM)
}

func tempIdentityConfig() (*IdentityConfig, func(), error) {
	tmpDir, err := ioutil.TempDir("", "tempIdentity")
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	return &IdentityConfig{
		CertPath: filepath.Join(tmpDir, "chain.pem"),
		KeyPath:  filepath.Join(tmpDir, "key.pem"),
	}, cleanup, nil
}

func tempIdentity(t *testing.T) (func(), *IdentityConfig, *FullIdentity, uint16) {
	// NB: known difficulty
	difficulty := uint16(12)

	chain := `-----BEGIN CERTIFICATE-----
MIIBQTCB6KADAgECAhEA7iLmNy8uop2bC4Yv1uXvwjAKBggqhkjOPQQDAjAAMCIY
DzAwMDEwMTAxMDAwMDAwWhgPMDAwMTAxMDEwMDAwMDBaMAAwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAATD84AzWKMs7rSuQ0pGbtQE5X6EvKe74ORUgayxLimvs0dX
1KOLg5XmbUF4bwHPvkbDLUlSCWx5qgFmL+XhuR5doz8wPTAOBgNVHQ8BAf8EBAMC
BaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAw
CgYIKoZIzj0EAwIDSAAwRQIgQkJgjRar0nIOQbEAin5bQe4+9BUjSIQzrlkJgXsC
liICIQDz6LeN9nRKCuRcqiK8tnaKbOJ+/Q3PQNHuK7coFFuB1g==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBOjCB4aADAgECAhEA4A+Fdf1cyylCp0GCWMtpJDAKBggqhkjOPQQDAjAAMCIY
DzAwMDEwMTAxMDAwMDAwWhgPMDAwMTAxMDEwMDAwMDBaMAAwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAAQz10hua+xRFmIRKJLMZh9os3PM3mWtElD3WyoR2U6m6U1B
zRJ7cXS0CaPsbilglXjnWHOSV6QKmgcHYTroWkgvozgwNjAOBgNVHQ8BAf8EBAMC
AgQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjO
PQQDAgNIADBFAiEAnvRK+MtT7hWt9CeQvKID40CcPJDhYIEQjN91W1sseNICICgL
y9HDctQtMjRMG3UHifkDl7kPINkiP7w068I5RWvx
-----END CERTIFICATE-----`

	key := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEILQar8Z01NkX/czx8yGevdBATINSW1+U6AQS0Sl5WbdVoAoGCCqGSM49
AwEHoUQDQgAEw/OAM1ijLO60rkNKRm7UBOV+hLynu+DkVIGssS4pr7NHV9Sji4OV
5m1BeG8Bz75Gwy1JUglseaoBZi/l4bkeXQ==
-----END EC PRIVATE KEY-----`

	ic, cleanup, err := tempIdentityConfig()

	fi, err := FullIdentityFromPEM([]byte(chain), []byte(key))
	assert.NoError(t, err)

	return cleanup, ic, fi, difficulty
}

func TestIdentityConfig_LoadIdentity(t *testing.T) {
	done, ic, expectedFI, _ := tempIdentity(t)
	defer done()

	err := ic.Save(expectedFI)
	assert.NoError(t, err)

	fi, err := ic.Load()
	assert.NoError(t, err)
	assert.NotEmpty(t, fi)
	assert.NotEmpty(t, fi.PrivateKey)
	assert.NotEmpty(t, fi.Leaf)
	assert.NotEmpty(t, fi.CA)
	assert.NotEmpty(t, fi.ID().Bytes())

	assert.Equal(t, expectedFI.PrivateKey, fi.PrivateKey)
	assert.Equal(t, expectedFI.Leaf, fi.Leaf)
	assert.Equal(t, expectedFI.CA, fi.CA)
	assert.Equal(t, expectedFI.ID().Bytes(), fi.ID().Bytes())
}

func TestFullIdentity_Difficulty(t *testing.T) {
	done, _, fi, knownDifficulty := tempIdentity(t)
	defer done()

	difficulty := fi.Difficulty()
	assert.True(t, difficulty >= knownDifficulty)
}
