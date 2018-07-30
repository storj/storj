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

	pi, err := PeerIdentityFromCertChain(&cert)
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
	// defer os.RemoveAll(tmpDir)

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
	difficulty := uint16(15)

	chain := `-----BEGIN CERTIFICATE-----
MIIBQDCB56ADAgECAhBvqEtJvK4142wkszbEn83aMAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABLfxhMJRvl55lf0DgBV/Hb9njJ8LFtC3u2dO+wx82U1aircD
yF0G4ij8z//iPi9mYmzzCeDaW6Vw6QqpKBK74tmjPzA9MA4GA1UdDwEB/wQEAwIF
oDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAK
BggqhkjOPQQDAgNIADBFAiAay1qSfFz/C0Xjx36aq3mywm2x8p1VFDv770lnHrIl
5wIhAIqBxbYBGB0GnNLJruTVce3Mph8Otpj/F8J0kGv7Lqyo
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBOjCB4aADAgECAhEA6DWDDVdX0iwLZamPjH0BhTAKBggqhkjOPQQDAjAAMCIY
DzAwMDEwMTAxMDAwMDAwWhgPMDAwMTAxMDEwMDAwMDBaMAAwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAATS2o30ZpQgrBeiQLmTW9bVLjM1erHXnBd7Iosg+0qbDJQX
1HtExf0jCjMVB+szUmh/k0bS5MgRLIJchrWQrHjHozgwNjAOBgNVHQ8BAf8EBAMC
AgQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjO
PQQDAgNIADBFAiEAgU0NrvC54iSDLDVdenFpjRLrF5OjGkArierZrPSZF3ACIDJY
g87i3C2ojCbWEWDqOIhJEs6lT5xRnVlZhcyUEJg2
-----END CERTIFICATE-----`

	key := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIPKyB2RCdmxW24CCnLqPIV7/iKVTMIxxRkq4WM+wJY42oAoGCCqGSM49
AwEHoUQDQgAEt/GEwlG+XnmV/QOAFX8dv2eMnwsW0Le7Z077DHzZTVqKtwPIXQbi
KPzP/+I+L2ZibPMJ4NpbpXDpCqkoErvi2Q==
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
