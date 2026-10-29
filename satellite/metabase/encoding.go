// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql"
	"database/sql/driver"
	"encoding/binary"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/shared/dbutil/pgutil"
)

type encoder interface {
	driver.Valuer
}

type decoder interface {
	sql.Scanner
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
func (params *SegmentPosition) Scan(value any) error {
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
		return nil, nil
	}

	elems := make([][]byte, len(pieces))
	for i, piece := range pieces {
		var buf [2 + len(piece.StorageNode)]byte
		binary.BigEndian.PutUint16(buf[0:], piece.Number)
		copy(buf[2:], piece.StorageNode[:])

		elems[i] = buf[:]
	}

	return pgutil.EncodeArray(elems)
}

type invalidElementLength struct{}

func (invalidElementLength) Error() string { return "invalid element length" }

// Scan implements sql.Scanner interface.
func (pieces *Pieces) Scan(value any) error {
	var elems [][]byte
	if err := pgutil.ScanArray(value, &elems); err != nil {
		return err
	}

	if len(elems) == 0 {
		*pieces = nil
		return nil
	}

	scan := make(Pieces, len(elems))
	for i, elem := range elems {
		piece := Piece{}
		if len(elem) != 2+len(piece.StorageNode) {
			return invalidElementLength{}
		}

		piece.Number = binary.BigEndian.Uint16(elem[0:])
		copy(piece.StorageNode[:], elem[2:])
		scan[i] = piece
	}

	*pieces = scan
	return nil
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
