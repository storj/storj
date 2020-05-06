// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
)

const (
	defaultCipher = storj.EncAESGCM
)

// EncryptionAccess represents an encryption access context. It holds
// information about how various buckets and objects should be
// encrypted and decrypted.
type EncryptionAccess struct {
	store *encryption.Store
}

// NewEncryptionAccess creates an encryption access context
func NewEncryptionAccess() *EncryptionAccess {
	store := encryption.NewStore()
	return &EncryptionAccess{store: store}
}

// NewEncryptionAccessWithDefaultKey creates an encryption access context with
// a default key set.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key
func NewEncryptionAccessWithDefaultKey(defaultKey storj.Key) *EncryptionAccess {
	ec := NewEncryptionAccess()
	ec.SetDefaultKey(defaultKey)
	return ec
}

// validate returns an error if the EncryptionAccess is not valid to be used.
func (s *EncryptionAccess) validate() error {
	if s == nil {
		return errs.New("invalid nil encryption access")
	}
	if s.store == nil {
		return errs.New("invalid encryption access: no store")
	}
	return nil
}

// Store returns the underlying encryption store for the access context.
func (s *EncryptionAccess) Store() *encryption.Store {
	return s.store
}

// SetDefaultKey sets the default key for the encryption access context.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key
func (s *EncryptionAccess) SetDefaultKey(defaultKey storj.Key) {
	s.store.SetDefaultKey(&defaultKey)
}

// SetDefaultPathCipher sets the default path cipher for the encryption access context.
func (s *EncryptionAccess) SetDefaultPathCipher(defaultPathCipher storj.CipherSuite) {
	s.store.SetDefaultPathCipher(defaultPathCipher)
}

// Import merges the other encryption access context into this one. In cases
// of conflicting path decryption settings (including if both accesses have
// a default key), the new settings are kept.
func (s *EncryptionAccess) Import(other *EncryptionAccess) error {
	if key := other.store.GetDefaultKey(); key != nil {
		s.store.SetDefaultKey(key)
	}
	s.store.SetDefaultPathCipher(other.store.GetDefaultPathCipher())
	return other.store.Iterate(s.store.Add)
}

// EncryptionRestriction represents a scenario where some set of objects
// may need to be encrypted/decrypted
type EncryptionRestriction struct {
	Bucket     string
	PathPrefix storj.Path
}

// Restrict creates a new EncryptionAccess with no default key, where the key material
// in the new access is just enough to allow someone to access all of the given
// restrictions but no more.
func (s *EncryptionAccess) Restrict(apiKey APIKey, restrictions ...EncryptionRestriction) (APIKey, *EncryptionAccess, error) {
	if len(restrictions) == 0 {
		// Should the function signature be
		// func (s *EncryptionAccess) Restrict(apiKey APIKey, restriction EncryptionRestriction, restrictions ...EncryptionRestriction) (APIKey, *EncryptionAccess, error) {
		// so we don't have to do this test?
		return APIKey{}, nil, errs.New("at least one restriction required")
	}

	caveat := macaroon.Caveat{}

	access := NewEncryptionAccess()
	access.SetDefaultPathCipher(s.store.GetDefaultPathCipher())
	if len(restrictions) == 0 {
		access.Store().SetDefaultKey(s.store.GetDefaultKey())
	}

	for _, res := range restrictions {
		// If the share prefix ends in a `/` we need to remove this final slash.
		// Otherwise, if we the shared prefix is `/bob/`, the encrypted shared
		// prefix results in `enc("")/enc("bob")/enc("")`. This is an incorrect
		// encrypted prefix, what we really want is `enc("")/enc("bob")`.
		unencPath := paths.NewUnencrypted(strings.TrimSuffix(res.PathPrefix, "/"))

		encPath, err := encryption.EncryptPathWithStoreCipher(res.Bucket, unencPath, s.store)
		if err != nil {
			return APIKey{}, nil, err
		}
		derivedKey, err := encryption.DerivePathKey(res.Bucket, unencPath, s.store)
		if err != nil {
			return APIKey{}, nil, err
		}

		if err := access.store.Add(res.Bucket, unencPath, encPath, *derivedKey); err != nil {
			return APIKey{}, nil, err
		}
		caveat.AllowedPaths = append(caveat.AllowedPaths, &macaroon.Caveat_Path{
			Bucket:              []byte(res.Bucket),
			EncryptedPathPrefix: []byte(encPath.Raw()),
		})
	}

	restrictedAPIKey, err := apiKey.Restrict(caveat)
	if err != nil {
		return APIKey{}, nil, err
	}

	return restrictedAPIKey, access, nil
}

