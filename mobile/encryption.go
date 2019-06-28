// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// EncryptionAccess holds data about encryption keys for a bucket.
type EncryptionAccess struct {
	lib *libuplink.EncryptionAccess
}

// NewEncryptionAccess constructs an empty encryption context.
func NewEncryptionAccess() *EncryptionAccess {
	return &EncryptionAccess{lib: libuplink.NewEncryptionAccess()}
}

// SetDefaultKey sets the default key to use when no matching keys are found
// for the encryption context.
func (e *EncryptionAccess) SetDefaultKey(keyData []byte) error {
	key, err := storj.NewKey(keyData)
	if err != nil {
		return safeError(err)
	}
	e.lib.SetDefaultKey(*key)
	return nil
}

// ParseEncryptionAccess parses the base58 encoded encryption context data and
// returns the resulting context.
func ParseEncryptionAccess(b58data string) (*EncryptionAccess, error) {
	access, err := libuplink.ParseEncryptionAccess(b58data)
	if err != nil {
		return nil, safeError(err)
	}
	return &EncryptionAccess{lib: access}, nil
}
