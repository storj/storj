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
	"fmt"
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

	nodeID, err := Load(idPath)
	assert.NoError(t, err)
	assert.NotNil(t, nodeID)

	loadedKadCreds, ok := nodeID.(*KadCreds)
	assert.True(t, ok)

	assert.Equal(t, savedKadCreds.hashLen, loadedKadCreds.hashLen)
	assert.Equal(t, savedKadCreds.hash, loadedKadCreds.hash)
	assert.Equal(t, savedKadCreds.tlsH.Certificate(), loadedKadCreds.tlsH.Certificate())
}

func TestKadCreds_Difficulty(t *testing.T) {
	// var idPemBytes = []byte{45, 45, 45, 45, 45, 66, 69, 71, 73, 78, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 77, 73, 73, 66, 81, 68, 67, 66, 54, 75, 65, 68, 65, 103, 69, 67, 65, 104, 69, 65, 116, 75, 84, 86, 121, 119, 88, 85, 54, 51, 98, 57, 49, 88, 47, 65, 114, 82, 112, 87, 48, 68, 65, 75, 66, 103, 103, 113, 104, 107, 106, 79, 80, 81, 81, 68, 65, 106, 65, 65, 77, 67, 73, 89, 10, 68, 122, 65, 119, 77, 68, 69, 119, 77, 84, 65, 120, 77, 68, 65, 119, 77, 68, 65, 119, 87, 104, 103, 80, 77, 68, 65, 119, 77, 84, 65, 120, 77, 68, 69, 119, 77, 68, 65, 119, 77, 68, 66, 97, 77, 65, 65, 119, 87, 84, 65, 84, 66, 103, 99, 113, 104, 107, 106, 79, 80, 81, 73, 66, 10, 66, 103, 103, 113, 104, 107, 106, 79, 80, 81, 77, 66, 66, 119, 78, 67, 65, 65, 82, 83, 47, 103, 56, 85, 97, 82, 87, 104, 54, 121, 76, 48, 65, 50, 52, 86, 122, 80, 84, 122, 72, 101, 52, 80, 104, 120, 87, 56, 81, 51, 83, 109, 105, 117, 79, 122, 76, 52, 103, 100, 98, 108, 121, 107, 10, 107, 48, 67, 82, 120, 112, 116, 112, 98, 122, 43, 99, 69, 56, 56, 55, 111, 52, 112, 68, 81, 71, 108, 90, 116, 81, 108, 55, 73, 100, 52, 83, 88, 106, 109, 113, 55, 77, 75, 43, 111, 122, 56, 119, 80, 84, 65, 79, 66, 103, 78, 86, 72, 81, 56, 66, 65, 102, 56, 69, 66, 65, 77, 67, 10, 66, 97, 65, 119, 72, 81, 89, 68, 86, 82, 48, 108, 66, 66, 89, 119, 70, 65, 89, 73, 75, 119, 89, 66, 66, 81, 85, 72, 65, 119, 69, 71, 67, 67, 115, 71, 65, 81, 85, 70, 66, 119, 77, 67, 77, 65, 119, 71, 65, 49, 85, 100, 69, 119, 69, 66, 47, 119, 81, 67, 77, 65, 65, 119, 10, 67, 103, 89, 73, 75, 111, 90, 73, 122, 106, 48, 69, 65, 119, 73, 68, 82, 119, 65, 119, 82, 65, 73, 103, 75, 69, 84, 54, 81, 69, 112, 77, 54, 80, 51, 87, 122, 113, 122, 71, 80, 72, 65, 111, 47, 97, 66, 121, 108, 90, 113, 106, 73, 48, 75, 97, 56, 55, 103, 53, 53, 121, 78, 50, 10, 117, 116, 73, 67, 73, 65, 50, 101, 107, 51, 56, 111, 97, 86, 48, 122, 69, 75, 73, 84, 43, 73, 105, 43, 121, 90, 55, 84, 84, 73, 101, 118, 81, 102, 103, 51, 105, 86, 118, 105, 105, 76, 57, 73, 104, 76, 103, 79, 10, 45, 45, 45, 45, 45, 69, 78, 68, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 45, 45, 45, 45, 45, 66, 69, 71, 73, 78, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 77, 73, 73, 66, 79, 84, 67, 66, 52, 97, 65, 68, 65, 103, 69, 67, 65, 104, 69, 65, 119, 118, 70, 88, 47, 97, 108, 50, 107, 56, 85, 100, 98, 48, 54, 78, 120, 87, 66, 88, 54, 68, 65, 75, 66, 103, 103, 113, 104, 107, 106, 79, 80, 81, 81, 68, 65, 106, 65, 65, 77, 67, 73, 89, 10, 68, 122, 65, 119, 77, 68, 69, 119, 77, 84, 65, 120, 77, 68, 65, 119, 77, 68, 65, 119, 87, 104, 103, 80, 77, 68, 65, 119, 77, 84, 65, 120, 77, 68, 69, 119, 77, 68, 65, 119, 77, 68, 66, 97, 77, 65, 65, 119, 87, 84, 65, 84, 66, 103, 99, 113, 104, 107, 106, 79, 80, 81, 73, 66, 10, 66, 103, 103, 113, 104, 107, 106, 79, 80, 81, 77, 66, 66, 119, 78, 67, 65, 65, 81, 57, 117, 105, 98, 68, 70, 48, 98, 117, 88, 119, 122, 55, 49, 54, 68, 68, 52, 105, 101, 106, 85, 72, 43, 49, 65, 87, 70, 90, 80, 99, 83, 90, 104, 84, 109, 79, 48, 103, 84, 117, 112, 111, 115, 116, 10, 78, 120, 122, 119, 87, 121, 67, 120, 121, 49, 102, 82, 118, 65, 81, 68, 87, 109, 100, 115, 49, 47, 86, 73, 72, 111, 81, 103, 116, 104, 69, 65, 83, 74, 67, 66, 79, 82, 47, 89, 111, 122, 103, 119, 78, 106, 65, 79, 66, 103, 78, 86, 72, 81, 56, 66, 65, 102, 56, 69, 66, 65, 77, 67, 10, 65, 103, 81, 119, 69, 119, 89, 68, 86, 82, 48, 108, 66, 65, 119, 119, 67, 103, 89, 73, 75, 119, 89, 66, 66, 81, 85, 72, 65, 119, 69, 119, 68, 119, 89, 68, 86, 82, 48, 84, 65, 81, 72, 47, 66, 65, 85, 119, 65, 119, 69, 66, 47, 122, 65, 75, 66, 103, 103, 113, 104, 107, 106, 79, 10, 80, 81, 81, 68, 65, 103, 78, 72, 65, 68, 66, 69, 65, 105, 65, 43, 51, 72, 116, 119, 121, 53, 88, 122, 72, 69, 112, 116, 117, 69, 89, 77, 47, 87, 103, 118, 53, 113, 49, 43, 53, 110, 65, 105, 115, 48, 68, 79, 99, 70, 99, 76, 99, 57, 120, 83, 72, 65, 73, 103, 71, 79, 74, 114, 10, 102, 71, 90, 76, 79, 86, 117, 103, 43, 83, 75, 79, 116, 98, 107, 115, 118, 113, 82, 76, 56, 65, 115, 120, 72, 110, 86, 56, 107, 83, 79, 54, 78, 71, 88, 48, 78, 118, 56, 61, 10, 45, 45, 45, 45, 45, 69, 78, 68, 32, 67, 69, 82, 84, 73, 70, 73, 67, 65, 84, 69, 45, 45, 45, 45, 45, 10, 45, 45, 45, 45, 45, 66, 69, 71, 73, 78, 32, 69, 67, 32, 80, 82, 73, 86, 65, 84, 69, 32, 75, 69, 89, 45, 45, 45, 45, 45, 10, 77, 72, 99, 67, 65, 81, 69, 69, 73, 65, 71, 101, 73, 77, 53, 77, 103, 68, 115, 109, 99, 87, 104, 66, 109, 69, 112, 87, 79, 55, 48, 49, 79, 57, 52, 99, 53, 51, 53, 121, 112, 90, 105, 72, 122, 71, 110, 68, 87, 102, 52, 102, 111, 65, 111, 71, 67, 67, 113, 71, 83, 77, 52, 57, 10, 65, 119, 69, 72, 111, 85, 81, 68, 81, 103, 65, 69, 85, 118, 52, 80, 70, 71, 107, 86, 111, 101, 115, 105, 57, 65, 78, 117, 70, 99, 122, 48, 56, 120, 51, 117, 68, 52, 99, 86, 118, 69, 78, 48, 112, 111, 114, 106, 115, 121, 43, 73, 72, 87, 53, 99, 112, 74, 78, 65, 107, 99, 97, 98, 10, 97, 87, 56, 47, 110, 66, 80, 80, 79, 54, 79, 75, 81, 48, 66, 112, 87, 98, 85, 74, 101, 121, 72, 101, 69, 108, 52, 53, 113, 117, 122, 67, 118, 103, 61, 61, 10, 45, 45, 45, 45, 45, 69, 78, 68, 32, 69, 67, 32, 80, 82, 73, 86, 65, 84, 69, 32, 75, 69, 89, 45, 45, 45, 45, 45, 10, 0, 1}

	expectedDifficulty := uint16(1)
	var kadCreds *KadCreds
	hashLen := uint16(256)
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
			fmt.Println(actualDifficulty)
			break
		}
	}

	b := bytes.NewBuffer([]byte{})
	kadCreds.write(b)
	fmt.Println(b.String())
	fmt.Println(b.Bytes()[len(b.Bytes())-36:])
	fmt.Println(kadCreds.Hash()[len(kadCreds.Hash())-5:])
	fmt.Printf("%x\n", "\n")
	fmt.Printf("%v\n", []byte("\n"))
	fmt.Println(b.Bytes())

	difficulty := kadCreds.Difficulty()
	assert.True(t, difficulty > expectedDifficulty)
}

func TestNewID(t *testing.T) {
	path, err := ioutil.TempDir("", "TestLoadID")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	// hashLen := uint16(36)
	hashLen := uint16(128)
	rootKeyPath := filepath.Join(path, "rootKey.pem")

	// hashes := []int{}
	for range make([]bool, 10) {
		expectedDifficulty := uint16(3)

		nodeID, err := NewID(expectedDifficulty, hashLen, 5, rootKeyPath)
		kadCreds := nodeID.(*KadCreds)

		assert.NoError(t, err)
		assert.NotNil(t, kadCreds)
		assert.NotEmpty(t, *kadCreds)

		// actualDifficulty := kadCreds.Difficulty()
		// assert.True(t, actualDifficulty >= expectedDifficulty)
	}
}
