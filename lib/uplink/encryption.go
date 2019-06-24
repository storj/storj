// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/macaroon"
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
// a default key), the new settings are kept.
func (s *EncryptionCtx) Import(other *EncryptionCtx) error {
	if key := other.store.GetDefaultKey(); key != nil {
		s.store.SetDefaultKey(key)
	}
	return other.store.Iterate(s.store.Add)
}

// EncryptionRestriction represents a scenario where some set of objects
// may need to be encrypted/decrypted
type EncryptionRestriction struct {
	Bucket     string
	PathPrefix storj.Path
}

// Restrict creates a new EncryptionCtx with no default key, where the key material
// in the new context is just enough to allow someone to access all of the given
// restrictions but no more.
func (s *EncryptionCtx) Restrict(apiKey APIKey, restrictions ...EncryptionRestriction) (APIKey, *EncryptionCtx, error) {
	if len(restrictions) == 0 {
		// Should the function signature be
		// func (s *EncryptionCtx) Restrict(apiKey APIKey, restriction EncryptionRestriction, restrictions ...EncryptionRestriction) (APIKey, *EncryptionCtx, error) {
		// so we don't have to do this test?
		return APIKey{}, nil, errs.New("at least one restriction required")
	}

	caveat := macaroon.Caveat{}
	encCtx := NewEncryptionCtx()

	for _, res := range restrictions {
		unencPath := paths.NewUnencrypted(res.PathPrefix)
		cipher := storj.AESGCM // TODO(jeff): pick the right path cipher

		encPath, err := encryption.StoreEncryptPath(res.Bucket, unencPath, cipher, s.store)
		if err != nil {
			return APIKey{}, nil, err
		}
		derivedKey, err := encryption.StoreDerivePathKey(res.Bucket, unencPath, s.store)
		if err != nil {
			return APIKey{}, nil, err
		}

		if err := encCtx.store.Add(res.Bucket, unencPath, encPath, *derivedKey); err != nil {
			return APIKey{}, nil, err
		}
		caveat.AllowedPaths = append(caveat.AllowedPaths, &macaroon.Caveat_Path{
			Bucket:              res.Bucket,
			EncryptedPathPrefix: []byte(encPath.Raw()),
		})
	}

	apiKey, err := apiKey.Restrict(caveat)
	if err != nil {
		return APIKey{}, nil, err
	}

	return apiKey, encCtx, nil
}

// Serialize turns an EncryptionCtx into base58
func (s *EncryptionCtx) Serialize() ([]byte, error) {
	var storeEntries []*pb.EncryptionCtx_StoreEntry
	err := s.store.Iterate(func(bucket string, unenc, enc storj.Path, key storj.Key) error {
		storeEntries = append(storeEntries, &pb.EncryptionCtx_StoreEntry{
			Bucket:          []byte(bucket),
			UnencryptedPath: []byte(unenc.Raw()),
			EncryptedPath:   []byte(enc.Raw()),
			Key:             key[:],
		})
		return nil
	})
	var defaultKey []byte
	if key := s.store.GetDefaultKey(); key != nil {
		defaultKey = key[:]
	}

	data, err := proto.Marshal(&pb.Scope{
		DefaultKey:   defaultKey,
		StoreEntries: storeEntries,
	})
	if err != nil {
		return "", errs.New("unable to marshal encryption ctx: %v", err)
	}

	return base58.CheckEncode(data, 0), nil

}

// ParseEncryptionCtx parses a base58 serialized encryption context into a working one.
func ParseEncryptionCtx(data []byte) (*EncryptionCtx, error) {
	// TODO(jeff): should parse operate on a string?
	data, version, err := base58.CheckDecode(string(data))
	if err != nil || version != 0 {
		return nil, errs.New("invalid encryption context format")
	}

	p := new(pb.EncryptionCtx)
	if err := proto.Unmarshal(data, p); err != nil {
		return nil, errs.New("unable to unmarshal encryption context: %v", err)
	}

	encCtx := NewEncryptionCtx()

	if len(p.DefaultKey) > 0 {
		if len(p.DefaultKey) != len(storj.Key{}) {
			return nil, errs.New("invalid default key in encryption context")
		}
		var defaultKey storj.Key
		copy(defaultKey[:], p.DefaultKey)
		encCtx.SetDefaultKey(defaultKey)
	}

	for _, entry := range p.StoreEntries {
		if len(entry.Key) != len(storj.Key{}) {
			return nil, errs.New("invalid key in encryption context entry")
		}
		var key storj.Key
		copy(key[:], entry.Key)

		err := encCtx.store.Add(
			string(entry.Bucket),
			paths.NewUnencrypted(entry.UnencryptedPath),
			paths.NewEncrypted(entry.EncryptedPath),
			key)
		if err != nil {
			return nil, errs.New("invalid encryption context entry: %v", err)
		}
	}

	return encCtx, nil
}
