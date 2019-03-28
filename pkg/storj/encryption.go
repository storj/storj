// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

// EncryptionScheme is the scheme and parameters used for encryption
type EncryptionScheme struct {
	// Cipher specifies the ciphersuite to be used for encryption.
	Cipher Cipher
	// BlockSize determines the unit size at which encryption is performed.
	// It is important to distinguish this from the block size used by the
	// ciphersuite (probably 128 bits). There is some small overhead for
	// each encryption unit, so BlockSize should not be too small, but
	// smaller sizes yield shorter first-byte latency and better seek times.
	// Note that BlockSize itself is the size of data blocks _after_ they
	// have been encrypted and the authentication overhead has been added.
	// It is _not_ the size of the data blocks to _be_ encrypted.
	BlockSize int32
}

// IsZero returns true if no field in the struct is set to non-zero value
func (scheme EncryptionScheme) IsZero() bool {
	return scheme == (EncryptionScheme{})
}

// Cipher specifies an encryption algorithm
type Cipher byte

// List of supported encryption algorithms
const (
	// Unencrypted indicates no encryption or decryption is to be performed.
	Unencrypted = Cipher(iota)
	// AESGCM indicates use of AES128-GCM encryption.
	AESGCM
	// SecretBox indicates use of XSalsa20-Poly1305 encryption, as provided by
	// the NaCl cryptography library under the name "Secretbox".
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
