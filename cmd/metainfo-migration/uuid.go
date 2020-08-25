// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"io"

	"github.com/zeebo/errs"
)

// Error is the default error class for uuid.
var Error = errs.Class("uuid")

// UUID is big-endian encoded UUID.
//
// UUID can be of any version or variant.
type UUID [16]byte

// NewUUID returns a random UUID (version 4 variant 2).
func NewUUID() (UUID, error) {
	return newUUIDRandomFromReader(rand.Reader)
}

// newUUIDRandomFromReader returns a random UUID  (version 4 variant 2)
// using a custom reader.
func newUUIDRandomFromReader(r io.Reader) (UUID, error) {
	var uuid UUID
	_, err := io.ReadFull(r, uuid[:])
	if err != nil {
		return uuid, Error.Wrap(err)
	}

	// version 4, variant 2
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return uuid, nil
}

// IsZero returns true when all bytes in uuid are 0.
func (uuid UUID) IsZero() bool { return uuid == UUID{} }

// String returns uuid in "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" format.
func (uuid UUID) String() string {
	s := [36]byte{8: '-', 13: '-', 18: '-', 23: '-'}
	hex.Encode(s[0:8], uuid[0:4])
	hex.Encode(s[9:13], uuid[4:6])
	hex.Encode(s[14:18], uuid[6:8])
	hex.Encode(s[19:23], uuid[8:10])
	hex.Encode(s[24:36], uuid[10:16])
	return string(s[:])
}

// FromBytes converts big-endian raw-bytes to an UUID.
//
// FromBytes allows for any version or variant of an UUID.
func FromBytes(bytes []byte) (UUID, error) {
	var uuid UUID
	if len(uuid) != len(bytes) {
		return uuid, Error.New("bytes have wrong length %d expected %d", len(bytes), len(uuid))
	}
	copy(uuid[:], bytes)
	return uuid, nil
}

// FromString parses "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" string form.
//
// FromString allows for any version or variant of an UUID.
func FromString(s string) (UUID, error) {
	var uuid UUID
	if len(s) != 36 {
		return uuid, Error.New("invalid string length %d expected %d", len(s), 36)
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return uuid, Error.New("invalid string")
	}

	var err error
	_, err = hex.Decode(uuid[0:4], []byte(s)[0:8])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[4:6], []byte(s)[9:13])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[6:8], []byte(s)[14:18])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[8:10], []byte(s)[19:23])
	if err != nil {
		return uuid, Error.New("invalid string")
	}
	_, err = hex.Decode(uuid[10:16], []byte(s)[24:36])
	if err != nil {
		return uuid, Error.New("invalid string")
	}

	return uuid, nil
}

// Value implements sql/driver.Valuer interface.
func (uuid UUID) Value() (driver.Value, error) {
	return uuid[:], nil
}

// Scan implements sql.Scanner interface.
func (uuid *UUID) Scan(value interface{}) error {
	switch value := value.(type) {
	case []byte:
		x, err := FromBytes(value)
		if err != nil {
			return Error.Wrap(err)
		}
		*uuid = x
		return nil
	case string:
		x, err := FromString(value)
		if err != nil {
			return Error.Wrap(err)
		}
		*uuid = x
		return nil
	default:
		return Error.New("unable to scan %T into UUID", value)
	}
}

// MarshalText marshals UUID in "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" form.
func (uuid UUID) MarshalText() ([]byte, error) {
	return []byte(uuid.String()), nil
}

// UnmarshalText unmarshals UUID from "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
func (uuid *UUID) UnmarshalText(b []byte) error {
	x, err := FromString(string(b))
	if err != nil {
		return Error.Wrap(err)
	}
	*uuid = x
	return nil
}
