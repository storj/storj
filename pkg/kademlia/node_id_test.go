// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

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
	kadCreds, err := CertToKadCreds(&cert, hashLen)
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
	kadCreds, err := CertToKadCreds(&cert, hashLen)
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
	kadCreds, err := CertToKadCreds(&cert, hashLen)
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
	kadCreds, err := CertToKadCreds(&cert, hashLen)
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

	savedKadCreds, err := CertToKadCreds(&cert, hashLen)
	assert.NoError(t, err)

	idPath := filepath.Join(path, "id.pem")
	err = savedKadCreds.Save(idPath)
	assert.NoError(t, err)

	loadedKadCreds, err := Load(idPath)
	assert.NoError(t, err)
	assert.NotNil(t, loadedKadCreds)

	assert.Equal(t, savedKadCreds.hashLen, loadedKadCreds.hashLen)
	assert.Equal(t, savedKadCreds.hash, loadedKadCreds.hash)
	assert.Equal(t, savedKadCreds.tlsH.Certificate(), loadedKadCreds.tlsH.Certificate())
}

func TestKadCreds_Difficulty_FAST(t *testing.T) {
	// NB: (hash length: 36 | difficulty: 3)
	var idPemBytes = []byte{45, 45, 45, 45, 45, 66, 69, 71, 73, 78, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 77, 73, 73, 66, 80, 122, 67, 66, 53, 54, 65, 68, 65, 103, 69, 67, 65, 104, 66, 78, 81, 81, 77, 99, 107, 76, 111, 90, 53, 75, 114, 80, 66, 77, 53, 100, 122, 107, 85, 114, 77, 65, 111, 71, 67, 67, 113, 71, 83, 77, 52, 57, 66, 65, 77, 67, 77, 65, 65, 119, 73, 104, 103, 80, 10, 77, 68, 65, 119, 77, 84, 65, 120, 77, 68, 69, 119, 77, 68, 65, 119, 77, 68, 66, 97, 71, 65, 56, 119, 77, 68, 65, 120, 77, 68, 69, 119, 77, 84, 65, 119, 77, 68, 65, 119, 77, 70, 111, 119, 65, 68, 66, 90, 77, 66, 77, 71, 66, 121, 113, 71, 83, 77, 52, 57, 65, 103, 69, 71, 10, 67, 67, 113, 71, 83, 77, 52, 57, 65, 119, 69, 72, 65, 48, 73, 65, 66, 70, 114, 70, 74, 74, 54, 81, 109, 76, 87, 50, 83, 82, 103, 118, 78, 69, 55, 121, 77, 104, 120, 110, 85, 110, 66, 113, 116, 88, 71, 51, 104, 68, 82, 109, 98, 51, 74, 68, 100, 120, 65, 76, 103, 71, 55, 65, 10, 51, 89, 69, 51, 78, 111, 73, 121, 83, 48, 74, 57, 121, 109, 99, 109, 102, 122, 72, 81, 113, 66, 108, 99, 47, 79, 57, 120, 55, 97, 83, 118, 57, 71, 54, 81, 120, 118, 113, 106, 80, 122, 65, 57, 77, 65, 52, 71, 65, 49, 85, 100, 68, 119, 69, 66, 47, 119, 81, 69, 65, 119, 73, 70, 10, 111, 68, 65, 100, 66, 103, 78, 86, 72, 83, 85, 69, 70, 106, 65, 85, 66, 103, 103, 114, 66, 103, 69, 70, 66, 81, 99, 68, 65, 81, 89, 73, 75, 119, 89, 66, 66, 81, 85, 72, 65, 119, 73, 119, 68, 65, 89, 68, 86, 82, 48, 84, 65, 81, 72, 47, 66, 65, 73, 119, 65, 68, 65, 75, 10, 66, 103, 103, 113, 104, 107, 106, 79, 80, 81, 81, 68, 65, 103, 78, 72, 65, 68, 66, 69, 65, 105, 66, 50, 108, 51, 86, 112, 110, 55, 113, 107, 66, 104, 104, 79, 77, 77, 101, 54, 88, 89, 48, 122, 100, 114, 90, 97, 53, 90, 109, 69, 50, 111, 101, 69, 67, 117, 80, 117, 109, 84, 81, 83, 10, 51, 65, 73, 103, 97, 84, 72, 77, 47, 113, 56, 108, 101, 72, 67, 105, 67, 79, 114, 56, 105, 114, 66, 102, 114, 111, 104, 77, 73, 78, 117, 81, 112, 57, 48, 118, 55, 100, 120, 100, 72, 99, 105, 111, 90, 101, 111, 61, 10, 45, 45, 45, 45, 45, 69, 78, 68, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 45, 45, 45, 45, 45, 66, 69, 71, 73, 78, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 77, 73, 73, 66, 79, 106, 67, 66, 52, 75, 65, 68, 65, 103, 69, 67, 65, 104, 66, 118, 70, 77, 122, 55, 87, 73, 89, 66, 70, 76, 110, 69, 78, 73, 101, 84, 112, 98, 116, 49, 77, 65, 111, 71, 67, 67, 113, 71, 83, 77, 52, 57, 66, 65, 77, 67, 77, 65, 65, 119, 73, 104, 103, 80, 10, 77, 68, 65, 119, 77, 84, 65, 120, 77, 68, 69, 119, 77, 68, 65, 119, 77, 68, 66, 97, 71, 65, 56, 119, 77, 68, 65, 120, 77, 68, 69, 119, 77, 84, 65, 119, 77, 68, 65, 119, 77, 70, 111, 119, 65, 68, 66, 90, 77, 66, 77, 71, 66, 121, 113, 71, 83, 77, 52, 57, 65, 103, 69, 71, 10, 67, 67, 113, 71, 83, 77, 52, 57, 65, 119, 69, 72, 65, 48, 73, 65, 66, 71, 121, 105, 69, 79, 116, 51, 50, 106, 101, 103, 81, 85, 73, 71, 110, 79, 89, 107, 86, 109, 69, 97, 97, 119, 80, 77, 72, 84, 108, 71, 110, 112, 115, 106, 88, 102, 55, 49, 70, 108, 54, 121, 54, 69, 114, 89, 10, 89, 112, 98, 100, 79, 76, 101, 53, 121, 82, 87, 86, 90, 87, 112, 113, 56, 70, 47, 86, 122, 75, 111, 84, 72, 79, 65, 80, 81, 105, 69, 98, 114, 53, 74, 81, 68, 76, 117, 106, 79, 68, 65, 50, 77, 65, 52, 71, 65, 49, 85, 100, 68, 119, 69, 66, 47, 119, 81, 69, 65, 119, 73, 67, 10, 66, 68, 65, 84, 66, 103, 78, 86, 72, 83, 85, 69, 68, 68, 65, 75, 66, 103, 103, 114, 66, 103, 69, 70, 66, 81, 99, 68, 65, 84, 65, 80, 66, 103, 78, 86, 72, 82, 77, 66, 65, 102, 56, 69, 66, 84, 65, 68, 65, 81, 72, 47, 77, 65, 111, 71, 67, 67, 113, 71, 83, 77, 52, 57, 10, 66, 65, 77, 67, 65, 48, 107, 65, 77, 69, 89, 67, 73, 81, 68, 115, 108, 50, 78, 77, 49, 97, 66, 110, 113, 112, 108, 80, 85, 65, 106, 52, 119, 51, 108, 97, 52, 122, 72, 53, 108, 54, 51, 79, 102, 85, 87, 74, 84, 107, 81, 74, 118, 122, 109, 73, 54, 81, 73, 104, 65, 75, 67, 109, 10, 49, 112, 67, 76, 80, 115, 83, 113, 57, 67, 80, 55, 88, 54, 110, 54, 107, 75, 116, 116, 83, 112, 69, 113, 52, 70, 108, 88, 75, 49, 81, 81, 108, 68, 53, 99, 110, 120, 57, 54, 10, 45, 45, 45, 45, 45, 69, 78, 68, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 45, 45, 45, 45, 45, 66, 69, 71, 73, 78, 32, 69, 67, 32, 80, 82, 73, 86, 65, 84, 69, 32, 75, 69, 89, 45, 45, 45, 45, 45, 10, 77, 72, 99, 67, 65, 81, 69, 69, 73, 80, 47, 81, 47, 97, 99, 104, 98, 78, 67, 67, 117, 80, 105, 122, 102, 122, 85, 99, 101, 57, 83, 86, 47, 100, 111, 97, 67, 105, 74, 98, 119, 120, 115, 48, 75, 70, 117, 109, 106, 83, 106, 76, 111, 65, 111, 71, 67, 67, 113, 71, 83, 77, 52, 57, 10, 65, 119, 69, 72, 111, 85, 81, 68, 81, 103, 65, 69, 87, 115, 85, 107, 110, 112, 67, 89, 116, 98, 90, 74, 71, 67, 56, 48, 84, 118, 73, 121, 72, 71, 100, 83, 99, 71, 113, 49, 99, 98, 101, 69, 78, 71, 90, 118, 99, 107, 78, 51, 69, 65, 117, 65, 98, 115, 68, 100, 103, 84, 99, 50, 10, 103, 106, 74, 76, 81, 110, 51, 75, 90, 121, 90, 47, 77, 100, 67, 111, 71, 86, 122, 56, 55, 51, 72, 116, 112, 75, 47, 48, 98, 112, 68, 71, 43, 103, 61, 61, 10, 45, 45, 45, 45, 45, 69, 78, 68, 32, 69, 67, 32, 80, 82, 73, 86, 65, 84, 69, 32, 75, 69, 89, 45, 45, 45, 45, 45, 10, 36, 0}
	knownDifficulty := uint16(3)

	cert, hashLen, err := read(idPemBytes)
	assert.NoError(t, err)

	kadCreds, err := CertToKadCreds(cert, hashLen)
	assert.NoError(t, err)

	difficulty := kadCreds.Difficulty()
	assert.True(t, difficulty >= knownDifficulty)
}

