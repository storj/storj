// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"math/bits"
	"unsafe"
)

type shortKey [5]byte

func shortKeyFrom(k Key) shortKey {
	return *(*shortKey)(k[len(Key{})-len(shortKey{}):])
}

func (sk *shortKey) control() byte { return 1<<7 | sk[4] }

//
//
//

type flatGroupEntry [5 + 4]byte

func (ge *flatGroupEntry) key() *shortKey { return (*shortKey)(ge[:5]) }
func (ge *flatGroupEntry) val() *[4]byte  { return (*[4]byte)(ge[5:]) }

//
//
//

const (
	groupHeader_LSB = 0x0101010101010101
	groupHeader_MSB = 0x8080808080808080
)

type flatGroupHeader [8]byte

func (gh *flatGroupHeader) controls() uint64 {
	return binary.LittleEndian.Uint64(gh[:])
}

func (gh *flatGroupHeader) count() byte {
	return byte(bits.OnesCount64(gh.controls() & groupHeader_MSB))
}

func (gh *flatGroupHeader) appendEntry(control byte) (byte, bool) {
	if count := gh.count(); int(count) < len(gh) {
		gh[count] = control
		return count, true
	}
	return 0, false
}

//
//
//

type flatGroupEntries [80 - 8]byte

func (ge *flatGroupEntries) entry(i byte) *flatGroupEntry { return (*flatGroupEntry)(ge[int(i)*9:]) }

//
//
//

type flatGroup [80]byte

func (g *flatGroup) header() *flatGroupHeader   { return (*flatGroupHeader)(g[:8]) }
func (g *flatGroup) entries() *flatGroupEntries { return (*flatGroupEntries)(g[8:]) }

func (g *flatGroup) find(key shortKey) (*flatGroupEntry, bool) {
	control := key.control()

	header := g.header()
	entries := g.entries()

	controls := header.controls()
	matches := controls ^ (groupHeader_LSB * uint64(control))
	bitset := ((matches - groupHeader_LSB) &^ matches) & groupHeader_MSB

	for bitset != 0 {
		ent := entries.entry(byte(bits.TrailingZeros64(bitset) / 8))

		if *ent.key() == key {
			return ent, true
		}

		bitset &= bitset - 1
	}

	if count := header.count(); count < 8 {
		return entries.entry(count), false
	}

	return nil, false
}

//
//
//

type flatMap struct {
	buf    []byte
	groups []flatGroup
}

func flatMapSize(numElements int) int {
	return ((numElements + 8 - 1) / 8) * len(flatGroup{})
}

func newFlatMap(buf []byte) *flatMap {
	return &flatMap{
		buf: buf,
		groups: unsafe.Slice(
			(*flatGroup)(unsafe.Pointer(unsafe.SliceData(buf))),
			len(buf)/len(flatGroup{}),
		),
	}
}

type flatOp struct {
	group *flatGroup      // where the control word should be updated/inserted
	entry *flatGroupEntry // where the key should be updated/inserted
	key   shortKey        // key that was found
	match bool            // if there was a match
}

func (op flatOp) Valid() bool    { return op.entry != nil }
func (op flatOp) Exists() bool   { return op.match }
func (op flatOp) Value() [4]byte { return *op.entry.val() }

// set is written ugly and manually inlines to appease the inliner.
func (op flatOp) set(val [4]byte) {
	if !op.match {
		*op.entry.key() = op.key
		op.group.header().appendEntry(1<<7 | op.key[4])
	}
	*op.entry.val() = val
}

func (m *flatMap) find(key shortKey) (op flatOp) {
	op.key = key
	hash := uint64(binary.LittleEndian.Uint32(key[0:4])) | uint64(key[4])<<32
	groups := m.groups
	numGroups := uint64(len(groups))
	for offset := uint64(0); offset < numGroups; offset++ {
		op.group = &groups[(hash+offset)%numGroups]
		op.entry, op.match = op.group.find(key)
		if op.entry != nil {
			return
		}
	}
	return
}
