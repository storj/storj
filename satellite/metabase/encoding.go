// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"strconv"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgtype"

	"storj.io/common/storj"
)

// Constants for encoding an object's retention mode and legal hold status
// as a single value in the retention_mode column of the objects table.
const (
	// retentionModeMask is a bit mask used to identify bits related to storj.RetentionMode.
	retentionModeMask = 0b11

	// legalHoldFlag is a bit flag signifying that an object version is locked in legal hold
	// and cannot be deleted or modified until the legal hold is removed.
	legalHoldFlag = 0b100
)

type encoderDecoder interface {
	driver.Valuer
	sql.Scanner
	spanner.Encoder
	spanner.Decoder
}

var (
	_ encoderDecoder = (*SegmentPosition)(nil)
	_ encoderDecoder = lockModeWrapper{}
	_ encoderDecoder = timeWrapper{}
)

type nullableValue[T sql.Scanner] struct {
	isnull bool
	value  T
}

func (v *nullableValue[T]) Scan(value interface{}) error {
	if value == nil {
		v.isnull = true
		return nil
	}
	v.isnull = false
	return v.value.Scan(value)
}

// Value implements sql/driver.Valuer interface.
func (params SegmentPosition) Value() (driver.Value, error) {
	return int64(params.Encode()), nil
}

// Scan implements sql.Scanner interface.
func (params *SegmentPosition) Scan(value interface{}) error {
	switch value := value.(type) {
	case int64:
		*params = SegmentPositionFromEncoded(uint64(value))
		return nil
	default:
		return Error.New("unable to scan %T into SegmentPosition", value)
	}
}

// Value implements sql/driver.Valuer interface.
func (pieces Pieces) Value() (driver.Value, error) {
	if len(pieces) == 0 {
		arr := &pgtype.ByteaArray{Status: pgtype.Null}
		return arr.Value()
	}

	elems := make([]pgtype.Bytea, len(pieces))
	for i, piece := range pieces {
		var buf [2 + len(piece.StorageNode)]byte
		binary.BigEndian.PutUint16(buf[0:], piece.Number)
		copy(buf[2:], piece.StorageNode[:])

		elems[i].Bytes = buf[:]
		elems[i].Status = pgtype.Present
	}

	arr := &pgtype.ByteaArray{
		Elements:   elems,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(pieces)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
	return arr.Value()
}

type unexpectedDimension struct{}
type invalidElementLength struct{}

func (unexpectedDimension) Error() string  { return "unexpected data dimension" }
func (invalidElementLength) Error() string { return "invalid element length" }

// Scan implements sql.Scanner interface.
func (pieces *Pieces) Scan(value interface{}) error {
	var arr pgtype.ByteaArray
	if err := arr.Scan(value); err != nil {
		return err
	}

	if len(arr.Dimensions) == 0 {
		*pieces = nil
		return nil
	} else if len(arr.Dimensions) != 1 {
		return unexpectedDimension{}
	}

	scan := make(Pieces, len(arr.Elements))
	for i, elem := range arr.Elements {
		piece := Piece{}
		if len(elem.Bytes) != 2+len(piece.StorageNode) {
			return invalidElementLength{}
		}

		piece.Number = binary.BigEndian.Uint16(elem.Bytes[0:])
		copy(piece.StorageNode[:], elem.Bytes[2:])
		scan[i] = piece
	}

	*pieces = scan
	return nil
}

// RetentionMode implements scanning for retention_mode column.
type RetentionMode struct {
	Mode      storj.RetentionMode
	LegalHold bool
}

// Value implements the sql/driver.Valuer interface.
func (r RetentionMode) Value() (driver.Value, error) {
	if int64(r.Mode)&retentionModeMask != int64(r.Mode) {
		return nil, Error.New("invalid retention mode")
	}

	val := int64(r.Mode)
	if r.LegalHold {
		val |= legalHoldFlag
	}

	return val, nil
}

func (r *RetentionMode) set(v int64) {
	r.Mode = storj.RetentionMode(v & retentionModeMask)
	r.LegalHold = v&legalHoldFlag != 0
}

// Scan implements the sql.Scanner interface.
func (r *RetentionMode) Scan(val interface{}) error {
	if val == nil {
		*r = RetentionMode{}
		return nil
	}
	if v, ok := val.(int64); ok {
		r.set(v)
		return nil
	}
	return Error.New("unable to scan %T", val)
}

// EncodeSpanner implements the spanner.Encoder interface.
func (r RetentionMode) EncodeSpanner() (interface{}, error) {
	return r.Value()
}

// DecodeSpanner implements the spanner.Decoder interface.
func (r *RetentionMode) DecodeSpanner(val interface{}) error {
	switch v := val.(type) {
	case *string:
		if v == nil {
			*r = RetentionMode{}
			return nil
		}
		iVal, err := strconv.ParseInt(*v, 10, 64)
		if err != nil {
			return Error.New("unable to parse %q as int64: %w", *v, err)
		}
		r.set(iVal)
		return nil
	case string:
		iVal, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Error.New("unable to parse %q as int64: %w", v, err)
		}
		r.set(iVal)
		return nil
	case int64:
		r.set(v)
		return nil
	default:
		return r.Scan(val)
	}
}

