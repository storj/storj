// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// EncryptionCtx holds data about encryption keys for a bucket.
type EncryptionCtx struct {
	lib *libuplink.EncryptionCtx
}

// NewEncryptionCtx constructs an empty encryption context.
func NewEncryptionCtx() *EncryptionCtx {
	return &EncryptionCtx{lib: libuplink.NewEncryptionCtx()}
}

// SetDefaultKey sets the default key to use when no matching keys are found
// for the encryption context.
func (e *EncryptionCtx) SetDefaultKey(keyData []byte) error {
	key, err := storj.NewKey(keyData)
	if err != nil {
		return safeError(err)
	}
	e.lib.SetDefaultKey(*key)
	return nil
}

// ParseEncryptionCtx parses the base58 encoded encryption context data and
// returns the resulting context.
func ParseEncryptionCtx(b58data string) (*EncryptionCtx, error) {
	encCtx, err := libuplink.ParseEncryptionCtx(b58data)
	if err != nil {
		return nil, safeError(err)
	}
	return &EncryptionCtx{lib: encCtx}, nil
}
