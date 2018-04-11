// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"testing"
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
	key := randData(32)
	encrypter, err := NewSecretboxEncrypter(key, 4*1024)
	if err != nil {
		t.Fatal(err)
	}
	data := randData(encrypter.InBlockSize() * 10)
	encrypted := TransformReader(bytes.NewReader(data),
		encrypter, 0)
	decrypter, err := NewSecretboxDecrypter(key, 4*1024)
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
