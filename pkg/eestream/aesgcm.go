// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"crypto/aes"
	"crypto/cipher"
)

type aesgcmEncrypter struct {
	blockSize     int
	key           [32]byte
	startingNonce [12]byte
	overhead      int
	aesgcm        cipher.AEAD
}

// NewAESGCMEncrypter returns a Transformer that encrypts the data passing
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
func NewAESGCMEncrypter(key *[32]byte, startingNonce *[12]byte,
	encryptedBlockSize int) (Transformer, error) {
	block, err := aes.NewCipher((*key)[:])
	if err != nil {
		return nil, err
	}
	aesgcmEncrypt, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if encryptedBlockSize <= aesgcmEncrypt.Overhead() {
		return nil, Error.New("block size too small")
	}
	return &aesgcmEncrypter{
		blockSize:     encryptedBlockSize - aesgcmEncrypt.Overhead(),
		key:           *key,
		startingNonce: *startingNonce,
		overhead:      aesgcmEncrypt.Overhead(),
		aesgcm:        aesgcmEncrypt,
	}, nil
}

func (s *aesgcmEncrypter) InBlockSize() int {
	return s.blockSize
}

func (s *aesgcmEncrypter) OutBlockSize() int {
	return s.blockSize + s.overhead
}

func calcGCMNonce(startingNonce *[12]byte, blockNum int64) (rv [12]byte,
	err error) {
	if copy(rv[:], (*startingNonce)[:]) != len(rv) {
		return rv, Error.New("didn't copy memory?!")
	}
	_, err = incrementBytes(rv[:], blockNum)
	return rv, err
}

func (s *aesgcmEncrypter) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	n, err := calcGCMNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}

	ciphertext := s.aesgcm.Seal(out, n[:], in, nil)
	return ciphertext, nil
}

type aesgcmDecrypter struct {
	blockSize     int
	key           [32]byte
	startingNonce [12]byte
	overhead      int
	aesgcm        cipher.AEAD
}

// NewAESGCMDecrypter returns a Transformer that decrypts the data passing
// through with key. See the comments for NewAESGCMEncrypter about
// startingNonce.
func NewAESGCMDecrypter(key *[32]byte, startingNonce *[12]byte,
	encryptedBlockSize int) (Transformer, error) {
	block, err := aes.NewCipher((*key)[:])
	if err != nil {
		return nil, err
	}
	aesgcmDecrypt, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if encryptedBlockSize <= aesgcmDecrypt.Overhead() {
		return nil, Error.New("block size too small")
	}
	return &aesgcmDecrypter{
		blockSize:     encryptedBlockSize - aesgcmDecrypt.Overhead(),
		key:           *key,
		startingNonce: *startingNonce,
		overhead:      aesgcmDecrypt.Overhead(),
		aesgcm:        aesgcmDecrypt,
	}, nil
}
func (s *aesgcmDecrypter) InBlockSize() int {
	return s.blockSize + s.overhead
}

func (s *aesgcmDecrypter) OutBlockSize() int {
	return s.blockSize
}

func (s *aesgcmDecrypter) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	n, err := calcGCMNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}

	return s.aesgcm.Open(out, n[:], in, nil)
}
