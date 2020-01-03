// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"storj.io/common/paths"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
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

// Restrict creates a new EncryptionAccess with no default key, where the key material
// in the new access is just enough to allow someone to access all of the given
// restrictions but no more.
func (e *EncryptionAccess) Restrict(satelliteAddr string, apiKey *APIKey, restrictions *EncryptionRestrictions) (_ *Scope, err error) {
	libAPIKey, ea, err := e.lib.Restrict(*apiKey.lib, restrictions.restrictions...)
	return &Scope{
		lib: &libuplink.Scope{
			SatelliteAddr:    satelliteAddr,
			APIKey:           libAPIKey,
			EncryptionAccess: ea,
		},
	}, err
}

// Import merges the other encryption access context into this one. In cases
// of conflicting path decryption settings (including if both accesses have
// a default key), the new settings are kept.
func (e *EncryptionAccess) Import(other *EncryptionAccess) error {
	return e.lib.Import(other.lib)
}

// EncryptionRestriction represents a scenario where some set of objects
// may need to be encrypted/decrypted
type EncryptionRestriction struct {
	lib *libuplink.EncryptionRestriction
}

// NewEncryptionRestriction creates new EncryptionRestriction
func NewEncryptionRestriction(bucket, path string) *EncryptionRestriction {
	return &EncryptionRestriction{
		lib: &libuplink.EncryptionRestriction{
			Bucket:     bucket,
			PathPrefix: path,
		},
	}
}

// EncryptionRestrictions combines EncryptionRestriction to overcome gomobile limitation (no arrays)
type EncryptionRestrictions struct {
	restrictions []libuplink.EncryptionRestriction
}

// NewEncryptionRestrictions creates new EncryptionRestrictions
func NewEncryptionRestrictions() *EncryptionRestrictions {
	return &EncryptionRestrictions{
		restrictions: make([]libuplink.EncryptionRestriction, 0),
	}
}

// Add adds EncryptionRestriction
func (e *EncryptionRestrictions) Add(restriction *EncryptionRestriction) {
	e.restrictions = append(e.restrictions, *restriction.lib)
}
