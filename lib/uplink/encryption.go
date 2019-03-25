// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/storj/pkg/storj"
)

// Cipher represents the type of encryption employed in a path or an object.
type Cipher byte

const (
	// UnsetCipher indicates the cipher type has not been set explicitly.
	UnsetCipher = Cipher(iota)
	// Unencrypted indicates no encryption or decryption is to be performed.
	Unencrypted
	// AESGCM indicates use of AES128-GCM encryption.
	AESGCM
	// SecretBox indicates use of XSalsa20-Poly1305 encryption, as provided by
	// the NaCl cryptography library under the name "Secretbox".
	SecretBox
)

const (
	defaultCipher = AESGCM
)

// EncryptionAccess specifies the encryption details needed to encrypt/decrypt objects
type EncryptionAccess struct {
	Key                 storj.Key
	EncryptedPathPrefix storj.Path
}

func (c Cipher) convert() (storj.Cipher, error) {
	switch c {
	case Unencrypted:
		return storj.Unencrypted, nil
	case AESGCM:
		return storj.AESGCM, nil
	case SecretBox:
		return storj.SecretBox, nil
	default:
		return 0, Error.New("unknown cipher")
	}
}
