// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"crypto/x509"
	"testing"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/peertls"
	"bytes"
	"encoding/pem"
	"crypto/ecdsa"
)

func TestPeerIdentityFromCertChain(t *testing.T) {
	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	ca, err := x509.ParseCertificate(cert.Certificate[len(cert.Certificate) - 1])
	assert.NoError(t, err)

	pi, err := PeerIdentityFromCertChain(cert.Certificate)
	assert.NoError(t, err)
	assert.Equal(t, ca, pi.CA)
	assert.Equal(t, cert.Leaf, pi.Leaf)
	assert.NotEmpty(t, pi.ID)
}

func TestFullIdentityFromPEM(t *testing.T) {
	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	chainPEM := bytes.NewBuffer([]byte{})
	for _, c := range cert.Certificate {
		pem.Encode(chainPEM, peertls.NewCertBlock(c))
	}

	privateKey, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, privateKey)

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, keyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	pem.Encode(keyPEM, peertls.NewKeyBlock(keyBytes))

	fi, err := FullIdentityFromPEM(chainPEM.Bytes(), keyPEM.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, cert.PrivateKey, fi.PrivateKey)
}

func TestIdentityConfig_SaveIdentity(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestIdentityConfig_SaveIdentity")
	assert.NoError(t, err)
	if err != nil {
		t.Fatal(err)
	}

	ic := IdentityConfig{
		CertPath: filepath.Join(tmpDir, "cert.pem"),
		KeyPath:  filepath.Join(tmpDir, "key.pem"),
	}

	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	chainPEM := bytes.NewBuffer([]byte{})
	for _, c := range cert.Certificate {
		pem.Encode(chainPEM, peertls.NewCertBlock(c))
	}

	privateKey, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, privateKey)

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, keyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	pem.Encode(keyPEM, peertls.NewKeyBlock(keyBytes))

	fi, err := FullIdentityFromPEM(chainPEM.Bytes(), keyPEM.Bytes())
	assert.NoError(t, err)

	err = ic.SaveIdentity(fi)
	assert.NoError(t, err)

	for _, path := range []string{ic.CertPath, ic.KeyPath} {
		fileInfo, err := os.Stat(path)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), fileInfo.Mode())
	}

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

func tempIdentity(t *testing.T) (func(), *IdentityConfig, string, string, uint16, error) {
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

	err = ioutil.WriteFile(ic.CertPath, []byte(chain), 0600)
	if err != nil {
		cleanup()
		return nil, nil,  "", "", 0, err
	}

	err = ioutil.WriteFile(ic.KeyPath, []byte(key), 0600)
	assert.NoError(t, err)
	if !assert.NoError(t, err) {
		cleanup()
		t.Fatal(err)
	}

	return cleanup, ic, chain, key, difficulty, nil
}

func TestIdentityConfig_LoadIdentity(t *testing.T) {
	done, ic, chainPEM, keyPEM, _, err := tempIdentity(t)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	defer done()

	fi, err := ic.LoadIdentity()
	assert.NoError(t, err)
	assert.NotEmpty(t, fi)
	assert.NotEmpty(t, fi.PrivateKey)
	assert.NotEmpty(t, fi.PeerIdentity.Leaf)
	assert.NotEmpty(t, fi.PeerIdentity.CA)
	assert.NotEmpty(t, fi.PeerIdentity.ID)

	expectedFI, err := FullIdentityFromPEM([]byte(chainPEM), []byte(keyPEM))
	assert.NoError(t, err)
	assert.NotEmpty(t, expectedFI)

	assert.Equal(t, expectedFI.PrivateKey, fi.PrivateKey)
	assert.Equal(t, expectedFI.PeerIdentity.Leaf, fi.PeerIdentity.Leaf)
	assert.Equal(t, expectedFI.PeerIdentity.CA, fi.PeerIdentity.CA)
	assert.Equal(t, expectedFI.PeerIdentity.ID, fi.PeerIdentity.ID)
}

func TestFullIdentity_Difficulty(t *testing.T) {
	done, _, chainPEM, keyPEM, knownDifficulty, err := tempIdentity(t)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	defer done()

	fi, err := FullIdentityFromPEM([]byte(chainPEM), []byte(keyPEM))
	assert.NoError(t, err)

	difficulty := fi.Difficulty()
	assert.True(t, difficulty >= knownDifficulty)
}

func TestNewID(t *testing.T) {
	ic, done, err := tempIdentityConfig()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	defer done()

	expectedDifficulty := uint16(12)

	fi := ic.Generate(expectedDifficulty, 5)
	assert.NotEmpty(t, fi)

	actualDifficulty := fi.Difficulty()
	assert.True(t, actualDifficulty >= expectedDifficulty)
}

func BenchmarkIdentityConfig_Generate_Difficulty8_Concurrency1(b *testing.B) {
	ic, done, err := tempIdentityConfig()
	if !assert.NoError(b, err) {
		b.Fatal(err)
	}
	defer done()

	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		ic.Generate(expectedDifficulty, 1)
	}
}

func BenchmarkIdentityConfig_Generate_Difficulty8_Concurrency2(b *testing.B) {
	ic, done, err := tempIdentityConfig()
	if !assert.NoError(b, err) {
		b.Fatal(err)
	}
	defer done()

	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		ic.Generate(expectedDifficulty, 2)
	}
}

func BenchmarkIdentityConfig_Generate_Difficulty8_Concurrency5(b *testing.B) {
	ic, done, err := tempIdentityConfig()
	if !assert.NoError(b, err) {
		b.Fatal(err)
	}
	defer done()

	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		ic.Generate(expectedDifficulty, 5)
	}
}

func BenchmarkIdentityConfig_Generate_Difficulty8_Concurrency10(b *testing.B) {
	ic, done, err := tempIdentityConfig()
	if !assert.NoError(b, err) {
		b.Fatal(err)
	}
	defer done()

	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		ic.Generate(expectedDifficulty, 10)
	}
}
