// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"encoding/binary"

	"golang.org/x/crypto/nacl/secretbox"
)

type secretboxEncrypter struct {
	blockSize int
	key       [32]byte
}

func setKey(dst *[32]byte, key []byte) error {
	if len((*dst)[:]) != len(key) {
		return Error.New("invalid key length, expected %d", len((*dst)[:]))
	}
	copy((*dst)[:], key)
	return nil
}

// NewSecretboxEncrypter returns a Transformer that encrypts the data passing
// through with key.
func NewSecretboxEncrypter(key []byte, encryptedBlockSize int) (
	Transformer, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, Error.New("block size too small")
	}
	rv := &secretboxEncrypter{blockSize: encryptedBlockSize - secretbox.Overhead}
	return rv, setKey(&rv.key, key)
}

func (s *secretboxEncrypter) InBlockSize() int {
	return s.blockSize
}

func (s *secretboxEncrypter) OutBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func calcNonce(blockNum int64) *[24]byte {
	var buf [uint32Size]byte
	binary.BigEndian.PutUint32(buf[:], uint32(blockNum))
	var nonce [24]byte
	copy(nonce[:], buf[1:])
	return &nonce
}

func (s *secretboxEncrypter) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	return secretbox.Seal(out, in, calcNonce(blockNum), &s.key), nil
}

type secretboxDecrypter struct {
	blockSize int
	key       [32]byte
}

// NewSecretboxDecrypter returns a Transformer that decrypts the data passing
// through with key.
func NewSecretboxDecrypter(key []byte, encryptedBlockSize int) (
	Transformer, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, Error.New("block size too small")
	}
	rv := &secretboxDecrypter{blockSize: encryptedBlockSize - secretbox.Overhead}
	return rv, setKey(&rv.key, key)
}

func (s *secretboxDecrypter) InBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func (s *secretboxDecrypter) OutBlockSize() int {
	return s.blockSize
}

func (s *secretboxDecrypter) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	rv, success := secretbox.Open(out, in, calcNonce(blockNum), &s.key)
	if !success {
		return nil, Error.New("failed decrypting")
	}
	return rv, nil
}
