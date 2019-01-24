// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"testing"

	"storj.io/storj/pkg/storj"
)

func randData(amount int) []byte {
	buf := make([]byte, amount)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return buf
}

func TestSecretbox(t *testing.T) {
	var key storj.Key
	copy(key[:], randData(storj.KeySize))
	var firstNonce storj.Nonce
	copy(firstNonce[:], randData(storj.NonceSize))
	encrypter, err := NewSecretboxEncrypter(&key, &firstNonce, 4*1024)
	if err != nil {
		t.Fatal(err)
	}
	data := randData(encrypter.InBlockSize() * 10)
	encrypted := TransformReader(
		ioutil.NopCloser(bytes.NewReader(data)), encrypter, 0)
	decrypter, err := NewSecretboxDecrypter(&key, &firstNonce, 4*1024)
	if err != nil {
		t.Fatal(err)
	}
	decrypted := TransformReader(encrypted, decrypter, 0)
	data2, err := ioutil.ReadAll(decrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("encryption/decryption failed")
	}
}
