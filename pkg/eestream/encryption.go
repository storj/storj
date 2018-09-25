// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"github.com/zeebo/errs"
)

// Cipher is a type used to define the type of encryption to use
type Cipher byte

// Constant definitions for no encryption (0), AESGCM (1), and SecretBox (2)
const (
	None = Cipher(iota)
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

// Bytes returns the key as a byte array pointer
func (key *GenericKey) Bytes() *[GenericKeySize]byte {
	return (*[GenericKeySize]byte)(key)
}

// GenericNonce represents the largest nonce used by any encryption protocol
type GenericNonce [GenericNonceSize]byte

// Bytes returns the nonce as a byte array pointer
func (nonce *GenericNonce) Bytes() *[GenericNonceSize]byte {
	return (*[GenericNonceSize]byte)(nonce)
}

// AESGCMNonce returns the nonce as a AES-GCM nonce
func (nonce *GenericNonce) AESGCMNonce() *AESGCMNonce {
	aes := new(AESGCMNonce)
	copy((*aes)[:], (*nonce)[AESGCMNonceSize:])
	return aes
}

// AESGCMNonce represents the nonce used by the AES-GCM protocol
type AESGCMNonce [AESGCMNonceSize]byte

// Bytes returns the nonce as a byte array pointer
func (nonce *AESGCMNonce) Bytes() *[AESGCMNonceSize]byte {
	return (*[AESGCMNonceSize]byte)(nonce)
}

// Encrypt encrypts byte data with a key and nonce. The cipher data is returned
// The type of encryption to use can be modified with encType
func (cipher Cipher) Encrypt(data []byte, key *GenericKey, nonce *GenericNonce) (cipherData []byte, err error) {
	switch cipher {
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
func (cipher Cipher) Decrypt(cipherData []byte, key *GenericKey, nonce *GenericNonce) (data []byte, err error) {
	switch cipher {
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
func (cipher Cipher) NewEncrypter(key *GenericKey, startingNonce *GenericNonce, encBlockSize int) (Transformer, error) {
	switch cipher {
	case None:
		return &NoopTransformer{}, nil
	case AESGCM:
		return NewAESGCMEncrypter(key, startingNonce.AESGCMNonce(), encBlockSize)
	case SecretBox:
		return NewSecretboxEncrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

// NewDecrypter creates transform stream using a key and a nonce to decrypt data passing through it
// The type of encryption to use can be modified with encType
func (cipher Cipher) NewDecrypter(key *GenericKey, startingNonce *GenericNonce, encBlockSize int) (Transformer, error) {
	switch cipher {
	case None:
		return &NoopTransformer{}, nil
	case AESGCM:
		return NewAESGCMDecrypter(key, startingNonce.AESGCMNonce(), encBlockSize)
	case SecretBox:
		return NewSecretboxDecrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}
