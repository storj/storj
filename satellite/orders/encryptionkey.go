// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/zeebo/errs"
	"golang.org/x/crypto/nacl/secretbox"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/internalpb"
)

// ErrEncryptionKey is error class used for keys.
var ErrEncryptionKey = errs.Class("order encryption key")

// EncryptionKeyID is used to identify an encryption key.
type EncryptionKeyID [8]byte

// IsZero returns whether the key contains no data.
func (key EncryptionKeyID) IsZero() bool { return key == EncryptionKeyID{} }

// EncryptionKeys contains a collection of keys.
//
// Can be used as a flag.
type EncryptionKeys struct {
	Default EncryptionKey
	List    []EncryptionKey
	KeyByID map[EncryptionKeyID]storj.Key
}

// NewEncryptionKeys creates a new EncrytpionKeys object with the provided keys.
func NewEncryptionKeys(keys ...EncryptionKey) (*EncryptionKeys, error) {
	var ekeys EncryptionKeys
	for _, key := range keys {
		if err := ekeys.Add(key); err != nil {
			return nil, err
		}
	}
	return &ekeys, nil
}

// EncryptionKey contains an identifier and an encryption key that is used to
// encrypt transient metadata in orders.
//
// Can be used as a flag.
type EncryptionKey struct {
	ID  EncryptionKeyID
	Key storj.Key
}

// When this fails to compile, then `serialToNonce` should be adjusted accordingly.
var _ = ([16]byte)(storj.SerialNumber{})

func serialToNonce(serial storj.SerialNumber) (x [24]byte) {
	copy(x[:], serial[:])
	return x
}

// Encrypt encrypts data and nonce using the key.
func (key *EncryptionKey) Encrypt(plaintext []byte, nonce storj.SerialNumber) []byte {
	out := make([]byte, 0, len(plaintext)+secretbox.Overhead)
	n := serialToNonce(nonce)
	k := ([32]byte)(key.Key)
	return secretbox.Seal(out, plaintext, &n, &k)
}

// Decrypt decrypts data and nonce using the key.
func (key *EncryptionKey) Decrypt(ciphertext []byte, nonce storj.SerialNumber) ([]byte, error) {
	out := make([]byte, 0, len(ciphertext)-secretbox.Overhead)
	n := serialToNonce(nonce)
	k := ([32]byte)(key.Key)
	dec, ok := secretbox.Open(out, ciphertext, &n, &k)
	if !ok {
		return nil, ErrEncryptionKey.New("unable to decrypt")
	}
	return dec, nil
}

// EncryptMetadata encrypts order limit metadata.
func (key *EncryptionKey) EncryptMetadata(serial storj.SerialNumber, metadata *internalpb.OrderLimitMetadata) ([]byte, error) {
	marshaled, err := pb.Marshal(metadata)
	if err != nil {
		return nil, ErrEncryptionKey.Wrap(err)
	}
	return key.Encrypt(marshaled, serial), nil
}

// DecryptMetadata decrypts order limit metadata.
func (key *EncryptionKey) DecryptMetadata(serial storj.SerialNumber, encrypted []byte) (*internalpb.OrderLimitMetadata, error) {
	decrypted, err := key.Decrypt(encrypted, serial)
	if err != nil {
		return nil, ErrEncryptionKey.Wrap(err)
	}

	metadata := &internalpb.OrderLimitMetadata{}
	err = pb.Unmarshal(decrypted, metadata)
	if err != nil {
		return nil, ErrEncryptionKey.Wrap(err)
	}

	return metadata, nil
}

// IsZero returns whether they key contains some data.
func (key *EncryptionKey) IsZero() bool {
	return key.ID.IsZero() || key.Key.IsZero()
}

// Type implements pflag.Value.
func (EncryptionKey) Type() string { return "orders.EncryptionKey" }

// String is required for pflag.Value.
func (key *EncryptionKey) String() string {
	return hex.EncodeToString(key.ID[:]) + "=" + hex.EncodeToString(key.Key[:])
}

// Set sets the value from an hex encoded string "hex(id)=hex(key)".
func (key *EncryptionKey) Set(s string) error {
	tokens := strings.SplitN(s, "=", 2)
	if len(tokens) != 2 {
		return ErrEncryptionKey.New("invalid definition %q", s)
	}

	err := setHexEncodedArray(key.ID[:], tokens[0])
	if err != nil {
		return ErrEncryptionKey.New("invalid id %q: %v", tokens[0], err)
	}

	err = setHexEncodedArray(key.Key[:], tokens[1])
	if err != nil {
		return ErrEncryptionKey.New("invalid key %q: %v", tokens[1], err)
	}

	if key.ID.IsZero() || key.Key.IsZero() {
		return ErrEncryptionKey.New("neither identifier or key can be zero")
	}

	return nil
}

// Type implements pflag.Value.
func (EncryptionKeys) Type() string { return "orders.EncryptionKeys" }

// Set adds the values from a comma delimited hex encoded strings "hex(id1)=hex(key1),hex(id2)=hex(key2)".
func (keys *EncryptionKeys) Set(s string) error {
	if s == "" {
		return nil
	}

	keys.Clear()

	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		var ekey EncryptionKey
		if err := ekey.Set(x); err != nil {
			return ErrEncryptionKey.New("invalid keys %q: %w", s, err)
		}
		if err := keys.Add(ekey); err != nil {
			return err
		}
	}

	return nil
}

// Add adds an encryption key to EncryptionsKeys object.
func (keys *EncryptionKeys) Add(ekey EncryptionKey) error {
	if keys.KeyByID == nil {
		keys.KeyByID = map[EncryptionKeyID]storj.Key{}
	}
	if ekey.IsZero() {
		return ErrEncryptionKey.New("key is zero")
	}

	if keys.Default.IsZero() {
		keys.Default = ekey
	}

	if _, exists := keys.KeyByID[ekey.ID]; exists {
		return ErrEncryptionKey.New("duplicate key identifier %q", ekey.String())
	}

	keys.List = append(keys.List, ekey)
	keys.KeyByID[ekey.ID] = ekey.Key
	return nil
}

// Clear removes all keys.
func (keys *EncryptionKeys) Clear() {
	keys.Default = EncryptionKey{}
	keys.List = nil
	keys.KeyByID = map[EncryptionKeyID]storj.Key{}
}

// String is required for pflag.Value.
func (keys *EncryptionKeys) String() string {
	var s strings.Builder
	if keys.Default.IsZero() {
		return ""
	}

	s.WriteString(keys.Default.String())
	for _, key := range keys.List {
		if key.ID == keys.Default.ID {
			continue
		}

		s.WriteString(",")
		s.WriteString(key.String())
	}

	return s.String()
}

// setHexEncodedArray sets dst bytes to hex decoded s, verify that the result matches dst.
func setHexEncodedArray(dst []byte, s string) error {
	s = strings.TrimSpace(s)
	if len(s) != len(dst)*2 {
		return fmt.Errorf("wrong hex length %d, expected %d", len(s), len(dst)*2)
	}

	bytes, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	copy(dst, bytes)
	return nil
}