func TestKadCreds_Difficulty_SLOW(t *testing.T) {
	t.SkipNow()

	expectedDifficulty := uint16(3)
	var kadCreds *KadCreds
	hashLen := uint16(38)
	for {
		tlsH, err := peertls.NewTLSHelper(nil)
		assert.NoError(t, err)

		cert := tlsH.Certificate()
		kadCreds, err = CertToKadCreds(&cert, hashLen)
		assert.NoError(t, err)
		assert.Equal(t, cert, kadCreds.tlsH.Certificate())
		assert.Equal(t, hashLen, kadCreds.hashLen)
		assert.NotEmpty(t, kadCreds.hash)

		actualDifficulty := uint16(0)
		hash := kadCreds.Hash()
		for i := 1; i < len(hash); i++ {
			b := hash[len(hash)-i]

			if b != 0 {
				break
			}

			actualDifficulty = uint16(i)
		}

		if actualDifficulty >= expectedDifficulty {
			break
		}
	}

	difficulty := kadCreds.Difficulty()
	assert.True(t, difficulty >= expectedDifficulty)
}

func TestNewID(t *testing.T) {
	path, err := ioutil.TempDir("", "TestLoadID")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	hashLen := uint16(38)
	rootKeyPath := filepath.Join(path, "rootKey.pem")

	expectedDifficulty := uint16(2)

	nodeID, err := NewID(expectedDifficulty, hashLen, 2, rootKeyPath)
	kadCreds := nodeID.(*KadCreds)

	assert.NoError(t, err)
	assert.NotNil(t, kadCreds)
	assert.NotEmpty(t, *kadCreds)

	actualDifficulty := kadCreds.Difficulty()
	assert.True(t, actualDifficulty >= expectedDifficulty)
}
