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
	//aesgcm        AEAD
}

// NewAesGcmEncrypter returns a Transformer that encrypts the data passing
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
func NewAesGcmEncrypter(key *[32]byte, startingNonce *[12]byte,
	encryptedBlockSize int) (Transformer, error) {
	block, err := aes.NewCipher((*key)[:])
	if err != nil {
		panic(err.Error())
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	if encryptedBlockSize <= aesgcm.Overhead() {
		return nil, Error.New("block size too small")
	}
	return &aesgcmEncrypter{
		blockSize:     encryptedBlockSize - aesgcm.Overhead(),
		key:           *key,
		startingNonce: *startingNonce,
		overhead:      aesgcm.Overhead(),
		//aesgcm:        aesgcmEncryp,
	}, nil
}

func (s *aesgcmEncrypter) InBlockSize() int {
	return s.blockSize
}

func (s *aesgcmEncrypter) OutBlockSize() int {
	return s.blockSize + s.overhead
}

func calcGcmNonce(startingNonce *[12]byte, blockNum int64) (rv [12]byte,
	err error) {
	if copy(rv[:], (*startingNonce)[:]) != len(rv) {
		return rv, Error.New("didn't copy memory?!")
	}
	_, err = incrementBytes(rv[:], blockNum)
	return rv, err
}

func (s *aesgcmEncrypter) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		panic(err.Error())
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	n, err := calcGcmNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(out, n[:], in, nil)
	//fmt.Printf("Encryption text %x\n", ciphertext)
	return ciphertext, nil
}

type aesgcmDecrypter struct {
	blockSize     int
	key           [32]byte
	startingNonce [12]byte
	overhead      int
	//aesgcm        AEAD
}

// NewAesGcmDecrypter returns a Transformer that decrypts the data passing
// through with key. See the comments for NewSecretboxEncrypter about
// startingNonce.
func NewAesGcmDecrypter(key *[32]byte, startingNonce *[12]byte,
	encryptedBlockSize int) (Transformer, error) {
	block, err := aes.NewCipher((*key)[:])
	if err != nil {
		panic(err.Error())
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	if encryptedBlockSize <= aesgcm.Overhead() {
		return nil, Error.New("block size too small")
	}
	return &aesgcmDecrypter{
		blockSize:     encryptedBlockSize - aesgcm.Overhead(),
		key:           *key,
		startingNonce: *startingNonce,
		overhead:      aesgcm.Overhead(),
		//aesgcm:        aesgcmDecryp,
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
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		panic(err.Error())
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	n, err := calcGcmNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(out, n[:], in, nil)
	if err != nil {
		panic(err.Error())
	}

	//fmt.Printf("Decryption text %x\n", plaintext)
	return plaintext, nil
}
