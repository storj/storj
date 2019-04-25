// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

// EncryptionScheme is the scheme and parameters used for encryption.
// Use the similar EncryptionParameters struct instead, if possible.
type EncryptionScheme struct {
	// Cipher specifies the cipher suite to be used for encryption.
	Cipher Cipher
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
func (scheme EncryptionScheme) IsZero() bool {
	return scheme == (EncryptionScheme{})
}

// ToEncryptionParameters transforms an EncryptionScheme object into the
// equivalent EncryptionParameters object.
func (scheme EncryptionScheme) ToEncryptionParameters() EncryptionParameters {
	return EncryptionParameters{
		CipherSuite: scheme.Cipher.ToCipherSuite(),
		BlockSize:   scheme.BlockSize,
	}
}

// EncryptionParameters is the cipher suite and parameters used for encryption
// It is like EncryptionScheme, but uses the CipherSuite type instead of Cipher.
// EncryptionParameters is preferred for new uses.
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

// ToEncryptionScheme transforms an EncryptionParameters object into the
// equivalent EncryptionScheme object.
func (params EncryptionParameters) ToEncryptionScheme() EncryptionScheme {
	return EncryptionScheme{
		Cipher:    params.CipherSuite.ToCipher(),
		BlockSize: params.BlockSize,
	}
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
	// Invalid indicates a Cipher value whose use is not valid. This may be
	// used as a replacement for "unspecified" in a pinch, although it is not
	// the zero value.
	Invalid
)

// ToCipherSuite converts a Cipher value to a CipherSuite value.
func (c Cipher) ToCipherSuite() CipherSuite {
	switch c {
	case Unencrypted:
		return EncNull
	case AESGCM:
		return EncAESGCM
	case SecretBox:
		return EncSecretBox
	}
	return EncUnspecified
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

// ToCipher converts a CipherSuite value to a Cipher value.
func (cs CipherSuite) ToCipher() Cipher {
	switch cs {
	case EncNull:
		return Unencrypted
	case EncAESGCM:
		return AESGCM
	case EncSecretBox:
		return SecretBox
	}
	return Invalid
}

// Constant definitions for key and nonce sizes
const (
	KeySize   = 32
	NonceSize = 24
)

// NewKey creates a new key from a passphrase
func NewKey(passphrase []byte) *Key {
	key := &Key{}
	copy(key[:], passphrase)
	return key
}

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
