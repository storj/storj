// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql"
	"database/sql/driver"
	"encoding/binary"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgtype"
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

type (
	unexpectedDimension  struct{}
	invalidElementLength struct{}
)

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

// EncodeSpanner implements spanner.Encoder.
func (s StreamIDSuffix) EncodeSpanner() (any, error) {
	return s.Value()
}

// Value implements sql/driver.Valuer.
func (s StreamIDSuffix) Value() (driver.Value, error) {
	return s[:], nil
}
