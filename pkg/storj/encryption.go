// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

// EncryptionScheme is the scheme and parameters used for encryption
type EncryptionScheme struct {
	Algorithm    EncryptionAlgorithm
	EncryptedKey EncryptedPrivateKey
}

// EncryptionAlgorithm specifies an encryption algorithm
type EncryptionAlgorithm byte

// List of supported encryption algorithms
const (
	InvalidEncryptionAlgorithm = EncryptionAlgorithm(iota)
	Unencrypted
	AESGCM
	Secretbox
)

// EncryptedPrivateKey is a private key that has been encrypted
type EncryptedPrivateKey []byte

// EncryptedNonce is a nonce that has been encrypted
type EncryptedNonce []byte