type lockModeWrapper struct {
	retentionMode *storj.RetentionMode
	legalHold     *bool
}

// Value implements the sql/driver.Valuer interface.
func (r lockModeWrapper) Value() (driver.Value, error) {
	var val int64
	if r.retentionMode != nil {
		val = int64(*r.retentionMode)
	}
	if r.legalHold != nil && *r.legalHold {
		val |= legalHoldFlag
	}
	if val == 0 {
		return nil, nil
	}
	return val, nil
}

// Clear resets to the default values.
func (r lockModeWrapper) Clear() {
	if r.retentionMode != nil {
		*r.retentionMode = storj.NoRetention
	}
	if r.legalHold != nil {
		*r.legalHold = false
	}
}

// Set from am encoded value.
func (r lockModeWrapper) Set(val int64) {
	if r.retentionMode != nil {
		*r.retentionMode = storj.RetentionMode(val & retentionModeMask)
	}
	if r.legalHold != nil {
		*r.legalHold = val&legalHoldFlag != 0
	}
}

// Scan implements the sql.Scanner interface.
func (r lockModeWrapper) Scan(val interface{}) error {
	if val == nil {
		r.Clear()
		return nil
	}
	if v, ok := val.(int64); ok {
		r.Set(v)
		return nil
	}
	return Error.New("unable to scan %T", val)
}

// EncodeSpanner implements the spanner.Encoder interface.
func (r lockModeWrapper) EncodeSpanner() (interface{}, error) {
	return r.Value()
}

// DecodeSpanner implements the spanner.Decoder interface.
func (r lockModeWrapper) DecodeSpanner(val interface{}) error {
	if strPtrVal, ok := val.(*string); ok {
		if strPtrVal == nil {
			r.Clear()
			return nil
		}
		val = strPtrVal
	}
	if strVal, ok := val.(string); ok {
		iVal, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return Error.New("unable to parse %q as int64: %w", strVal, err)
		}
		r.Set(iVal)
		return nil
	}
	return r.Scan(val)
}

type timeWrapper struct {
	*time.Time
}

// Value implements the sql/driver.Valuer interface.
func (t timeWrapper) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return *t.Time, nil
}

// Scan implements the sql.Scanner interface.
func (t timeWrapper) Scan(val interface{}) error {
	if val == nil {
		*t.Time = time.Time{}
		return nil
	}
	if v, ok := val.(time.Time); ok {
		*t.Time = v
		return nil
	}
	return Error.New("unable to scan %T into time.Time", val)
}

// EncodeSpanner implements the spanner.Encoder interface.
func (t timeWrapper) EncodeSpanner() (interface{}, error) {
	return t.Value()
}

// DecodeSpanner implements the spanner.Decoder interface.
func (t timeWrapper) DecodeSpanner(val interface{}) error {
	if strPtrVal, ok := val.(*string); ok {
		if strPtrVal == nil {
			*t.Time = time.Time{}
			return nil
		}
		val = strPtrVal
	}
	if strVal, ok := val.(string); ok {
		tVal, err := time.Parse(time.RFC3339Nano, strVal)
		if err != nil {
			return Error.New("unable to parse %q as time.Time: %w", strVal, err)
		}
		*t.Time = tVal
		return nil
	}
	return t.Scan(val)
}

// EncodeSpanner implements spanner.Encoder.
func (s StreamIDSuffix) EncodeSpanner() (any, error) {
	return s.Value()
}

// Value implements sql/driver.Valuer.
func (s StreamIDSuffix) Value() (driver.Value, error) {
	return s[:], nil
}
