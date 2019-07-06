// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

// EncryptionParameters is the cipher suite and parameters used for encryption
type EncryptionParameters struct {
	// CipherSuite specifies the cipher suite to be used for encryption.
	CipherSuite CipherSuite
	// BlockSize determines the unit size at which encryption is performed.
	// It is important to distinguish this from the block size used by the
	// cipher suite (probably 128 bits). There is some small overhead for
	// each encryption unit, so BlockSize should not be too small, but
	// smaller sizes yield shorter first-byte latency and better seek times.
	// Note that BlockSize itself is the size of data blocks _after_ they
	// have been encrypted and the authentication overhead has been added.
	// It is _not_ the size of the data blocks to _be_ encrypted.
	BlockSize int32
}

// IsZero returns true if no field in the struct is set to non-zero value
func (params EncryptionParameters) IsZero() bool {
	return params == (EncryptionParameters{})
}

// CipherSuite specifies one of the encryption suites supported by Storj
// libraries for encryption of in-network data.
type CipherSuite byte

const (
	// EncUnspecified indicates no encryption suite has been selected.
	EncUnspecified = CipherSuite(iota)
	// EncNull indicates use of the NULL cipher; that is, no encryption is
	// done. The ciphertext is equal to the plaintext.
	EncNull
	// EncAESGCM indicates use of AES128-GCM encryption.
	EncAESGCM
	// EncSecretBox indicates use of XSalsa20-Poly1305 encryption, as provided
	// by the NaCl cryptography library under the name "Secretbox".
	EncSecretBox
)

// Constant definitions for key and nonce sizes
const (
	KeySize   = 32
	NonceSize = 24
)

// NewKey creates a new Storj key from humanReadableKey.
func NewKey(humanReadableKey []byte) (*Key, error) {
	var key Key

	// Because of backward compatibility the key is filled with 0 or truncated if
	// humanReadableKey isn't of the same size that KeySize.
	// See https://github.com/storj/storj/pull/1967#discussion_r285544849
	copy(key[:], humanReadableKey)
	return &key, nil
}

// Key represents the largest key used by any encryption protocol
type Key [KeySize]byte

// Raw returns the key as a raw byte array pointer
func (key *Key) Raw() *[KeySize]byte {
	return (*[KeySize]byte)(key)
}

// IsZero returns true if key is nil or it points to its zero value
func (key *Key) IsZero() bool {
	return key == nil || *key == (Key{})
}

// Nonce represents the largest nonce used by any encryption protocol
type Nonce [NonceSize]byte

// Raw returns the nonce as a raw byte array pointer
func (nonce *Nonce) Raw() *[NonceSize]byte {
	return (*[NonceSize]byte)(nonce)
}

// EncryptedPrivateKey is a private key that has been encrypted
type EncryptedPrivateKey []byte