// Serialize turns an EncryptionAccess into base58
func (s *EncryptionAccess) Serialize() (string, error) {
	p, err := s.toProto()
	if err != nil {
		return "", err
	}

	data, err := pb.Marshal(p)
	if err != nil {
		return "", errs.New("unable to marshal encryption access: %v", err)
	}

	return base58.CheckEncode(data, 0), nil
}

func (s *EncryptionAccess) toProto() (*pb.EncryptionAccess, error) {
	var storeEntries []*pb.EncryptionAccess_StoreEntry
	err := s.store.IterateWithCipher(func(bucket string, unenc paths.Unencrypted, enc paths.Encrypted, key storj.Key, pathCipher storj.CipherSuite) error {
		storeEntries = append(storeEntries, &pb.EncryptionAccess_StoreEntry{
			Bucket:          []byte(bucket),
			UnencryptedPath: []byte(unenc.Raw()),
			EncryptedPath:   []byte(enc.Raw()),
			Key:             key[:],
			PathCipher:      pb.CipherSuite(pathCipher),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	var defaultKey []byte
	if key := s.store.GetDefaultKey(); key != nil {
		defaultKey = key[:]
	}

	return &pb.EncryptionAccess{
		DefaultKey:        defaultKey,
		StoreEntries:      storeEntries,
		DefaultPathCipher: pb.CipherSuite(s.store.GetDefaultPathCipher()),
	}, nil
}

// ParseEncryptionAccess parses a base58 serialized encryption access into a working one.
func ParseEncryptionAccess(serialized string) (*EncryptionAccess, error) {
	data, version, err := base58.CheckDecode(serialized)
	if err != nil || version != 0 {
		return nil, errs.New("invalid encryption access format")
	}

	p := new(pb.EncryptionAccess)
	if err := pb.Unmarshal(data, p); err != nil {
		return nil, errs.New("unable to unmarshal encryption access: %v", err)
	}

	return parseEncryptionAccessFromProto(p)
}

func parseEncryptionAccessFromProto(p *pb.EncryptionAccess) (*EncryptionAccess, error) {
	access := NewEncryptionAccess()
	if len(p.DefaultKey) > 0 {
		if len(p.DefaultKey) != len(storj.Key{}) {
			return nil, errs.New("invalid default key in encryption access")
		}
		var defaultKey storj.Key
		copy(defaultKey[:], p.DefaultKey)
		access.SetDefaultKey(defaultKey)
	}

	access.SetDefaultPathCipher(storj.CipherSuite(p.DefaultPathCipher))
	if p.DefaultPathCipher == pb.CipherSuite_ENC_UNSPECIFIED {
		access.SetDefaultPathCipher(storj.EncAESGCM)
	}

	for _, entry := range p.StoreEntries {
		if len(entry.Key) != len(storj.Key{}) {
			return nil, errs.New("invalid key in encryption access entry")
		}
		var key storj.Key
		copy(key[:], entry.Key)

		err := access.store.AddWithCipher(
			string(entry.Bucket),
			paths.NewUnencrypted(string(entry.UnencryptedPath)),
			paths.NewEncrypted(string(entry.EncryptedPath)),
			key,
			storj.CipherSuite(entry.PathCipher),
		)
		if err != nil {
			return nil, errs.New("invalid encryption access entry: %v", err)
		}
	}

	return access, nil
}
