// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"time"

	"github.com/jackc/pgtype"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// The following XArray() helper methods exist alongside similar methods in the
// jackc/pgtype library. The difference with the methods in pgtype is that they
// will accept any of a wide range of types. That is nice, but it comes with
// the potential that someone might pass in an invalid type; thus, those
// methods have to return (*pgtype.XArray, error).
//
// The methods here do not need to return an error because they require passing
// in the correct type to begin with.
//
// An alternative implementation for the following methods might look like
// calls to pgtype.ByteaArray() followed by `if err != nil { panic }` blocks.
// That would probably be ok, but we decided on this approach, as it ought to
// require fewer allocations and less time, in addition to having no error
// return.

// ByteaArray returns an object usable by pg drivers for passing a [][]byte slice
// into a database as type BYTEA[].
//
// If any elements of bytesArray are nil, they will be represented in the
// database as an empty bytes array (not NULL). See also NullByteaArray.
func ByteaArray(bytesArray [][]byte) *pgtype.ByteaArray {
	pgtypeByteaArray := make([]pgtype.Bytea, len(bytesArray))
	for i, byteSlice := range bytesArray {
		pgtypeByteaArray[i].Bytes = byteSlice
		pgtypeByteaArray[i].Status = pgtype.Present
	}
	return &pgtype.ByteaArray{
		Elements:   pgtypeByteaArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(bytesArray)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// NullByteaArray returns an object usable by pg drivers for passing a
// [][]byte slice into a database as type BYTEA[]. It allows for elements of
// bytesArray to be nil, which will correspond to a NULL value in the database.
//
// This is probably the way that ByteaArray should have worked all along, but
// we won't change it now in case some things depend on the existing behavior.
func NullByteaArray(bytesArray [][]byte) *pgtype.ByteaArray {
	pgtypeByteaArray := make([]pgtype.Bytea, len(bytesArray))
	for i, byteSlice := range bytesArray {
		pgtypeByteaArray[i].Bytes = byteSlice
		if byteSlice == nil {
			pgtypeByteaArray[i].Status = pgtype.Null
		} else {
			pgtypeByteaArray[i].Status = pgtype.Present
		}
	}
	return &pgtype.ByteaArray{
		Elements:   pgtypeByteaArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(bytesArray)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// TextArray returns an object usable by pg drivers for passing a []string slice
// into a database as type TEXT[].
func TextArray(stringSlice []string) *pgtype.TextArray {
	pgtypeTextArray := make([]pgtype.Text, len(stringSlice))
	for i, s := range stringSlice {
		pgtypeTextArray[i].String = s
		pgtypeTextArray[i].Status = pgtype.Present
	}
	return &pgtype.TextArray{
		Elements:   pgtypeTextArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(stringSlice)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// TimestampTZArray returns an object usable by pg drivers for passing a []time.Time
// slice into a database as type TIMESTAMPTZ[].
func TimestampTZArray(timeSlice []time.Time) *pgtype.TimestamptzArray {
	pgtypeTimestamptzArray := make([]pgtype.Timestamptz, len(timeSlice))
	for i, t := range timeSlice {
		pgtypeTimestamptzArray[i].Time = t
		pgtypeTimestamptzArray[i].Status = pgtype.Present
	}
	return &pgtype.TimestamptzArray{
		Elements:   pgtypeTimestamptzArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(timeSlice)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// NullTimestampTZArray returns an object usable by pg drivers for passing a []*time.Time
// slice into a database as type TIMESTAMPTZ[].
func NullTimestampTZArray(timeSlice []*time.Time) *pgtype.TimestamptzArray {
	pgtypeTimestamptzArray := make([]pgtype.Timestamptz, len(timeSlice))
	for i, t := range timeSlice {
		if t == nil {
			pgtypeTimestamptzArray[i].Status = pgtype.Null
		} else {
			pgtypeTimestamptzArray[i].Time = *t
			pgtypeTimestamptzArray[i].Status = pgtype.Present
		}
	}
	return &pgtype.TimestamptzArray{
		Elements:   pgtypeTimestamptzArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(timeSlice)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// DateArray returns an object usable by pg drivers for passing a []time.Time
// slice into a database as type Date[].
func DateArray(timeSlice []time.Time) *pgtype.DateArray {
	pgtypeDateArray := make([]pgtype.Date, len(timeSlice))
	for i, t := range timeSlice {
		pgtypeDateArray[i].Time = t
		pgtypeDateArray[i].Status = pgtype.Present
	}
	return &pgtype.DateArray{
		Elements:   pgtypeDateArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(timeSlice)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// Int2Array returns an object usable by pg drivers for passing a []int16 slice
// into a database as type INT2[].
func Int2Array(ints []int16) *pgtype.Int2Array {
	pgtypeInt2Array := make([]pgtype.Int2, len(ints))
	for i, someInt := range ints {
		pgtypeInt2Array[i].Int = someInt
		pgtypeInt2Array[i].Status = pgtype.Present
	}
	return &pgtype.Int2Array{
		Elements:   pgtypeInt2Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(ints)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// Int4Array returns an object usable by pg drivers for passing a []int32 slice
// into a database as type INT4[].
func Int4Array(ints []int32) *pgtype.Int4Array {
	pgtypeInt4Array := make([]pgtype.Int4, len(ints))
	for i, someInt := range ints {
		pgtypeInt4Array[i].Int = someInt
		pgtypeInt4Array[i].Status = pgtype.Present
	}
	return &pgtype.Int4Array{
		Elements:   pgtypeInt4Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(ints)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// Int8Array returns an object usable by pg drivers for passing a []int64 slice
// into a database as type INT8[].
func Int8Array(bigInts []int64) *pgtype.Int8Array {
	pgtypeInt8Array := make([]pgtype.Int8, len(bigInts))
	for i, bigInt := range bigInts {
		pgtypeInt8Array[i].Int = bigInt
		pgtypeInt8Array[i].Status = pgtype.Present
	}
	return &pgtype.Int8Array{
		Elements:   pgtypeInt8Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(bigInts)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// Float8Array returns an object usable by pg drivers for passing a []float64 slice
// into a database as type FLOAT8[].
func Float8Array(floats []float64) *pgtype.Float8Array {
	pgtypeFloat8Array := make([]pgtype.Float8, len(floats))
	for i, someFloat := range floats {
		pgtypeFloat8Array[i].Float = someFloat
		pgtypeFloat8Array[i].Status = pgtype.Present
	}
	return &pgtype.Float8Array{
		Elements:   pgtypeFloat8Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(floats)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// NodeIDArray returns an object usable by pg drivers for passing a []storj.NodeID
// slice into a database as type BYTEA[].
func NodeIDArray(nodeIDs []storj.NodeID) *pgtype.ByteaArray {
	if nodeIDs == nil {
		return &pgtype.ByteaArray{Status: pgtype.Null}
	}
	pgtypeByteaArray := make([]pgtype.Bytea, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		nodeIDCopy := nodeID
		pgtypeByteaArray[i].Bytes = nodeIDCopy[:]
		pgtypeByteaArray[i].Status = pgtype.Present
	}
	return &pgtype.ByteaArray{
		Elements:   pgtypeByteaArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(nodeIDs)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// UUIDArray returns an object usable by pg drivers for passing a []uuid.UUID
// slice into a database as type BYTEA[].
func UUIDArray(uuids []uuid.UUID) *pgtype.ByteaArray {
	if uuids == nil {
		return &pgtype.ByteaArray{Status: pgtype.Null}
	}
	pgtypeByteaArray := make([]pgtype.Bytea, len(uuids))
	for i, uuid := range uuids {
		uuidCopy := uuid
		pgtypeByteaArray[i].Bytes = uuidCopy[:]
		pgtypeByteaArray[i].Status = pgtype.Present
	}
	return &pgtype.ByteaArray{
		Elements:   pgtypeByteaArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(uuids)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// PlacementConstraintArray returns an object usable by pg drivers for passing a
// []storj.PlacementConstraint slice into a database as type INT2[].
func PlacementConstraintArray(constraints []storj.PlacementConstraint) *pgtype.Int2Array {
	pgtypeInt2Array := make([]pgtype.Int2, len(constraints))
	for i, someInt := range constraints {
		pgtypeInt2Array[i].Int = int16(someInt)
		pgtypeInt2Array[i].Status = pgtype.Present
	}
	return &pgtype.Int2Array{
		Elements:   pgtypeInt2Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(constraints)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}
