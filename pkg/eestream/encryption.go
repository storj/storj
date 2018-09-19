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

// Constant definitions for key and nonce sizes
const (
	GenericKeySize   = 32
	GenericNonceSize = 24
	AESGCMNonceSize  = 12
)

// GenericKey represents the largest key used by any encryption protocol
type GenericKey [GenericKeySize]byte

// GenericNonce represents the largest nonce used by any encryption protocol
type GenericNonce [GenericNonceSize]byte

// AESGCMNonce represents the nonce used by the AESGCM protocol
type AESGCMNonce [AESGCMNonceSize]byte

// Encrypt encrypts byte data with a key and nonce. The cipher data is returned
// The type of encryption to use can be modified with encType
func Encrypt(data []byte, key *GenericKey, nonce *GenericNonce, encType int) (cipherData []byte, err error) {
	switch encType {
	case None:
		return data, nil
	case AESGCM:
		return EncryptAESGCM(data, key[:], nonce[AESGCMNonceSize:])
	case SecretBox:
		return EncryptSecretBox(data, key, nonce)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

// Decrypt decrypts byte data with a key and nonce. The plain data is returned
// The type of encryption to use can be modified with encType
func Decrypt(cipherData []byte, key *GenericKey, nonce *GenericNonce, encType int) (data []byte, err error) {
	switch encType {
	case None:
		return cipherData, nil
	case AESGCM:
		return DecryptAESGCM(cipherData, key[:], nonce[AESGCMNonceSize:])
	case SecretBox:
		return DecryptSecretBox(cipherData, key, nonce)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

// NewEncrypter creates transform stream using a key and a nonce to encrypt data passing through it
// The type of encryption to use can be modified with encType
func NewEncrypter(key *GenericKey, startingNonce *GenericNonce, encBlockSize, encType int) (Transformer, error) {
	switch encType {
	case None:
		return &NoopTransformer{}, nil
	case AESGCM:
		nonce := new(AESGCMNonce)
		copy((*nonce)[:], (*startingNonce)[AESGCMNonceSize:])
		return NewAESGCMEncrypter(key, nonce, encBlockSize)
	case SecretBox:
		return NewSecretboxEncrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

// NewDecrypter creates transform stream using a key and a nonce to decrypt data passing through it
// The type of encryption to use can be modified with encType
func NewDecrypter(key *GenericKey, startingNonce *GenericNonce, encBlockSize, encType int) (Transformer, error) {
	switch encType {
	case None:
		return &NoopTransformer{}, nil
	case AESGCM:
		nonce := new(AESGCMNonce)
		copy((*nonce)[:], (*startingNonce)[AESGCMNonceSize:])
		return NewAESGCMDecrypter(key, nonce, encBlockSize)
	case SecretBox:
		return NewSecretboxDecrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}
