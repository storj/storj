// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/gogo/protobuf/proto"
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
	return &EncryptionAccess{
		store: encryption.NewStore(),
	}
}

// NewEncryptionAccessWithDefaultKey creates an encryption access context with
// a default key set.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key
func NewEncryptionAccessWithDefaultKey(defaultKey storj.Key) *EncryptionAccess {
	ec := NewEncryptionAccess()
	ec.SetDefaultKey(defaultKey)
	return ec
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

// Import merges the other encryption access context into this one. In cases
// of conflicting path decryption settings (including if both accesses have
// a default key), the new settings are kept.
func (s *EncryptionAccess) Import(other *EncryptionAccess) error {
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

	for _, res := range restrictions {
		unencPath := paths.NewUnencrypted(res.PathPrefix)
		cipher := storj.EncAESGCM // TODO(jeff): pick the right path cipher

		encPath, err := encryption.EncryptPath(res.Bucket, unencPath, cipher, s.store)
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

	apiKey, err := apiKey.Restrict(caveat)
	if err != nil {
		return APIKey{}, nil, err
	}

	return apiKey, access, nil
}

// Serialize turns an EncryptionAccess into base58
func (s *EncryptionAccess) Serialize() (string, error) {
	p, err := s.toProto()
	if err != nil {
		return "", err
	}

	data, err := proto.Marshal(p)
	if err != nil {
		return "", errs.New("unable to marshal encryption access: %v", err)
	}

	return base58.CheckEncode(data, 0), nil
}

func (s *EncryptionAccess) toProto() (*pb.EncryptionAccess, error) {
	var storeEntries []*pb.EncryptionAccess_StoreEntry
	err := s.store.Iterate(func(bucket string, unenc paths.Unencrypted, enc paths.Encrypted, key storj.Key) error {
		storeEntries = append(storeEntries, &pb.EncryptionAccess_StoreEntry{
			Bucket:          []byte(bucket),
			UnencryptedPath: []byte(unenc.Raw()),
			EncryptedPath:   []byte(enc.Raw()),
			Key:             key[:],
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
		DefaultKey:   defaultKey,
		StoreEntries: storeEntries,
	}, nil
}

// ParseEncryptionAccess parses a base58 serialized encryption access into a working one.
func ParseEncryptionAccess(serialized string) (*EncryptionAccess, error) {
	data, version, err := base58.CheckDecode(serialized)
	if err != nil || version != 0 {
		return nil, errs.New("invalid encryption access format")
	}

	p := new(pb.EncryptionAccess)
	if err := proto.Unmarshal(data, p); err != nil {
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

	for _, entry := range p.StoreEntries {
		if len(entry.Key) != len(storj.Key{}) {
			return nil, errs.New("invalid key in encryption access entry")
		}
		var key storj.Key
		copy(key[:], entry.Key)

		err := access.store.Add(
			string(entry.Bucket),
			paths.NewUnencrypted(string(entry.UnencryptedPath)),
			paths.NewEncrypted(string(entry.EncryptedPath)),
			key)
		if err != nil {
			return nil, errs.New("invalid encryption access entry: %v", err)
		}
	}

	return access, nil
}
