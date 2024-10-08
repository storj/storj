// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"fmt"

	"github.com/zeebo/xxh3"
)

const (
	rSize         = 64
	pSize         = 4096
	rPerP         = pSize / rSize
	_     uintptr = -(pSize % rSize) // ensure records evenly divide the page size
)

type page [pSize]byte

func (p *page) readRecord(n uint64, rec *record) {
	if b := p[(n*rSize)%pSize:]; len(b) >= rSize {
		rec.read((*[rSize]byte)(b))
	}
}

func (p *page) writeRecord(n uint64, rec record) {
	if b := p[(n*rSize)%pSize:]; len(b) >= rSize {
		rec.write((*[rSize]byte)(b))
	}
}

type expiration uint32

func newExpiration(t uint32, trash bool) expiration {
	if trash {
		return expiration(t<<1 | 1)
	}
	return expiration(t << 1)
}

func (e expiration) set() bool    { return e != 0 }
func (e expiration) trash() bool  { return e&1 == 1 }
func (e expiration) time() uint32 { return uint32(e >> 1) }

func maxExpiration(a, b expiration) expiration {
	if !a.set() || !b.set() {
		return 0
	}
	if a.trash() && !b.trash() { // if a is trash and b is not, keep a
		return a
	}
	if !a.trash() && b.trash() { // if b is trash and a is not, keep b
		return b
	}
	// they are the same, so pick the larger one.
	if a > b {
		return a
	}
	return b
}

type record struct {
	key      Key
	offset   uint64
	log      uint32
	length   uint32
	created  uint32     // 32 bits of days since epoch
	expires  expiration // 31 bits of days since epoch, 1 bit flag for if set because trash
	checksum uint64
}

func (r record) String() string {
	return fmt.Sprintf(
		"{key:%x offset:%d log:%d length:%d created:%d expires:%d trash:%v checksum:%x}",
		r.key, r.offset, r.log, r.length, r.created, r.expires.time(), r.expires.trash(), r.checksum,
	)
}

func recordsEqualish(a, b record) bool {
	a.expires, a.checksum = 0, 0
	b.expires, b.checksum = 0, 0
	return a == b
}

func (r *record) index() uint64 { return keyIndex(&r.key) }

func (r *record) validChecksum() bool { return r.checksum == r.computeChecksum() }
func (r *record) setChecksum()        { r.checksum = r.computeChecksum() }
func (r *record) computeChecksum() uint64 {
	var buf [rSize]byte
	r.write(&buf)

	// reserve a bit of checksum space just in case we need a gross hacky flag in the future.
	return xxh3.Hash(buf[:56]) >> 1
}

func (r *record) write(buf *[rSize]byte) {
	*(*Key)(buf[0:32]) = r.key
	binary.LittleEndian.PutUint64(buf[32:40], r.offset)
	binary.LittleEndian.PutUint32(buf[40:44], r.log)
	binary.LittleEndian.PutUint32(buf[44:48], r.length)
	binary.LittleEndian.PutUint32(buf[48:52], r.created)
	binary.LittleEndian.PutUint32(buf[52:56], uint32(r.expires))
	binary.LittleEndian.PutUint64(buf[56:64], r.checksum)
}

func (r *record) read(buf *[rSize]byte) {
	r.key = *(*Key)(buf[0:32])
	r.offset = binary.LittleEndian.Uint64(buf[32:40])
	r.log = binary.LittleEndian.Uint32(buf[40:44])
	r.length = binary.LittleEndian.Uint32(buf[44:48])
	r.created = binary.LittleEndian.Uint32(buf[48:52])
	r.expires = expiration(binary.LittleEndian.Uint32(buf[52:56]))
	r.checksum = binary.LittleEndian.Uint64(buf[56:64])
}
