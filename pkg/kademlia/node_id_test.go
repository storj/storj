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

	nodeID, err := Load(idPath)
	assert.NoError(t, err)
	assert.NotNil(t, nodeID)

	loadedKadCreds, ok := nodeID.(*KadCreds)
	assert.True(t, ok)

	assert.Equal(t, savedKadCreds.hashLen, loadedKadCreds.hashLen)
	assert.Equal(t, savedKadCreds.hash, loadedKadCreds.hash)
	assert.Equal(t, savedKadCreds.tlsH.Certificate(), loadedKadCreds.tlsH.Certificate())
}

func TestNewID(t *testing.T) {
	path, err := ioutil.TempDir("", "TestLoadID")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	hashLen := uint16(36)
	rootKeyPath := filepath.Join(path, "rootKey.pem")
	nodeID, err := NewID(3, hashLen, 5, rootKeyPath)
	kadCreds := nodeID.(*KadCreds)

	assert.NoError(t, err)
	assert.NotNil(t, kadCreds)
	assert.NotEmpty(t, *kadCreds)

	// wip: resume here...
}
