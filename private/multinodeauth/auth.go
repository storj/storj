// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodeauth

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"

	"github.com/zeebo/errs"
)

// Secret crypto random 32 bytes array for multinode auth.
type Secret [32]byte

// NewSecret creates new multinode auth secret.
func NewSecret() (Secret, error) {
	var b [32]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return b, errs.New("error creating multinode auth secret")
	}

	return b, nil
}

// String implements Stringer.
func (secret Secret) String() string {
	return base64.URLEncoding.EncodeToString(secret[:])
}

// IsZero returns if secret is not set.
func (secret Secret) IsZero() bool {
	var zero Secret
	// this doesn't need to be constant-time, because we're explicitly testing
	// against a hardcoded, well-known value
	return bytes.Equal(secret[:], zero[:])
}

// MarshalJSON implements json.Marshaler Interface.
func (secret Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(secret.String())
}

// UnmarshalJSON implements json.Unmarshaler Interface.
func (secret *Secret) UnmarshalJSON(data []byte) error {
	var err error
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*secret, err = SecretFromBase64(s)
	return err
}

// SecretFromBase64 creates new secret from base64 string.
func SecretFromBase64(s string) (Secret, error) {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Secret{}, err
	}

	return SecretFromBytes(b)
}

// SecretFromBytes creates secret from bytes slice.
func SecretFromBytes(b []byte) (Secret, error) {
	if len(b) != 32 {
		return Secret{}, errs.New("invalid secret")
	}

	var secret Secret
	copy(secret[:], b)
	return secret, nil
}
