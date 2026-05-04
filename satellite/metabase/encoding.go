// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/binary"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgtype"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/internalpb"
)

type encoder interface {
	driver.Valuer
	spanner.Encoder
}

type decoder interface {
	sql.Scanner
	spanner.Decoder
}

type encoderDecoder interface {
	encoder
	decoder
}

var (
	_ encoderDecoder = (*SegmentPosition)(nil)
	_ encoderDecoder = lockModeWrapper{}
	_ encoderDecoder = timeWrapper{}

	_ encoder = Checksum{}
	_ decoder = (*Checksum)(nil)
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

// Value implements the driver.Valuer interface.
func (checksum Checksum) Value() (driver.Value, error) {
	if checksum.IsZero() {
		return nil, nil
	}
	value, err := pb.Marshal(&internalpb.ObjectChecksum{
		Algorithm:      pb.ObjectChecksumAlgorithm(checksum.Algorithm),
		IsComposite:    checksum.IsComposite,
		EncryptedValue: checksum.EncryptedValue,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return value, nil
}

// EncodeSpanner implements the spanner.Encoder interface.
func (checksum Checksum) EncodeSpanner() (any, error) {
	return checksum.Value()
}

// Scan implements the sql.Scanner interface.
func (checksum *Checksum) Scan(value any) error {
	switch value := value.(type) {
	case nil:
		*checksum = Checksum{}
	case []byte:
		var pbChecksum internalpb.ObjectChecksum
		if err := pb.Unmarshal(value, &pbChecksum); err != nil {
			return Error.Wrap(err)
		}
		*checksum = Checksum{
			Algorithm:      storj.ObjectChecksumAlgorithm(pbChecksum.Algorithm),
			IsComposite:    pbChecksum.IsComposite,
			EncryptedValue: pbChecksum.EncryptedValue,
		}
	default:
		return Error.New("unable to scan %T into %T", value, checksum)
	}
	return nil
}

// DecodeSpanner implements the spanner.Decoder interface.
func (checksum *Checksum) DecodeSpanner(value any) (err error) {
	if valueStrPtr, ok := value.(*string); ok {
		if valueStrPtr == nil {
			*checksum = Checksum{}
			return nil
		}
		value, err = base64.StdEncoding.DecodeString(*valueStrPtr)
		if err != nil {
			return Error.Wrap(err)
		}
	} else if valueStr, ok := value.(string); ok {
		value, err = base64.StdEncoding.DecodeString(valueStr)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return checksum.Scan(value)
}
