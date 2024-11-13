// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/zeebo/xxh3"
)

const (
	// RSize is the size of a serialized record in bytes.
	RSize = 64

	pSize = 4096
	rPerP = pSize / RSize

	_ uintptr = -(pSize % RSize) // ensure records evenly divide the page size
)

type page [pSize]byte

func (p *page) readRecord(n uint64, rec *Record) bool {
	if b := p[(n*RSize)%pSize:]; len(b) >= RSize {
		return rec.Read((*[RSize]byte)(b))
	}
	return false
}

func (p *page) writeRecord(n uint64, rec Record) {
	if b := p[(n*RSize)%pSize:]; len(b) >= RSize {
		rec.Write((*[RSize]byte)(b))
	}
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
		dateToTime(r.Created).Format(time.DateOnly),
		r.Expires.Time(),
		dateToTime(r.Expires.Time()).Format(time.DateOnly),
		r.Expires.Trash(),
	)
}

// RecordsEqualish returns true if the records are equalish. Records are equalish if they are equal
// except for the expires time and checksums.
func RecordsEqualish(a, b Record) bool {
	a.Expires, b.Expires = 0, 0
	return a == b
}

func checksumBuffer(buf *[RSize]byte) uint64 { return xxh3.Hash(buf[0:56]) >> 1 }

// Write stores the record and its checksum into the buffer.
func (r *Record) Write(buf *[RSize]byte) {
	*(*Key)(buf[0:32]) = r.Key
	binary.LittleEndian.PutUint64(buf[32:32+8], r.Offset&0xffffffffffff)
	binary.LittleEndian.PutUint64(buf[38:38+8], r.Log&0xffffffffffffffff)
	binary.LittleEndian.PutUint32(buf[46:46+4], r.Length&0xffffffff)
	binary.LittleEndian.PutUint32(buf[50:50+4], r.Created&0x7fffff)
	binary.LittleEndian.PutUint32(buf[53:53+4], uint32(r.Expires)&0xffffff)
	binary.LittleEndian.PutUint64(buf[56:56+8], checksumBuffer(buf))
}

// Read updates the record with the values from the buffer and returns true if the checksum is
// valid.
func (r *Record) Read(buf *[RSize]byte) bool {
	r.Key = *(*Key)(buf[0:32])
	r.Offset = binary.LittleEndian.Uint64(buf[32:32+8]) & 0xffffffffffff
	r.Log = binary.LittleEndian.Uint64(buf[38:38+8]) & 0xffffffffffffffff
	r.Length = binary.LittleEndian.Uint32(buf[46:46+4]) & 0xffffffff
	r.Created = binary.LittleEndian.Uint32(buf[50:50+4]) & 0xffffff
	r.Expires = Expiration(binary.LittleEndian.Uint32(buf[53:53+4]) & 0xffffff)
	return binary.LittleEndian.Uint64(buf[56:56+8]) == checksumBuffer(buf)
}
