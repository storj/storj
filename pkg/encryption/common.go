// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/eestream"
)

const (
	None = iota
	AESGCM
	SecretBox
)

// Error is the default encryption errs class
var Error = errs.Class("encryption error")

func Encrypt(data []byte, key *[32]byte, nonce *[24]byte, encType int) (cipherData []byte, err error) {
	switch encType {
	case None:
		return data, nil
	case AESGCM:
		return EncryptAESGCM(data, key[:], nonce[12:])
	case SecretBox:

		return EncryptSecretBox(data, key, nonce)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

func Decrypt(cipherData []byte, key *[32]byte, nonce *[24]byte, encType int) (data []byte, err error) {
	switch encType {
	case None:
		return cipherData, nil
	case AESGCM:
		return DecryptAESGCM(data, key[:], nonce[12:])
	case SecretBox:
		return DecryptSecretBox(data, key, nonce)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

func NewEncrypter(key *[32]byte, startingNonce *[24]byte,
	encBlockSize, encType int) (eestream.Transformer, error) {
	switch encType {
	case None:
		return &eestream.NoopTransformer{}, nil
	case AESGCM:
		var nonce *[12]byte
		copy((*nonce)[:], (*startingNonce)[12:])
		return NewAESGCMEncrypter(key, nonce, encBlockSize)
	case SecretBox:
		return NewSecretboxEncrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}

func NewDecrypter(key *[32]byte, startingNonce *[24]byte,
	encBlockSize, encType int) (eestream.Transformer, error) {
	switch encType {
	case None:
		return &eestream.NoopTransformer{}, nil
	case AESGCM:
		var nonce *[12]byte
		copy((*nonce)[:], (*startingNonce)[12:])
		return NewAESGCMEncrypter(key, nonce, encBlockSize)
	case SecretBox:
		return NewSecretboxEncrypter(key, startingNonce, encBlockSize)
	default:
		return nil, errs.New("Invalid encryption type")
	}
}
