// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"encoding/binary"
	"os"

	"github.com/zeebo/xxh3"

	"storj.io/common/memory"
)

const (
	kind_HashTbl = 0
	kind_MemTbl  = 1

	headerSize = 4096

	tbl_minLogSlots = 14 // log_2 of number of slots for smallest hash table
	tbl_maxLogSlots = 56 // log_2 of number of slots for largest hash table

	_ int64  = headerSize + 1<<tbl_maxLogSlots*RecordSize  // compiler error if overflows int64
	_ uint64 = 1<<tbl_minLogSlots*RecordSize - bigPageSize // compiler error if negative
)

// Tbl describes a hash table for a store.
type Tbl interface {
	Close()

	Handle() *os.File
	Stats() TblStats
	LogSlots() uint64
	Header() TblHeader
	CompactLoad() float64
	MaxLoad() float64
	Load() float64

	ComputeEstimates(context.Context) error
	ExpectOrdered(context.Context) (func() error, func(), error)

	Range(context.Context, func(context.Context, Record) (bool, error)) error
	Insert(context.Context, Record) (bool, error)
	Lookup(context.Context, Key) (Record, bool, error)
}

// TblStats contains statistics about a hash table.
type TblStats struct {
	NumSet uint64      // number of set records.
	LenSet memory.Size // sum of lengths in set records.
	AvgSet float64     // average size of length of records.

	NumTrash uint64      // number of set trash records.
	LenTrash memory.Size // sum of lengths in set trash records.
	AvgTrash float64     // average size of length of trash records.

	NumSlots  uint64      // total number of records available.
	TableSize memory.Size // total number of bytes in the hash table.
	Load      float64     // percent of slots that are set.

	Created uint32 // date that the hashtbl was created.
	Kind    uint8  // kind of table
}

// TblHeader is the header at the start of a hash table.
type TblHeader struct {
	Created  uint32 // when the hashtbl was created
	HashKey  bool   // if we apply a hash function to the key
	Kind     byte   // kind of table
	LogSlots uint64 // number of expected logslots
}

func writeBool(x *byte, v bool) {
	if v {
		*x = 1
	} else {
		*x = 0
	}
}

// WriteTblHeader writes the header page to the file handle.
func WriteTblHeader(fh *os.File, header TblHeader) error {
	var buf [headerSize]byte

	copy(buf[0:4], "HTBL") // write the magic bytes.

	binary.BigEndian.PutUint32(buf[4:8], header.Created)    // write the created field.
	writeBool(&buf[8], header.HashKey)                      // write the hashKey field.
	buf[9] = header.Kind                                    // write the kind field.
	binary.BigEndian.PutUint64(buf[10:18], header.LogSlots) // write the logSlots field.

	// write the checksum.
	binary.BigEndian.PutUint64(buf[headerSize-8:headerSize], xxh3.Hash(buf[:headerSize-8]))

	// write the header page.
	_, err := fh.WriteAt(buf[:], 0)
	return Error.Wrap(err)
}

// ReadTblHeader reads the header page from the file handle.
func ReadTblHeader(fh *os.File) (header TblHeader, err error) {
	// read the header page.
	var buf [headerSize]byte
	if _, err := fh.ReadAt(buf[:], 0); err != nil {
		return TblHeader{}, Error.New("unable to read header: %w", err)
	}

	// check the magic bytes.
	if string(buf[0:4]) != "HTBL" {
		return TblHeader{}, Error.New("invalid header: %q", buf[0:4])
	}

	// check the checksum.
	hash := binary.BigEndian.Uint64(buf[headerSize-8 : headerSize])
	if computed := xxh3.Hash(buf[:headerSize-8]); hash != computed {
		return TblHeader{}, Error.New("invalid header checksum: %x != %x", hash, computed)
	}

	header.Created = binary.BigEndian.Uint32(buf[4:8])    // read the created field.
	header.HashKey = buf[8] != 0                          // read the hashKey field.
	header.Kind = buf[9]                                  // read the kind field.
	header.LogSlots = binary.BigEndian.Uint64(buf[10:18]) // read the logSlots field.

	return header, nil
}
