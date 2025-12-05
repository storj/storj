// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/zeebo/xxh3"
)

const (
	// RecordSize is the size of a serialized record in bytes.
	RecordSize = 64

	pageSize       = 512
	recordsPerPage = pageSize / RecordSize

	bigPageSize       = 256 * pageSize
	recordsPerBigPage = bigPageSize / RecordSize

	_ uintptr = -(pageSize % RecordSize) // ensure records evenly divide the page size
)

type bigPage [bigPageSize]byte

func (p *bigPage) readRecord(n uint64, rec *Record) bool {
	if n < recordsPerBigPage {
		return rec.ReadFrom((*[RecordSize]byte)(p[n*RecordSize:]))
	}
	return false
}

func (p *bigPage) writeRecord(n uint64, rec *Record) bool {
	if n < recordsPerBigPage {
		rec.WriteTo((*[RecordSize]byte)(p[n*RecordSize:]))
		return true
	}
	return false
}

type page [pageSize]byte

func (p *page) readRecord(n uint64, rec *Record) bool {
	if n < recordsPerPage {
		return rec.ReadFrom((*[RecordSize]byte)(p[n*RecordSize:]))
	}
	return false
}

func (p *page) writeRecord(n uint64, rec *Record) bool {
	if n < recordsPerPage {
		rec.WriteTo((*[RecordSize]byte)(p[n*RecordSize:]))
		return true
	}
	return false
}

// Expiration is a 23-bit timestamp with a 1-bit flag for trash.
type Expiration uint32

// NewExpiration constructs an Expiration from a timestamp and a trash flag.
func NewExpiration(t uint32, trash bool) Expiration {
	t &= 0x7fffff
	if trash {
		return Expiration(t<<1 | 1)
	}
	return Expiration(t << 1)
}

// Set returns true if the expiration is set: not zero.
func (e Expiration) Set() bool { return e != 0 }

// Trash returns true if the trash bit is set.
func (e Expiration) Trash() bool { return e&1 == 1 }

// Time returns the timestamp part of the expiration.
func (e Expiration) Time() uint32 { return uint32(e >> 1) }

// MaxExpiration returns the larger of two expirations. All expirations with the trash bit set are
// larger than expirations without the trash bit set. If both have the same trash bit setting, then
// the larger of the timestamps is returned. An unset expiration is always largest.
func MaxExpiration(a, b Expiration) Expiration {
	if !a.Set() || !b.Set() {
		return 0
	}
	if a.Trash() && !b.Trash() { // if a is trash and b is not, keep a
		return a
	}
	if !a.Trash() && b.Trash() { // if b is trash and a is not, keep b
		return b
	}
	// they are the same, so pick the larger one.
	if a > b {
		return a
	}
	return b
}

// Record contains metadata about a piece stored in a hash table.
type Record struct {
	Key     Key        // 256 bits (32b) of key
	Offset  uint64     // 48  bits (6b) of offset (256TB max file size)
	Log     uint64     // 64  bits (8b) of log id (effectively unlimited number of logs)
	Length  uint32     // 32  bits (4b) of length (4GB max piece size)
	Created uint32     // 23  bits (3b) of days since epoch (~22900 years), 1 bit reserved
	Expires Expiration // 23  bits (3b) of days since epoch (~22900 years), 1 bit flag for trash
}

// String retruns a string representation of the record.
func (r Record) String() string {
	return fmt.Sprintf(
		"{key:%x offset:%d log:%d length:%d created:%d (%v) expires:%d (%v) trash:%v}",
		r.Key[:],
		r.Offset,
		r.Log,
		r.Length,
		r.Created,
		DateToTime(r.Created).Format(time.DateOnly),
		r.Expires.Time(),
		DateToTime(r.Expires.Time()).Format(time.DateOnly),
		r.Expires.Trash(),
	)
}

// IsZero returns true if the record is the zero value.
func (r Record) IsZero() bool {
	return r == Record{}
}

// RecordsEqualish returns true if the records are equalish. Records are equalish if they are equal
// except for the expires time.
func RecordsEqualish(a, b Record) bool {
	a.Expires, b.Expires = 0, 0
	return a == b
}

func checksumBuffer(buf *[RecordSize]byte) uint64 { return xxh3.Hash(buf[0:56]) >> 1 }

// WriteTo stores the record and its checksum into the buffer.
func (r *Record) WriteTo(buf *[RecordSize]byte) {
	*(*Key)(buf[0:32]) = r.Key
	binary.LittleEndian.PutUint64(buf[32:32+8], r.Offset&0xffffffffffff)
	binary.LittleEndian.PutUint64(buf[38:38+8], r.Log&0xffffffffffffffff)
	binary.LittleEndian.PutUint32(buf[46:46+4], r.Length&0xffffffff)
	binary.LittleEndian.PutUint32(buf[50:50+4], r.Created&0x7fffff)
	binary.LittleEndian.PutUint32(buf[53:53+4], uint32(r.Expires)&0xffffff)
	binary.LittleEndian.PutUint64(buf[56:56+8], checksumBuffer(buf))
}

// ReadFrom updates the record with the values from the buffer and returns true if the checksum is
// valid.
func (r *Record) ReadFrom(buf *[RecordSize]byte) bool {
	r.Key = *(*Key)(buf[0:32])
	r.Offset = binary.LittleEndian.Uint64(buf[32:32+8]) & 0xffffffffffff
	r.Log = binary.LittleEndian.Uint64(buf[38:38+8]) & 0xffffffffffffffff
	r.Length = binary.LittleEndian.Uint32(buf[46:46+4]) & 0xffffffff
	r.Created = binary.LittleEndian.Uint32(buf[50:50+4]) & 0xffffff
	r.Expires = Expiration(binary.LittleEndian.Uint32(buf[53:53+4]) & 0xffffff)
	return binary.LittleEndian.Uint64(buf[56:56+8]) == checksumBuffer(buf)
}

// RecordTail is a small fixed-size array of records used to track the most recent records in a log.
type RecordTail [5]Record

// Push adds a record to the tail if it has a larger offset then any record already in the tail.
func (r *RecordTail) Push(rec Record) {
	mi, moff := 0, r[0].Offset
	for i := 1; i < len(r); i++ {
		if r[i].IsZero() || r[i].Offset < moff {
			mi, moff = i, r[i].Offset
		}
	}
	if rec.Offset >= moff {
		r[mi] = rec
	}
}

// Sort sorts the records in the tail by offset, breaking ties by key.
func (r *RecordTail) Sort() {
	sort.Slice(r[:], func(i, j int) bool {
		switch {
		case r[i].Offset > r[j].Offset:
			return true
		case r[i].Offset < r[j].Offset:
			return false
		default:
			return string(r[i].Key[:]) > string(r[j].Key[:])
		}
	})
}
