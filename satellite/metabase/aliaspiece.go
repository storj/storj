// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/binary"
	"reflect"
)

// AliasPieces is a slice of AliasPiece.
type AliasPieces []AliasPiece

// AliasPiece is a piece with alias node ID.
type AliasPiece struct {
	Number uint16
	Alias  NodeAlias
}

const (
	// aliasPieceEncodingRLE run length encodes the zeros and node ID-s.
	//
	// Example:
	//   pieces = {2 x} {11 y}
	//   // converted into slice with zeros
	//   0 0 x 0 0 0 0 0 0 0 0 y
	//   // run length encoded
	//   <2 zeros, 1 value> x <7 zeros, 0 values> <1 zeros, 1 value> y
	aliasPieceEncodingRLE = 1

	aliasPieceEncodingZeroBits       = 3
	aliasPieceEncodingNodeAliasBits  = 8 - aliasPieceEncodingZeroBits
	aliasPieceEncodingMaxZeros       = 1<<aliasPieceEncodingZeroBits - 1
	aliasPieceEncodingMaxNodeAliases = 1<<aliasPieceEncodingNodeAliasBits - 1
)

// Bytes compresses alias pieces to a slice of bytes.
func (aliases AliasPieces) Bytes() ([]byte, error) {
	if len(aliases) == 0 {
		return nil, nil
	}

	var buffer [binary.MaxVarintLen64]byte

	// we're going to guess that it'll take 3 bytes per node alias + at most one per two nodes.
	data := make([]byte, 0, len(aliases)*3+len(aliases)/2)
	data = append(data, aliasPieceEncodingRLE)

	expectedPieceNumber := uint16(0)

	index := 0
	for index < len(aliases) {
		data = append(data, 0)

		// setup header for the next sequence of nodes
		lengthHeaderPos := len(data) - 1
		zeroCount, aliasCount := 0, 0
		setHeader := func() {
			data[lengthHeaderPos] = byte(aliasCount)<<aliasPieceEncodingZeroBits | byte(zeroCount)
		}

		// start examining the piece
		piece := aliases[index]
		if expectedPieceNumber > piece.Number {
			return nil, Error.New("alias pieces not ordered")
		}

		// count up until max zeros
		for i := 0; i < aliasPieceEncodingMaxZeros; i++ {
			if expectedPieceNumber == piece.Number {
				break
			}
			zeroCount++
			expectedPieceNumber++
		}

		// if there were too many zeros in sequence, we need to emit more headers
		if piece.Number != expectedPieceNumber {
			setHeader()
			continue
		}

		// emit all the pieces that are in sequence, but up to max node aliases
		for aliasCount < aliasPieceEncodingMaxNodeAliases {
			// emit the piece alias
			n := binary.PutUvarint(buffer[:], uint64(piece.Alias))
			data = append(data, buffer[:n]...)

			// update the header and the expected piece number
			aliasCount++
			expectedPieceNumber++

			// next piece
			index++
			if index >= len(aliases) {
				break
			}
			piece = aliases[index]

			// check whether we should emit zeros
			if piece.Number != expectedPieceNumber {
				break
			}
		}
		setHeader()
	}

	return data, nil
}

// SetBytes decompresses alias pieces from a slice of bytes.
func (aliases *AliasPieces) SetBytes(data []byte) error {
	if len(data) == 0 {
		*aliases = nil
		return nil
	}
	if data[0] != aliasPieceEncodingRLE {
		*aliases = nil
		return Error.New("unknown alias pieces header: %v", data[0])
	}

	if cap(*aliases) == 0 {
		// we're going to guess there's one alias pieces per two bytes of data
		*aliases = make(AliasPieces, 0, len(data)/2)
	} else {
		// if we have initial capacity, we can reuse the slice
		// and avoid the allocation
		*aliases = (*aliases)[:0]
	}

	p := 1
	pieceNumber := uint16(0)
	for p < len(data) {
		// read the header
		header := data[p]
		p++
		if p >= len(data) {
			return Error.New("invalid alias pieces data")
		}

		// extract header values
		aliasCount := int(header >> aliasPieceEncodingZeroBits)
		zeroCount := int(header & aliasPieceEncodingMaxZeros)

		// skip over the zero values
		pieceNumber += uint16(zeroCount)

		// read the aliases
		for k := 0; k < aliasCount; k++ {
			v, n := binary.Uvarint(data[p:])
			p += n
			if n <= 0 {
				return Error.New("invalid alias pieces data")
			}
			*aliases = append(*aliases, AliasPiece{
				Number: pieceNumber,
				Alias:  NodeAlias(v),
			})
			pieceNumber++
		}
	}

	return nil
}

// Scan implements the database/sql Scanner interface.
func (aliases *AliasPieces) Scan(src any) error {
	if src == nil {
		*aliases = nil
		return nil
	}
	if reflect.ValueOf(src).IsNil() {
		*aliases = nil
		return nil
	}

	switch src := src.(type) {
	case []byte:
		return aliases.SetBytes(src)
	default:
		return Error.New("invalid type for AliasPieces: %T", src)
	}
}

// Value implements the database/sql/driver Valuer interface.
func (aliases AliasPieces) Value() (driver.Value, error) {
	return aliases.Bytes()
}

// DecodeSpanner implements spanner.Decoder.
func (aliases *AliasPieces) DecodeSpanner(val any) (err error) {
	// TODO(spanner) why spanner returns BYTES as base64
	if v, ok := val.(string); ok {
		var buffer [256 + 128]byte
		decoded, err := base64.StdEncoding.AppendDecode(buffer[:0], []byte(v))
		if err != nil {
			return err
		}
		return aliases.SetBytes(decoded)
	}
	return aliases.Scan(val)
}

// EncodeSpanner implements spanner.Encoder.
func (aliases AliasPieces) EncodeSpanner() (any, error) {
	return aliases.Value()
}

// EqualAliasPieces compares whether xs and ys are equal.
func EqualAliasPieces(xs, ys AliasPieces) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i, x := range xs {
		if ys[i] != x {
			return false
		}
	}
	return true
}
