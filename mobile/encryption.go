// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/paths"
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

// NewEncryptionAccessWithRoot constructs an encryption access with a key rooted at the provided path inside of a bucket.
func NewEncryptionAccessWithRoot(bucket, unencryptedPath, encryptedPath string, keyData []byte) (*EncryptionAccess, error) {
	key, err := storj.NewKey(keyData)
	if err != nil {
		return nil, safeError(err)
	}
	encAccess := libuplink.NewEncryptionAccess()
	err = encAccess.Store().Add(bucket, paths.NewUnencrypted(unencryptedPath), paths.NewEncrypted(encryptedPath), *key)
	if err != nil {
		return nil, safeError(err)
	}
	return &EncryptionAccess{lib: encAccess}, nil
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

// Serialize returns a base58-serialized encryption access for use with later
// parsing.
func (e *EncryptionAccess) Serialize() (b58data string, err error) {
	return e.lib.Serialize()
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

// NewEncryptionAccessWithDefaultKey creates an encryption access context with
// a default key set.
// Use Project.SaltedKeyFromPassphrase to generate a default key
func NewEncryptionAccessWithDefaultKey(defaultKey []byte) (_ *EncryptionAccess, err error) {
	key, err := storj.NewKey(defaultKey)
	if err != nil {
		return nil, err
	}
	return &EncryptionAccess{lib: libuplink.NewEncryptionAccessWithDefaultKey(*key)}, nil
}
