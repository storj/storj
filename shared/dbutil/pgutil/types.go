// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"database/sql"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// The following XArray() helper methods return values usable as query
// arguments for the pgx driver, which encodes Go slices as PostgreSQL arrays
// natively.
//
// The main purpose of these helpers is to keep the semantics consistent:
// pgx encodes a nil slice as NULL, while most of the helpers here always
// produce an array, which is empty for nil input.

// nonNilSlice ensures that a nil slice is passed to the database as an empty
// array instead of NULL.
func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// ByteaArray returns an object usable by pg drivers for passing a [][]byte slice
// into a database as type BYTEA[].
//
// If any elements of bytesArray are nil, they will be represented in the
// database as an empty bytes array (not NULL). See also NullByteaArray.
func ByteaArray(bytesArray [][]byte) [][]byte {
	result := make([][]byte, len(bytesArray))
	for i, byteSlice := range bytesArray {
		if byteSlice == nil {
			byteSlice = []byte{}
		}
		result[i] = byteSlice
	}
	return result
}

// NullByteaArray returns an object usable by pg drivers for passing a
// [][]byte slice into a database as type BYTEA[]. It allows for elements of
// bytesArray to be nil, which will correspond to a NULL value in the database.
//
// This is probably the way that ByteaArray should have worked all along, but
// we won't change it now in case some things depend on the existing behavior.
func NullByteaArray(bytesArray [][]byte) [][]byte {
	return nonNilSlice(bytesArray)
}

// TextArray returns an object usable by pg drivers for passing a []string slice
// into a database as type TEXT[].
func TextArray(stringSlice []string) []string {
	return nonNilSlice(stringSlice)
}

// NullTextArray returns an object usable by pg drivers for passing a []*string
// slice into a database as type TEXT[]. It allows for elements of stringSlice to
// be nil, which will correspond to a NULL value in the database.
func NullTextArray(stringSlice []*string) []*string {
	return nonNilSlice(stringSlice)
}

// TimestampTZArray returns an object usable by pg drivers for passing a []time.Time
// slice into a database as type TIMESTAMPTZ[].
func TimestampTZArray(timeSlice []time.Time) []time.Time {
	return nonNilSlice(timeSlice)
}

// NullTimestampTZArray returns an object usable by pg drivers for passing a []*time.Time
// slice into a database as type TIMESTAMPTZ[].
func NullTimestampTZArray(timeSlice []*time.Time) []*time.Time {
	return nonNilSlice(timeSlice)
}

// DateArray returns an object usable by pg drivers for passing a []time.Time
// slice into a database as type Date[].
func DateArray(timeSlice []time.Time) []time.Time {
	return nonNilSlice(timeSlice)
}

// Int2Array returns an object usable by pg drivers for passing a []int16 slice
// into a database as type INT2[].
func Int2Array(ints []int16) []int16 {
	return nonNilSlice(ints)
}

// Int4Array returns an object usable by pg drivers for passing a []int32 slice
// into a database as type INT4[].
func Int4Array(ints []int32) []int32 {
	return nonNilSlice(ints)
}

// Int8Array returns an object usable by pg drivers for passing a []int64 slice
// into a database as type INT8[].
func Int8Array(bigInts []int64) []int64 {
	return nonNilSlice(bigInts)
}

// Float8Array returns an object usable by pg drivers for passing a []float64 slice
// into a database as type FLOAT8[].
func Float8Array(floats []float64) []float64 {
	return nonNilSlice(floats)
}

// NodeIDArray returns an object usable by pg drivers for passing a []storj.NodeID
// slice into a database as type BYTEA[]. A nil slice is passed as NULL.
func NodeIDArray(nodeIDs []storj.NodeID) [][]byte {
	if nodeIDs == nil {
		return nil
	}
	result := make([][]byte, len(nodeIDs))
	for i := range nodeIDs {
		result[i] = nodeIDs[i][:]
	}
	return result
}

// UUIDArray returns an object usable by pg drivers for passing a []uuid.UUID
// slice into a database as type BYTEA[]. A nil slice is passed as NULL.
func UUIDArray(uuids []uuid.UUID) [][]byte {
	if uuids == nil {
		return nil
	}
	result := make([][]byte, len(uuids))
	for i := range uuids {
		result[i] = uuids[i][:]
	}
	return result
}

// PlacementConstraintArray returns an object usable by pg drivers for passing a
// []storj.PlacementConstraint slice into a database as type INT2[].
func PlacementConstraintArray(constraints []storj.PlacementConstraint) []int16 {
	result := make([]int16, len(constraints))
	for i, constraint := range constraints {
		result[i] = int16(constraint)
	}
	return result
}

// typeMaps holds pgtype.Maps for reuse. A Map caches encode and scan plans,
// but it's not safe for concurrent use.
var typeMaps = sync.Pool{New: func() any { return pgtype.NewMap() }}

// ScanArray decodes src, a PostgreSQL array as returned by database/sql (in
// text format), into dest. dest must be a pointer to a slice of a type known
// to pgx (e.g. *[]string, *[]int64, *[][]byte). NULL is decoded as a nil slice.
func ScanArray(src, dest any) error {
	var buf []byte
	switch src := src.(type) {
	case nil:
	case string:
		buf = []byte(src)
	case []byte:
		buf = src
	default:
		return errs.New("unable to scan %T as array", src)
	}

	m := typeMaps.Get().(*pgtype.Map)
	defer typeMaps.Put(m)

	typ, ok := m.TypeForValue(dest)
	if !ok {
		return errs.New("unsupported array destination %T", dest)
	}
	return errs.Wrap(m.Scan(typ.OID, pgtype.TextFormatCode, buf, dest))
}

// EncodeArray returns the text representation of a PostgreSQL array, for use
// in driver.Valuer implementations. src must be a slice of a type known to
// pgx (e.g. []string, []int64, [][]byte).
func EncodeArray(src any) (string, error) {
	m := typeMaps.Get().(*pgtype.Map)
	defer typeMaps.Put(m)

	typ, ok := m.TypeForValue(src)
	if !ok {
		return "", errs.New("unsupported array source %T", src)
	}
	buf, err := m.Encode(typ.OID, pgtype.TextFormatCode, src, nil)
	if err != nil {
		return "", errs.Wrap(err)
	}
	return string(buf), nil
}

// ArrayScanner returns an sql.Scanner, which decodes a PostgreSQL array into
// dest. See ScanArray for the supported types.
func ArrayScanner(dest any) sql.Scanner {
	return arrayScanner{dest: dest}
}

type arrayScanner struct {
	dest any
}

// Scan implements sql.Scanner.
func (scanner arrayScanner) Scan(src any) error {
	return ScanArray(src, scanner.dest)
}
