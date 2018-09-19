// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"github.com/zeebo/errs"
)

// Constant definitions for no encryption (0), AESGCM (1), and SecretBox (2)
const (
	None = iota
	AESGCM
	SecretBox
)

// Encrypt encrypts byte data with a key and nonce. The cipher data is returned
// The type of encryption to use can be modified with encType
func Encrypt(data []byte, key *[32]byte, nonce *[24]byte, encType int) (cipherData []byte, err error) {
	switch encType {
	case None:
		return data, nil
	case AESGCM:
		return EncryptAESGCM(data, key[:], nonce[12:])
	case SecretBox:

		return EncryptSecretBox(data, key, nonce)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

// Decrypt decrypts byte data with a key and nonce. The plain data is returned
// The type of encryption to use can be modified with encType
func Decrypt(cipherData []byte, key *[32]byte, nonce *[24]byte, encType int) (data []byte, err error) {
	switch encType {
	case None:
		return cipherData, nil
	case AESGCM:
		return DecryptAESGCM(cipherData, key[:], nonce[12:])
	case SecretBox:
		return DecryptSecretBox(cipherData, key, nonce)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

// NewEncrypter creates transform stream using a key and a nonce to encrypt data passing through it
// The type of encryption to use can be modified with encType
func NewEncrypter(key *[32]byte, startingNonce *[24]byte,
	encBlockSize, encType int) (Transformer, error) {
	switch encType {
	case None:
		return &NoopTransformer{}, nil
	case AESGCM:
		nonce := new([12]byte)
		copy((*nonce)[:], (*startingNonce)[12:])
		return NewAESGCMEncrypter(key, nonce, encBlockSize)
	case SecretBox:
		return NewSecretboxEncrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

// NewDecrypter creates transform stream using a key and a nonce to decrypt data passing through it
// The type of encryption to use can be modified with encType
func NewDecrypter(key *[32]byte, startingNonce *[24]byte,
	encBlockSize, encType int) (Transformer, error) {
	switch encType {
	case None:
		return &NoopTransformer{}, nil
	case AESGCM:
		nonce := new([12]byte)
		copy((*nonce)[:], (*startingNonce)[12:])
		return NewAESGCMDecrypter(key, nonce, encBlockSize)
	case SecretBox:
		return NewSecretboxDecrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}
