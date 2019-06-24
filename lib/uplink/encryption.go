// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storj"
)

const (
	defaultCipher = storj.EncAESGCM
)

// EncryptionCtx represents an encryption context. It holds information about
// how various buckets and objects should be encrypted and decrypted.
type EncryptionCtx struct {
	store *encryption.Store
}

// NewEncryptionCtx creates an encryption ctx
func NewEncryptionCtx() *EncryptionCtx {
	return &EncryptionCtx{
		store: encryption.NewStore(),
	}
}

// NewEncryptionCtx creates an encryption ctx with a default key set
func NewEncryptionCtxWithDefaultKey(defaultKey storj.Key) *EncryptionCtx {
	ec := NewEncryptionCtx()
	ec.SetDefaultKey(defaultKey)
	return ec
}

// SetDefaultKey sets the default key for the encryption context.
func (s *EncryptionCtx) SetDefaultKey(defaultKey storj.Key) {
	s.store.SetDefaultKey(&defaultKey)
}

// Import merges the other encryption context into this one. In cases
// of conflicting path decryption settings (including if both contexts have
// a default key), the existing settings are kept.
func (s *EncryptionCtx) Import(other *EncryptionCtx) error {
	panic("TODO")
}

// EncryptionRestriction represents a scenario where some set of objects
// may need to be encrypted/decrypted
type EncryptionRestriction struct {
	Bucket                string
	UnencryptedPathPrefix storj.Path
}

// Export creates a new EncryptionCtx with no default key, where the key material
// in the new context is just enough to allow someone to access all of the given
// restrictions but no more.
func (s *EncryptionCtx) Export(restrictions ...EncryptionRestriction) (*EncryptionCtx, error) {
	panic("TODO")
}

// Serialize turns an EncryptionCtx into base58
func (s *EncryptionCtx) Serialize() ([]byte, error) {
	panic("TODO")
}

// ParseEncryptionCtx parses a base58 serialized encryption context into a working one.
func ParseEncryptionCtx(data []byte) (*EncryptionCtx, error) {
	panic("TODO")
}
