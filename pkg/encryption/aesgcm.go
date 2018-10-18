// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/zeebo/errs"
)

type aesgcmEncrypter struct {
	blockSize     int
	key           Key
	startingNonce AESGCMNonce
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
func NewAESGCMEncrypter(key *Key, startingNonce *AESGCMNonce, encryptedBlockSize int) (Transformer, error) {
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

func calcGCMNonce(startingNonce *AESGCMNonce, blockNum int64) (rv [12]byte, err error) {
	if copy(rv[:], (*startingNonce)[:]) != len(rv) {
		return rv, Error.New("didn't copy memory?!")
	}
	_, err = incrementBytes(rv[:], blockNum)
	return rv, err
}

func (s *aesgcmEncrypter) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	n, err := calcGCMNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}

	ciphertext := s.aesgcm.Seal(out, n[:], in, nil)
	return ciphertext, nil
}

type aesgcmDecrypter struct {
	blockSize     int
	key           Key
	startingNonce AESGCMNonce
	overhead      int
	aesgcm        cipher.AEAD
}

// NewAESGCMDecrypter returns a Transformer that decrypts the data passing
// through with key. See the comments for NewAESGCMEncrypter about
// startingNonce.
func NewAESGCMDecrypter(key *Key, startingNonce *AESGCMNonce, encryptedBlockSize int) (Transformer, error) {
	block, err := aes.NewCipher(key[:])
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

func (s *aesgcmDecrypter) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	n, err := calcGCMNonce(&s.startingNonce, blockNum)
	if err != nil {
		return nil, err
	}

	return s.aesgcm.Open(out, n[:], in, nil)
}

// EncryptAESGCM encrypts byte data with a key and nonce. The cipher data is returned
func EncryptAESGCM(data []byte, key *Key, nonce *AESGCMNonce) (cipherData []byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return []byte{}, errs.Wrap(err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return []byte{}, errs.Wrap(err)
	}
	cipherData = aesgcm.Seal(nil, nonce[:], data, nil)
	return cipherData, nil
}

// DecryptAESGCM decrypts byte data with a key and nonce. The plain data is returned
func DecryptAESGCM(cipherData []byte, key *Key, nonce *AESGCMNonce) (data []byte, err error) {
	if len(cipherData) == 0 {
		return []byte{}, errs.New("empty cipher data")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return []byte{}, errs.Wrap(err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return []byte{}, errs.Wrap(err)
	}
	decrypted, err := aesgcm.Open(nil, nonce[:], cipherData, nil)
	if err != nil {
		return []byte{}, errs.Wrap(err)
	}
	return decrypted, nil
}
