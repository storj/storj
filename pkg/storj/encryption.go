// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

// EncryptionScheme is the scheme and parameters used for encryption
type EncryptionScheme struct {
	Cipher Cipher
}

// Cipher specifies an encryption algorithm
type Cipher byte

// List of supported encryption algorithms
const (
	Unencrypted = Cipher(iota)
	AESGCM
	SecretBox
)

// Constant definitions for key and nonce sizes
const (
	KeySize   = 32
	NonceSize = 24
)

// Key represents the largest key used by any encryption protocol
type Key [KeySize]byte

// Raw returns the key as a raw byte array pointer
func (key *Key) Raw() *[KeySize]byte {
	return (*[KeySize]byte)(key)
}

// Nonce represents the largest nonce used by any encryption protocol
type Nonce [NonceSize]byte

// Raw returns the nonce as a raw byte array pointer
func (nonce *Nonce) Raw() *[NonceSize]byte {
	return (*[NonceSize]byte)(nonce)
}

// EncryptedPrivateKey is a private key that has been encrypted
type EncryptedPrivateKey []byte
