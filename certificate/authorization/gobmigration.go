// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/gob"
	"errors"
	"io"

	"storj.io/storj/private/kvstore"
)

const gobUint64Size = 8

var (
	gobRootTypeName    = []byte("Group")
	gobRootTypeNameOld = []byte("Authorizations") // before commit 1fc0c63

	errGobBadUint = errors.New("gob: encoded unsigned integer out of range")
)

// The serialized authorization.Group may contain instances of
// x509.Certificate whose PublicKey is elliptic.P256().
// Go 1.19 changed the implementation of elliptic.P256() such that
// it is incompatible with the way gob serialized it in earlier versions.
// Below is how it appeared in 1.18, and we register it with gob
// so we don't get an error when deserializing authorization.Group.
type p256Curve struct {
	*elliptic.CurveParams
}

func init() {
	gob.Register(&ecdsa.PublicKey{})
	gob.Register(&rsa.PublicKey{})
	gob.RegisterName("crypto/elliptic.p256Curve", &p256Curve{})
}

// MigrateGob migrates gob encoded Group to protobuf encoded Group.
func (authDB *DB) MigrateGob(ctx context.Context, progress func(count int)) (count int, err error) {
	defer mon.Task()(&ctx)(&err)
	i := 0
	err = authDB.db.Range(ctx, func(ctx context.Context, key kvstore.Key, value kvstore.Value) error {
		isGob, err := isLikelyGobEncoded(value)
		if err != nil {
			return ErrDBInternal.New("gob check failed key=%q: %w", key, err)
		}
		if !isGob {
			return nil
		}

		var group Group
		decoder := gob.NewDecoder(bytes.NewBuffer(value))
		if err := decoder.Decode(&group); err != nil {
			return ErrDBInternal.New("unmarshal failed key=%q: %w", key, err)
		}

		newValue, err := group.Marshal()
		if err != nil {
			return ErrDBInternal.New("re-marshal failed key=%q: %w", key, err)
		}

		err = authDB.db.CompareAndSwap(ctx, key, value, newValue)
		if err != nil {
			return ErrDBInternal.New("updating %q failed: %w", key, err)
		}

		i++
		if progress != nil {
			progress(i)
		}

		return nil
	})

	return i, ErrDBInternal.Wrap(err)
}

// isLikelyGobEncoded returns true if the byte slice is likely a gob-encoded buffer
// of []*authorization.Group and false if it isn't.
func isLikelyGobEncoded(data []byte) (bool, error) {
	r := bytes.NewReader(data)
	buf := make([]byte, len(gobRootTypeNameOld)) // gobRootTypeNameOld is the longest value we will read at once

	filterErrs := func(err error) error {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, errGobBadUint) {
			return nil
		}
		return err
	}

	// Each message is preceded by a value indicating how many bytes remain in the message.
	msgLen, err := readGobUint(r, buf)
	if err != nil {
		return false, filterErrs(err)
	}
	if uint64(r.Len()) < msgLen {
		return false, nil
	}

	// The next value in the buffer is the ID of an encoded type.
	// If it's negative, a type definition follows. Otherwise, a type instance follows.
	// We expect since the former since the definition of a type must precede its instances,
	// and we haven't consumed any definitions yet.
	typeID, err := readGobInt(r, buf)
	if err != nil {
		return false, filterErrs(err)
	}
	if typeID >= 0 {
		return false, nil
	}

	// The next 3 bytes must be 2, 1, and 1. These bytes serve as an index into the internal type gob.wireType.
	// - 2 corresponds to the 2nd field in gob.wireType, which is the field for slice info.
	//   If the first byte isn't 2, then the type definition isn't describing a []*authorization.Group.
	// - 1 corresponds to the 1st field in gob.sliceType, which contains common type info.
	// - 1 corresponds to the 1st field in gob.CommonType, which contains the type's name.
	// See: https://pkg.go.dev/encoding/gob#hdr-Encoding_Details
	_, err = io.ReadFull(r, buf[0:3])
	if err != nil {
		return false, filterErrs(err)
	}
	if !bytes.Equal([]byte{2, 1, 1}, buf[0:3]) {
		return false, nil
	}

	// The next value should be the length of the name of the root type.
	nameLen, err := readGobUint(r, buf)
	if err != nil {
		return false, filterErrs(err)
	}
	var name []byte
	switch int(nameLen) {
	case len(gobRootTypeName):
		name = gobRootTypeName
	case len(gobRootTypeNameOld):
		name = gobRootTypeNameOld
	default:
		return false, nil
	}

	// The next value should be the name of the root type.
	_, err = io.ReadFull(r, buf[0:nameLen])
	if err != nil {
		return false, filterErrs(err)
	}
	if !bytes.Equal(name, buf[0:nameLen]) {
		return false, nil
	}

	return true, nil
}

// readGobUint reads a gob-encoded unsigned integer from an io.Reader.
// Adapted from the Go source code: https://go.googlesource.com/go/+/09b5de4/src/encoding/gob/decode.go
func readGobUint(r io.Reader, buf []byte) (uint64, error) {
	n, err := io.ReadFull(r, buf[0:1])
	if n == 0 {
		return 0, err
	}
	b := buf[0]
	if b <= 0x7f {
		return uint64(b), nil
	}
	n = -int(int8(b))
	if n > gobUint64Size {
		return 0, errGobBadUint
	}
	width, err := io.ReadFull(r, buf[0:n])
	if err != nil {
		return 0, err
	}
	// Could check that the high byte is zero but it's not worth it.
	var x uint64
	for _, b := range buf[0:width] {
		x = x<<8 | uint64(b)
	}
	return x, nil
}

// readGobInt reads a gob-encoded integer from an io.Reader.
// Adapted from the Go source code: https://go.googlesource.com/go/+/09b5de4/src/encoding/gob/decode.go
func readGobInt(r io.Reader, buf []byte) (int64, error) {
	x, err := readGobUint(r, buf)
	if err != nil {
		return 0, err
	}
	i := int64(x >> 1)
	if x&1 != 0 {
		i = ^i
	}
	return i, nil
}
