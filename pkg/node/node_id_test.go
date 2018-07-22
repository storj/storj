// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"bytes"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/peertls"
)

func Test_certToKadCreds(t *testing.T) {
	hashLen := uint16(128)
	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	kadCreds, err := CertToCreds(&cert, hashLen)
	assert.NoError(t, err)
	assert.Equal(t, cert, kadCreds.tlsH.Certificate())
	assert.Equal(t, hashLen, kadCreds.hashLen)
	assert.NotEmpty(t, kadCreds.hash)
}

func TestParseID(t *testing.T) {
	hashLen := uint16(128)
	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	kadCreds, err := CertToCreds(&cert, hashLen)
	assert.NoError(t, err)

	kadID, err := ParseID(kadCreds.String())
	assert.NoError(t, err)
	assert.Equal(t, kadID.hashLen, kadCreds.hashLen)
	assert.Equal(t, kadID.hash, kadCreds.hash)

	pubKey := kadCreds.tlsH.PubKey()
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&pubKey)
	assert.NoError(t, err)
	assert.Equal(t, kadID.pubKey, pubKeyBytes)
}

func TestKadCreds_Bytes(t *testing.T) {
	// TODO(bryanchriswhite): 38 is possibly a nice default hash length:
	//   + it fills the last base64(url) block
	//     (the hash will read the same when separated
	//     from the key portion of an id)
	//   + hash+key+hashLen also fills the last base64(url) block
	hashLen := uint16(36)
	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	kadCreds, err := CertToCreds(&cert, hashLen)
	assert.NoError(t, err)

	kadCredBytes := kadCreds.Bytes()
	assert.NotNil(t, kadCredBytes)
}

func TestKadCreds_Save(t *testing.T) {
	path, err := ioutil.TempDir("", "TestKadCreds_Save")
	assert.NoError(t, err)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	hashLen := uint16(128)
	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	kadCreds, err := CertToCreds(&cert, hashLen)
	assert.NoError(t, err)

	idPath := filepath.Join(path, "id.pem")
	err = kadCreds.Save(idPath)
	assert.NoError(t, err)

	idBytes, err := ioutil.ReadFile(idPath)
	assert.NoError(t, err)
	assert.NotNil(t, idBytes)

	kadCredBytes := bytes.NewBuffer([]byte{})
	err = kadCreds.write(kadCredBytes)
	assert.NoError(t, err)

	bytesEqual := bytes.Compare(idBytes, kadCredBytes.Bytes()) == 0
	assert.True(t, bytesEqual)

	fileInfo, err := os.Stat(idPath)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fileInfo.Mode())
}

func TestLoadID(t *testing.T) {
	path, err := ioutil.TempDir("", "TestLoadID")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	hashLen := uint16(36)
	tlsH, err := peertls.NewTLSHelper(nil)
	assert.NoError(t, err)

	cert := tlsH.Certificate()
	assert.NoError(t, err)

	savedKadCreds, err := CertToCreds(&cert, hashLen)
	assert.NoError(t, err)

	idPath := filepath.Join(path, "id.pem")
	err = savedKadCreds.Save(idPath)
	assert.NoError(t, err)

	loadedKadCreds, err := LoadID(idPath, hashLen)
	assert.NoError(t, err)
	assert.NotNil(t, loadedKadCreds)

	assert.Equal(t, savedKadCreds.hashLen, loadedKadCreds.hashLen)
	assert.Equal(t, savedKadCreds.hash, loadedKadCreds.hash)
	assert.Equal(t, savedKadCreds.tlsH.Certificate(), loadedKadCreds.tlsH.Certificate())
}

func TestKadCreds_Difficulty_FAST(t *testing.T) {
	// NB: (hash length: 38 | difficulty: 3)
	var idPEM = `-----BEGIN CERTIFICATE-----
MIIBQTCB56ADAgECAhA9qbPSqt8UJt6jGuT8FEC0MAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABIp7TzDxkx6J9aBqUyHTscJYGdGKwIpgldkHRI3kW3T6IcAS
D5jXmRRQLm6W+ElUM7RQCkAf+Fkgw9D2G/DAA36jPzA9MA4GA1UdDwEB/wQEAwIF
oDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAK
BggqhkjOPQQDAgNJADBGAiEApqrWJZlPVyWQk+B1fIwgI8O15mLgLi834Df4z+DR
uEcCIQClwM8wrWTiK2ocDIhdG3DpkxBIU0IfhQSmxJLu6h6PgA==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBOTCB4KADAgECAhBOk9nVhfYiH0/+qx9tS0MdMAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABNln9hOovnij6w6d5TKClc/q4/Cv+JxXtzKc5f/Fb/bTJ3yg
U7ytuq7pLjxsCkaLk8EyAxv2JtrGLeaWLJJBNtejODA2MA4GA1UdDwEB/wQEAwIC
BDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49
BAMCA0gAMEUCIQCKgeWh5n3MOHUDzpcj+083CMmILqyzMov/C3NmS54sgQIgWWDB
FySG2fSnNA8UBKIhPQ6JM8/QbZ9LJvJJ1ctJQy0=
-----END CERTIFICATE-----
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIA7gJKDqIZsMmqe1t4qvJqeSYlZ5q7Ry6PeNrVNguD0toAoGCCqGSM49
AwEHoUQDQgAEintPMPGTHon1oGpTIdOxwlgZ0YrAimCV2QdEjeRbdPohwBIPmNeZ
FFAubpb4SVQztFAKQB/4WSDD0PYb8MADfg==
-----END EC PRIVATE KEY-----`

	knownDifficulty := uint16(10)
	hashLen := uint16(38)

	cert, err := read([]byte(idPEM))
	assert.NoError(t, err)

	kadCreds, err := CertToCreds(cert, hashLen)
	assert.NoError(t, err)

	difficulty := kadCreds.Difficulty()
	assert.True(t, difficulty >= knownDifficulty)
}

func TestKadCreds_Difficulty_SLOW(t *testing.T) {
	t.SkipNow()

	var creds *Creds
	expectedDifficulty := uint16(24)
	hashLen := uint16(38)

	c, err := NewID(expectedDifficulty, hashLen, 5)
	assert.NoError(t, err)

	creds = c.(*Creds)
	assert.True(t, creds.Difficulty() >= expectedDifficulty)
}

func TestNewID(t *testing.T) {
	hashLen := uint16(38)
	expectedDifficulty := uint16(16)

	nodeID, err := NewID(expectedDifficulty, hashLen, 2)
	kadCreds := nodeID.(*Creds)

	assert.NoError(t, err)
	assert.NotNil(t, kadCreds)
	assert.NotEmpty(t, *kadCreds)

	actualDifficulty := kadCreds.Difficulty()
	assert.True(t, actualDifficulty >= expectedDifficulty)
}

func BenchmarkNewID_Diffiiculty8_Concurrency1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashLen := uint16(38)
		expectedDifficulty := uint16(8)

		NewID(expectedDifficulty, hashLen, 1)
	}
}

func BenchmarkNewID_Diffiiculty8_Concurrency2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashLen := uint16(38)
		expectedDifficulty := uint16(8)

		NewID(expectedDifficulty, hashLen, 2)
	}
}

func BenchmarkNewID_Diffiiculty8_Concurrency5(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashLen := uint16(38)
		expectedDifficulty := uint16(8)

		NewID(expectedDifficulty, hashLen, 5)
	}
}

func BenchmarkNewID_Diffiiculty8_Concurrency10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashLen := uint16(38)
		expectedDifficulty := uint16(8)

		NewID(expectedDifficulty, hashLen, 10)
	}
}
