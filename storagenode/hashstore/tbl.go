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

// Tbl describes a hash table for a store.
type Tbl interface {
	Close()
	Handle() *os.File
	Stats() TblStats
	LogSlots() uint64
	Header() TblHeader
	ComputeEstimates(context.Context) error
	Load() float64
	Range(context.Context, func(context.Context, Record) (bool, error)) error
	ExpectOrdered(context.Context) (func() error, func(), error)
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
}

// TblHeader is the header at the start of a hash table.
type TblHeader struct {
	Created uint32 // when the hashtbl was created
	HashKey bool   // if we apply a hash function to the key
}

// WriteTblHeader writes the header page to the file handle.
func WriteTblHeader(fh *os.File, header TblHeader) error {
	var buf [pageSize]byte

	copy(buf[0:4], "HTBL")
	binary.BigEndian.PutUint32(buf[4:8], header.Created) // write the created field.
	if header.HashKey {
		buf[8] = 1 // write the hashKey field.
	} else {
		buf[8] = 0
	}

	// write the checksum
	binary.BigEndian.PutUint64(buf[pageSize-8:pageSize], xxh3.Hash(buf[:pageSize-8]))

	// write the header page.
	_, err := fh.WriteAt(buf[:], 0)
	return Error.Wrap(err)
}

// ReadTblHeader reads the header page from the file handle.
func ReadTblHeader(fh *os.File) (header TblHeader, err error) {
	// read the magic bytes.
	var buf [pageSize]byte
	if _, err := fh.ReadAt(buf[:], 0); err != nil {
		return TblHeader{}, Error.New("unable to read header: %w", err)
	} else if string(buf[0:4]) != "HTBL" {
		return TblHeader{}, Error.New("invalid header: %q", buf[0:4])
	}

	// check the checksum.
	hash := binary.BigEndian.Uint64(buf[pageSize-8 : pageSize])
	if computed := xxh3.Hash(buf[:pageSize-8]); hash != computed {
		return TblHeader{}, Error.New("invalid header checksum: %x != %x", hash, computed)
	}

	header.Created = binary.BigEndian.Uint32(buf[4:8]) // read the created field.
	header.HashKey = buf[8] != 0                       // read the hashKey field.

	return header, nil
}
