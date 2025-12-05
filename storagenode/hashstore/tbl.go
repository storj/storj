// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/zeebo/xxh3"

	"storj.io/common/memory"
)

// TableKind is an enumeration of the different table kinds.
type TableKind byte

// String returns a string representation of the table kind.
func (t TableKind) String() string {
	switch t {
	case TableKind_HashTbl:
		return "HashTbl"
	case TableKind_MemTbl:
		return "MemTbl"
	default:
		return fmt.Sprintf("TableKind(%d)", t)
	}
}

const (
	// TableKind_HashTbl is the TableKind for a hashtbl.
	TableKind_HashTbl TableKind = 0

	// TableKind_MemTbl is the TableKind for a memtbl.
	TableKind_MemTbl TableKind = 1

	tbl_headerSize = 4096

	tbl_minLogSlots = 14 // log_2 of number of slots for smallest hash table
	tbl_maxLogSlots = 56 // log_2 of number of slots for largest hash table

	_ int64  = tbl_headerSize + 1<<tbl_maxLogSlots*RecordSize // compiler error if overflows int64
	_ uint64 = 1<<tbl_minLogSlots*RecordSize - bigPageSize    // compiler error if negative
)

// Tbl describes a hash table for a store.
type Tbl interface {
	Handle() *os.File
	LogSlots() uint64
	Header() TblHeader

	Load() float64
	Stats() TblStats

	Range(context.Context, func(context.Context, Record) (bool, error)) error
	Insert(context.Context, Record) (bool, error)
	Lookup(context.Context, Key) (Record, bool, error)
	Sync(context.Context) error
	Close() error
}

// TblConstructor is a constructor for a hash table.
type TblConstructor interface {
	Append(context.Context, Record) (bool, error)
	Done(context.Context) (Tbl, error)
	Cancel()
}

// TblStats contains statistics about a hash table.
type TblStats struct {
	NumSet uint64      // number of set records.
	LenSet memory.Size // sum of lengths in set records.
	AvgSet float64     // average size of length of records.

	NumTrash uint64      // number of set trash records.
	LenTrash memory.Size // sum of lengths in set trash records.
	AvgTrash float64     // average size of length of trash records.

	NumTTL uint64      // number of set records with expiration but not trash.
	LenTTL memory.Size // sum of lengths in set records with expiration but not trash.
	AvgTTL float64     // average size of length of records with expiration but not trash.

	NumSlots  uint64      // total number of records available.
	TableSize memory.Size // total number of bytes in the hash table.
	Load      float64     // percent of slots that are set.

	Created uint32    // date that the hashtbl was created.
	Kind    TableKind // kind of table
}

// TblHeader is the header at the start of a hash table.
type TblHeader struct {
	Created  uint32    // when the hashtbl was created
	HashKey  bool      // if we apply a hash function to the key
	Kind     TableKind // kind of table
	LogSlots uint64    // number of expected logslots
}

func writeBool(x *byte, v bool) {
	if v {
		*x = 1
	} else {
		*x = 0
	}
}

// OpenTable reads the header and opens the appropriate table type.
func OpenTable(ctx context.Context, fh *os.File, cfg Config) (_ Tbl, _ map[uint64]*RecordTail, err error) {
	header, err := ReadTblHeader(fh)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}
	switch header.Kind {
	case TableKind_HashTbl:
		tbl, tails, err := OpenHashTbl(ctx, fh, cfg.Hashtbl)
		if err != nil {
			return nil, nil, err
		}
		return tbl, tails, nil
	case TableKind_MemTbl:
		tbl, tails, err := OpenMemTbl(ctx, fh, cfg.Memtbl)
		if err != nil {
			return nil, nil, err
		}
		return tbl, tails, nil
	default:
		return nil, nil, Error.New("unknown table kind: %d", header.Kind)
	}
}

// CreateTable creates a new table of the given kind.
func CreateTable(ctx context.Context, fh *os.File, logSlots uint64, created uint32, kind TableKind, cfg Config) (_ TblConstructor, err error) {
	switch kind {
	case TableKind_HashTbl:
		return CreateHashTbl(ctx, fh, logSlots, created, cfg.Hashtbl)
	case TableKind_MemTbl:
		return CreateMemTbl(ctx, fh, logSlots, created, cfg.Memtbl)
	default:
		return nil, Error.New("unknown table kind: %d", kind)
	}
}

// WriteTblHeader writes the header page to the file handle.
func WriteTblHeader(fh *os.File, header TblHeader) error {
	var buf [tbl_headerSize]byte

	copy(buf[0:4], "HTBL") // write the magic bytes.

	binary.BigEndian.PutUint32(buf[4:8], header.Created)    // write the created field.
	writeBool(&buf[8], header.HashKey)                      // write the hashKey field.
	buf[9] = byte(header.Kind)                              // write the kind field.
	binary.BigEndian.PutUint64(buf[10:18], header.LogSlots) // write the logSlots field.

	// write the checksum.
	binary.BigEndian.PutUint64(buf[tbl_headerSize-8:tbl_headerSize], xxh3.Hash(buf[:tbl_headerSize-8]))

	// write the header page.
	_, err := fh.WriteAt(buf[:], 0)
	return Error.Wrap(err)
}

// ReadTblHeader reads the header page from the file handle.
func ReadTblHeader(fh *os.File) (header TblHeader, err error) {
	// read the header page.
	var buf [tbl_headerSize]byte
	if _, err := fh.ReadAt(buf[:], 0); err != nil {
		return TblHeader{}, Error.New("unable to read header: %w", err)
	}

	// check the magic bytes.
	if string(buf[0:4]) != "HTBL" {
		return TblHeader{}, Error.New("invalid header: %q", buf[0:4])
	}

	// check the checksum.
	hash := binary.BigEndian.Uint64(buf[tbl_headerSize-8 : tbl_headerSize])
	if computed := xxh3.Hash(buf[:tbl_headerSize-8]); hash != computed {
		return TblHeader{}, Error.New("invalid header checksum: %x != %x", hash, computed)
	}

	header.Created = binary.BigEndian.Uint32(buf[4:8])    // read the created field.
	header.HashKey = buf[8] != 0                          // read the hashKey field.
	header.Kind = TableKind(buf[9])                       // read the kind field.
	header.LogSlots = binary.BigEndian.Uint64(buf[10:18]) // read the logSlots field.

	return header, nil
}
