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

// func TestParseID(t *testing.T) {
// 	// hashLen := uint16(256)
// 	tlsH, err := peertls.NewTLSHelper(nil)
// 	assert.NoError(t, err)
//
// 	cert := tlsH.Certificate()
// 	fullID, err := CertToCreds(&cert)
// 	assert.NoError(t, err)
//
// 	peerID, err := provider.ParsePeerIdentity(fullID.String())
// 	assert.NoError(t, err)
// 	assert.Equal(t, peerID.hash, fullID.hash)
//
// 	pubKey := fullID.tlsH.PubKey()
// 	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&pubKey)
// 	assert.NoError(t, err)
// 	assert.Equal(t, peerID.pubKey, pubKeyBytes)
// }
//
// func TestKadCreds_Bytes(t *testing.T) {
// 	hashLen := uint16(36)
// 	tlsH, err := peertls.NewTLSHelper(nil)
// 	assert.NoError(t, err)
//
// 	cert := tlsH.Certificate()
// 	kadCreds, err := CertToCreds(&cert, hashLen)
// 	assert.NoError(t, err)
//
// 	kadCredBytes := kadCreds.Bytes()
// 	assert.NotNil(t, kadCredBytes)
// }
//

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
	defer os.RemoveAll(tmpDir)

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
//
// idPath := filepath.Join(tmpDir, "id.pem")
// err = fi.Save(idPath)
// assert.NoError(t, err)
//
// idBytes, err := ioutil.ReadFile(idPath)
// assert.NoError(t, err)
// assert.NotNil(t, idBytes)
//
// kadCredBytes := bytes.NewBuffer([]byte{})
// err = fi.write(kadCredBytes)
// assert.NoError(t, err)
//
// bytesEqual := bytes.Compare(idBytes, kadCredBytes.Bytes()) == 0
// assert.True(t, bytesEqual)
//
// fileInfo, err := os.Stat(idPath)
// assert.NoError(t, err)
// assert.Equal(t, os.FileMode(0600), fileInfo.Mode())

// func TestIdentityConfig_LoadIdentity(t *testing.T) {
// 	tmpDir, err := ioutil.TempDir("", "TestLoadID")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.RemoveAll(tmpDir)
//
// 	ic := IdentityConfig{
// 		CertPath: filepath.Join(tmpDir, "cert.pem"),
// 		KeyPath: filepath.Join(tmpDir, "key.pem"),
// 	}
//
//
//
// 	fi, err := ic.LoadIdentity()
// 	assert.NoError(t, err)
// 	assert.NotEmpty(t, fi)
//
// 	tlsH, err := peertls.NewTLSHelper(nil)
// 	assert.NoError(t, err)
//
// 	cert := tlsH.Certificate()
// 	assert.NoError(t, err)
//
// 	// savedKadCreds, err := CertToCreds(&cert, hashLen)
// 	// assert.NoError(t, err)
// 	//
// 	// idPath := filepath.Join(tmpDir, "id.pem")
// 	// err = savedKadCreds.Save(idPath)
// 	// assert.NoError(t, err)
// 	//
// 	// loadedKadCreds, err := LoadID(idPath, hashLen)
// 	// assert.NoError(t, err)
// 	// assert.NotNil(t, loadedKadCreds)
// 	//
// 	// assert.Equal(t, savedKadCreds.hashLen, loadedKadCreds.hashLen)
// 	// assert.Equal(t, savedKadCreds.hash, loadedKadCreds.hash)
// 	// assert.Equal(t, savedKadCreds.tlsH.Certificate(), loadedKadCreds.tlsH.Certificate())
// }



