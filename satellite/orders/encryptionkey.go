// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
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

// EncryptionKey contains an identifier and an encryption key that is used to
// encrypt transient metadata in orders.
//
// Can be used as a flag.
type EncryptionKey struct {
	ID  EncryptionKeyID
	Key storj.Key
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
	if keys.KeyByID == nil {
		keys.KeyByID = map[EncryptionKeyID]storj.Key{}
	}

	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		var ekey EncryptionKey
		if err := ekey.Set(x); err != nil {
			return ErrEncryptionKey.New("invalid keys %q: %v", s, err)
		}
		if ekey.IsZero() {
			continue
		}

		if keys.Default.IsZero() {
			keys.Default = ekey
		}

		if _, exists := keys.KeyByID[ekey.ID]; exists {
			return ErrEncryptionKey.New("duplicate key identifier %q", s)
		}

		keys.List = append(keys.List, ekey)
		keys.KeyByID[ekey.ID] = ekey.Key
	}

	return nil
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
