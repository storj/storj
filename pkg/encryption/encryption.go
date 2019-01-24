// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/hmac"
	"crypto/sha512"

	"storj.io/storj/pkg/storj"
)

// AESGCMNonceSize is the size of an AES-GCM nonce
const AESGCMNonceSize = 12

// AESGCMNonce represents the nonce used by the AES-GCM protocol
type AESGCMNonce [AESGCMNonceSize]byte

// ToAESGCMNonce returns the nonce as a AES-GCM nonce
func ToAESGCMNonce(nonce *storj.Nonce) *AESGCMNonce {
	aes := new(AESGCMNonce)
	copy((*aes)[:], nonce[:AESGCMNonceSize])
	return aes
}

// Increment increments the nonce with the given amount
func Increment(nonce *storj.Nonce, amount int64) (truncated bool, err error) {
	return incrementBytes(nonce[:], amount)
}

// Encrypt encrypts data with the given cipher, key and nonce
func Encrypt(data []byte, cipher storj.Cipher, key *storj.Key, nonce *storj.Nonce) (cipherData []byte, err error) {
	// Don't encrypt empty slice
	if len(data) == 0 {
		return []byte{}, nil
	}

	switch cipher {
	case storj.Unencrypted:
		return data, nil
	case storj.AESGCM:
		return EncryptAESGCM(data, key, ToAESGCMNonce(nonce))
	case storj.SecretBox:
		return EncryptSecretBox(data, key, nonce)
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// Decrypt decrypts cipherData with the given cipher, key and nonce
func Decrypt(cipherData []byte, cipher storj.Cipher, key *storj.Key, nonce *storj.Nonce) (data []byte, err error) {
	// Don't decrypt empty slice
	if len(cipherData) == 0 {
		return []byte{}, nil
	}

	switch cipher {
	case storj.Unencrypted:
		return cipherData, nil
	case storj.AESGCM:
		return DecryptAESGCM(cipherData, key, ToAESGCMNonce(nonce))
	case storj.SecretBox:
		return DecryptSecretBox(cipherData, key, nonce)
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// NewEncrypter creates a Transformer using the given cipher, key and nonce to encrypt data passing through it
func NewEncrypter(cipher storj.Cipher, key *storj.Key, startingNonce *storj.Nonce, encryptedBlockSize int) (Transformer, error) {
	switch cipher {
	case storj.Unencrypted:
		return &NoopTransformer{}, nil
	case storj.AESGCM:
		return NewAESGCMEncrypter(key, ToAESGCMNonce(startingNonce), encryptedBlockSize)
	case storj.SecretBox:
		return NewSecretboxEncrypter(key, startingNonce, encryptedBlockSize)
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// NewDecrypter creates a Transformer using the given cipher, key and nonce to decrypt data passing through it
func NewDecrypter(cipher storj.Cipher, key *storj.Key, startingNonce *storj.Nonce, encryptedBlockSize int) (Transformer, error) {
	switch cipher {
	case storj.Unencrypted:
		return &NoopTransformer{}, nil
	case storj.AESGCM:
		return NewAESGCMDecrypter(key, ToAESGCMNonce(startingNonce), encryptedBlockSize)
	case storj.SecretBox:
		return NewSecretboxDecrypter(key, startingNonce, encryptedBlockSize)
	default:
		return nil, ErrInvalidConfig.New("encryption type %d is not supported", cipher)
	}
}

// EncryptKey encrypts keyToEncrypt with the given cipher, key and nonce
func EncryptKey(keyToEncrypt *storj.Key, cipher storj.Cipher, key *storj.Key, nonce *storj.Nonce) (storj.EncryptedPrivateKey, error) {
	return Encrypt(keyToEncrypt[:], cipher, key, nonce)
}

// DecryptKey decrypts keyToDecrypt with the given cipher, key and nonce
func DecryptKey(keyToDecrypt storj.EncryptedPrivateKey, cipher storj.Cipher, key *storj.Key, nonce *storj.Nonce) (*storj.Key, error) {
	plainData, err := Decrypt(keyToDecrypt, cipher, key, nonce)
	if err != nil {
		return nil, err
	}

	var decryptedKey storj.Key
	copy(decryptedKey[:], plainData)

	return &decryptedKey, nil
}

// DeriveKey derives new key from the given key and message using HMAC-SHA512
func DeriveKey(key *storj.Key, message string) (*storj.Key, error) {
	mac := hmac.New(sha512.New, key[:])
	_, err := mac.Write([]byte(message))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	derived := new(storj.Key)
	copy(derived[:], mac.Sum(nil))

	return derived, nil
}