//
// func TestKadCreds_Difficulty_FAST(t *testing.T) {
// 	// NB: (hash length: 38 | difficulty: 3)
// 	var idPEM = `-----BEGIN CERTIFICATE-----
// MIIBQTCB56ADAgECAhA9qbPSqt8UJt6jGuT8FEC0MAoGCCqGSM49BAMCMAAwIhgP
// MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
// CCqGSM49AwEHA0IABIp7TzDxkx6J9aBqUyHTscJYGdGKwIpgldkHRI3kW3T6IcAS
// D5jXmRRQLm6W+ElUM7RQCkAf+Fkgw9D2G/DAA36jPzA9MA4GA1UdDwEB/wQEAwIF
// oDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAK
// BggqhkjOPQQDAgNJADBGAiEApqrWJZlPVyWQk+B1fIwgI8O15mLgLi834Df4z+DR
// uEcCIQClwM8wrWTiK2ocDIhdG3DpkxBIU0IfhQSmxJLu6h6PgA==
// -----END CERTIFICATE-----
// -----BEGIN CERTIFICATE-----
// MIIBOTCB4KADAgECAhBOk9nVhfYiH0/+qx9tS0MdMAoGCCqGSM49BAMCMAAwIhgP
// MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
// CCqGSM49AwEHA0IABNln9hOovnij6w6d5TKClc/q4/Cv+JxXtzKc5f/Fb/bTJ3yg
// U7ytuq7pLjxsCkaLk8EyAxv2JtrGLeaWLJJBNtejODA2MA4GA1UdDwEB/wQEAwIC
// BDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49
// BAMCA0gAMEUCIQCKgeWh5n3MOHUDzpcj+083CMmILqyzMov/C3NmS54sgQIgWWDB
// FySG2fSnNA8UBKIhPQ6JM8/QbZ9LJvJJ1ctJQy0=
// -----END CERTIFICATE-----
// -----BEGIN EC PRIVATE KEY-----
// MHcCAQEEIA7gJKDqIZsMmqe1t4qvJqeSYlZ5q7Ry6PeNrVNguD0toAoGCCqGSM49
// AwEHoUQDQgAEintPMPGTHon1oGpTIdOxwlgZ0YrAimCV2QdEjeRbdPohwBIPmNeZ
// FFAubpb4SVQztFAKQB/4WSDD0PYb8MADfg==
// -----END EC PRIVATE KEY-----`
//
// 	knownDifficulty := uint16(10)
// 	hashLen := uint16(38)
//
// 	cert, err := read([]byte(idPEM))
// 	assert.NoError(t, err)
//
// 	kadCreds, err := CertToCreds(cert, hashLen)
// 	assert.NoError(t, err)
//
// 	difficulty := kadCreds.Difficulty()
// 	assert.True(t, difficulty >= knownDifficulty)
// }
//
// func TestKadCreds_Difficulty_SLOW(t *testing.T) {
// 	t.SkipNow()
//
// 	var creds *FullIdentity
// 	expectedDifficulty := uint16(24)
// 	hashLen := uint16(38)
//
// 	c, err := NewID(expectedDifficulty, hashLen, 5)
// 	assert.NoError(t, err)
//
// 	creds = c.(*FullIdentity)
// 	assert.True(t, creds.Difficulty() >= expectedDifficulty)
// }
//
// func TestNewID(t *testing.T) {
// 	hashLen := uint16(38)
// 	expectedDifficulty := uint16(16)
//
// 	nodeID, err := NewID(expectedDifficulty, hashLen, 2)
// 	kadCreds := nodeID.(*FullIdentity)
//
// 	assert.NoError(t, err)
// 	assert.NotNil(t, kadCreds)
// 	assert.NotEmpty(t, *kadCreds)
//
// 	actualDifficulty := kadCreds.Difficulty()
// 	assert.True(t, actualDifficulty >= expectedDifficulty)
// }
//
// func BenchmarkNewID_Diffiiculty8_Concurrency1(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		hashLen := uint16(38)
// 		expectedDifficulty := uint16(8)
//
// 		NewID(expectedDifficulty, hashLen, 1)
// 	}
// }
//
// func BenchmarkNewID_Diffiiculty8_Concurrency2(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		hashLen := uint16(38)
// 		expectedDifficulty := uint16(8)
//
// 		NewID(expectedDifficulty, hashLen, 2)
// 	}
// }
//
// func BenchmarkNewID_Diffiiculty8_Concurrency5(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		hashLen := uint16(38)
// 		expectedDifficulty := uint16(8)
//
// 		NewID(expectedDifficulty, hashLen, 5)
// 	}
// }
//
// func BenchmarkNewID_Diffiiculty8_Concurrency10(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		hashLen := uint16(38)
// 		expectedDifficulty := uint16(8)
//
// 		NewID(expectedDifficulty, hashLen, 10)
// 	}
// }
