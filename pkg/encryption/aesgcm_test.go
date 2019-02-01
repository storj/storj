// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"bytes"
	"io/ioutil"
	"testing"

	"storj.io/storj/pkg/storj"
)

func TestAesGcm(t *testing.T) {
	var key storj.Key
	copy(key[:], randData(storj.KeySize))
	var firstNonce AESGCMNonce
	copy(firstNonce[:], randData(AESGCMNonceSize))
	encrypter, err := NewAESGCMEncrypter(&key, &firstNonce, 4*1024)
	if err != nil {
		t.Fatal(err)
	}
	data := randData(encrypter.InBlockSize() * 10)
	encrypted := TransformReader(
		ioutil.NopCloser(bytes.NewReader(data)), encrypter, 0)
	decrypter, err := NewAESGCMDecrypter(&key, &firstNonce, 4*1024)
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
