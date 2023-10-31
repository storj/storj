// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql"
	"database/sql/driver"
	"encoding/binary"

	"github.com/jackc/pgtype"

	"storj.io/common/storj"
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

type encryptionParameters struct {
	*storj.EncryptionParameters
}

// Check that EncryptionParameters layout doesn't change.
var _ struct {
	CipherSuite storj.CipherSuite
	BlockSize   int32
} = storj.EncryptionParameters{}

// Value implements sql/driver.Valuer interface.
func (params encryptionParameters) Value() (driver.Value, error) {
	var bytes [8]byte
	bytes[0] = byte(params.CipherSuite)
	binary.LittleEndian.PutUint32(bytes[1:], uint32(params.BlockSize))
	return int64(binary.LittleEndian.Uint64(bytes[:])), nil
}

// Scan implements sql.Scanner interface.
func (params encryptionParameters) Scan(value interface{}) error {
	switch value := value.(type) {
	case int64:
		var bytes [8]byte
		binary.LittleEndian.PutUint64(bytes[:], uint64(value))
		params.CipherSuite = storj.CipherSuite(bytes[0])
		params.BlockSize = int32(binary.LittleEndian.Uint32(bytes[1:]))
		return nil
	default:
		return Error.New("unable to scan %T into EncryptionParameters", value)
	}
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

type redundancyScheme struct {
	*storj.RedundancyScheme
}

// Check that RedundancyScheme layout doesn't change.
var _ struct {
	Algorithm      storj.RedundancyAlgorithm
	ShareSize      int32
	RequiredShares int16
	RepairShares   int16
	OptimalShares  int16
	TotalShares    int16
} = storj.RedundancyScheme{}

func (params redundancyScheme) Value() (driver.Value, error) {
	switch {
	case params.ShareSize < 0 || params.ShareSize >= 1<<24:
		return nil, Error.New("invalid share size %v", params.ShareSize)
	case params.RequiredShares < 0 || params.RequiredShares >= 1<<8:
		return nil, Error.New("invalid required shares %v", params.RequiredShares)
	case params.RepairShares < 0 || params.RepairShares >= 1<<8:
		return nil, Error.New("invalid repair shares %v", params.RepairShares)
	case params.OptimalShares < 0 || params.OptimalShares >= 1<<8:
		return nil, Error.New("invalid optimal shares %v", params.OptimalShares)
	case params.TotalShares < 0 || params.TotalShares >= 1<<8:
		return nil, Error.New("invalid total shares %v", params.TotalShares)
	}

	var bytes [8]byte
	bytes[0] = byte(params.Algorithm)

	// little endian uint32
	bytes[1] = byte(params.ShareSize >> 0)
	bytes[2] = byte(params.ShareSize >> 8)
	bytes[3] = byte(params.ShareSize >> 16)

	bytes[4] = byte(params.RequiredShares)
	bytes[5] = byte(params.RepairShares)
	bytes[6] = byte(params.OptimalShares)
	bytes[7] = byte(params.TotalShares)

	return int64(binary.LittleEndian.Uint64(bytes[:])), nil
}

func (params redundancyScheme) Scan(value interface{}) error {
	switch value := value.(type) {
	case int64:
		var bytes [8]byte
		binary.LittleEndian.PutUint64(bytes[:], uint64(value))

		params.Algorithm = storj.RedundancyAlgorithm(bytes[0])

		// little endian uint32
		params.ShareSize = int32(bytes[1]) | int32(bytes[2])<<8 | int32(bytes[3])<<16

		params.RequiredShares = int16(bytes[4])
		params.RepairShares = int16(bytes[5])
		params.OptimalShares = int16(bytes[6])
		params.TotalShares = int16(bytes[7])

		return nil
	default:
		return Error.New("unable to scan %T into RedundancyScheme", value)
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
