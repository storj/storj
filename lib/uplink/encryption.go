// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/storj/pkg/storj"
)

type Cipher byte

const (
	UnsetCipher = Cipher(iota)
	Unencrypted
	AESGCM
	SecretBox
)

const (
	defaultCipher = SecretBox
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
