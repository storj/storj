// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"github.com/zeebo/errs"
	"golang.org/x/crypto/nacl/secretbox"
)

type secretboxEncrypter struct {
	blockSize     int
	key           Key
	startingNonce Nonce
}

// NewSecretboxEncrypter returns a Transformer that encrypts the data passing
// through with key.
//
// startingNonce is treated as a big-endian encoded unsigned
// integer, and as blocks pass through, their block number and the starting
// nonce is added together to come up with that block's nonce. Encrypting
// different data with the same key and the same nonce is a huge security
// issue. It's safe to always encode new data with a random key and random
// startingNonce. The monotonically-increasing nonce (that rolls over) is to
// protect against data reordering.
//
// When in doubt, generate a new key from crypto/rand and a startingNonce
// from crypto/rand as often as possible.
func NewSecretboxEncrypter(key *Key, startingNonce *Nonce, encryptedBlockSize int) (Transformer, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, Error.New("block size too small")
	}
	return &secretboxEncrypter{
		blockSize:     encryptedBlockSize - secretbox.Overhead,
		key:           *key,
		startingNonce: *startingNonce,
	}, nil
}

func (s *secretboxEncrypter) InBlockSize() int {
	return s.blockSize
}

func (s *secretboxEncrypter) OutBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func calcNonce(startingNonce *Nonce, blockNum int64) (rv *Nonce, err error) {
	rv = new(Nonce)
	if copy(rv[:], (*startingNonce)[:]) != len(rv) {
		return rv, Error.New("didn't copy memory?!")
	}
	_, err = incrementBytes(rv[:], blockNum)
	return rv, err
}

func (s *secretboxEncrypter) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	n, err := calcNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}
	return secretbox.Seal(out, in, n.Bytes(), s.key.Bytes()), nil
}

type secretboxDecrypter struct {
	blockSize     int
	key           Key
	startingNonce Nonce
}

// NewSecretboxDecrypter returns a Transformer that decrypts the data passing
// through with key. See the comments for NewSecretboxEncrypter about
// startingNonce.
func NewSecretboxDecrypter(key *Key, startingNonce *Nonce, encryptedBlockSize int) (Transformer, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, Error.New("block size too small")
	}
	return &secretboxDecrypter{
		blockSize:     encryptedBlockSize - secretbox.Overhead,
		key:           *key,
		startingNonce: *startingNonce,
	}, nil
}

func (s *secretboxDecrypter) InBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func (s *secretboxDecrypter) OutBlockSize() int {
	return s.blockSize
}

func (s *secretboxDecrypter) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	n, err := calcNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}
	rv, success := secretbox.Open(out, in, n.Bytes(), s.key.Bytes())
	if !success {
		return nil, Error.New("failed decrypting")
	}
	return rv, nil
}

// EncryptSecretBox encrypts byte data with a key and nonce. The cipher data is returned
func EncryptSecretBox(data []byte, key *Key, nonce *Nonce) (cipherData []byte, err error) {
	return secretbox.Seal(nil, data, nonce.Bytes(), key.Bytes()), nil
}

// DecryptSecretBox decrypts byte data with a key and nonce. The plain data is returned
func DecryptSecretBox(cipherData []byte, key *Key, nonce *Nonce) (data []byte, err error) {
	data, success := secretbox.Open(nil, cipherData, nonce.Bytes(), key.Bytes())
	if !success {
		return nil, errs.New("Failed decrypting")
	}
	return data, nil
}
