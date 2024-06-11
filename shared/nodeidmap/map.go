// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

// Package nodeidmap implements an optimized version of map for storj.NodeID.
package nodeidmap

import (
	"encoding/binary"

	"storj.io/common/storj"
)

type idprefix [4]byte
type idsuffix [storj.NodeIDSize - 4]byte

// Map implements a map[storj.NodeID]Value, which avoids hashing the whole node id.
type Map[Value any] struct {
	entries map[idprefix]*entry[Value]
}

// entry implments a linked list of node id-s and values.
type entry[Value any] struct {
	id    idsuffix
	value Value
	next  *entry[Value]
}

// Make creates a new Map.
func Make[Value any]() Map[Value] {
	var x Map[Value]
	x.entries = make(map[idprefix]*entry[Value])
	return x
}

// MakeSized creates a new Map with the specified size.
func MakeSized[Value any](size int) Map[Value] {
	var x Map[Value]
	x.Reset(size)
	return x
}

// Reset recreates the map.
func (m *Map[Value]) Reset(size int) {
	m.entries = make(map[idprefix]*entry[Value], size)
}

// Clear clears the map.
func (m Map[Value]) Clear() {
	for key := range m.entries {
		delete(m.entries, key)
	}
}

// Store stores the value at id.
func (m Map[Value]) Store(id storj.NodeID, value Value) {
	prefix := idprefix(id[0:4])
	first, ok := m.entries[prefix]
	if ok {
		first.Set(idsuffix(id[4:]), value)
	} else {
		m.entries[prefix] = &entry[Value]{
			id:    idsuffix(id[4:]),
			value: value,
		}
	}
}

// Load loads the value for id.
func (m Map[Value]) Load(id storj.NodeID) (value Value, ok bool) {
	chain := m.entries[idprefix(id[0:4])]
	for ; chain != nil; chain = chain.next {
		if sameSuffix(chain.id, id) {
			return chain.value, true
		}
	}
	return value, false
}

// Modify modifies the value at id.
func (m Map[Value]) Modify(id storj.NodeID, modify func(old Value, ok bool) Value) {
	prefix, suffix := idprefix(id[0:4]), idsuffix(id[4:])

	first, ok := m.entries[prefix]
	if !ok {
		var zero Value
		newvalue := modify(zero, false)

		m.entries[prefix] = &entry[Value]{
			id:    suffix,
			value: newvalue,
		}
		return
	}

	x, ok := first.Find(suffix)
	if !ok {
		var zero Value
		newvalue := modify(zero, false)
		x.next = &entry[Value]{
			id:    suffix,
			value: newvalue,
		}
		return
	}

	x.value = modify(x.value, true)
}

// Range iterates over all the values in the map.
// Callback should return false to stop iteration.
func (m Map[Value]) Range(fn func(k storj.NodeID, v Value) bool) {
	for prefix, entry := range m.entries {
		var k storj.NodeID
		copy(k[:4], prefix[:])

		for ; entry != nil; entry = entry.next {
			copy(k[4:], entry.id[:])

			if ok := fn(k, entry.value); !ok {
				return
			}
		}
	}
}

// Count returns the number of entries in the map.
func (m Map[Value]) Count() (count int) {
	for _, entry := range m.entries {
		for ; entry != nil; entry = entry.next {
			count++
		}
	}
	return count
}

// Clone makes a deep clone of the map.
func (m Map[Value]) Clone() Map[Value] {
	r := MakeSized[Value](len(m.entries))
	for prefix, entry := range m.entries {
		r.entries[prefix] = entry.Clone()
	}
	return r
}

// Add adds xs to the receiver, using combine to add values together
// when those values fall under the same NodeID.
func (m Map[Value]) Add(xs Map[Value], combine func(old, new Value) Value) {
	for prefix, entry := range xs.entries {
		old, ok := m.entries[prefix]
		if !ok {
			m.entries[prefix] = entry.Clone()
			continue
		}

		for ; entry != nil; entry = entry.next {
			oldentry, ok := old.Ensure(entry.id)
			if ok {
				oldentry.value = combine(oldentry.value, entry.value)
			} else {
				oldentry.value = entry.value
			}
		}
	}
}

// AsMap converts Map to a regular Go map.
func (m Map[Value]) AsMap() map[storj.NodeID]Value {
	r := make(map[storj.NodeID]Value, len(m.entries))
	m.Range(func(k storj.NodeID, v Value) bool {
		r[k] = v
		return true
	})
	return r
}

// Find finds the entry with the specified ID. If the entry does not exist,
// it will return the last entry in linked list and false.
func (e *entry[Value]) Find(id idsuffix) (_ *entry[Value], ok bool) {
	for {
		if equalSuffix(e.id, id) {
			return e, true
		}

		if e.next == nil {
			return e, false
		}

		e = e.next
	}
}

// Ensure either adds a new entry or finds entry with the specified id.
func (e *entry[Value]) Ensure(id idsuffix) (*entry[Value], bool) {
	x, ok := e.Find(id)
	if ok {
		return x, true
	}
	x.next = &entry[Value]{id: id}
	return x.next, false
}

// Set adds id or value to the linked list.
func (e *entry[Value]) Set(id idsuffix, value Value) {
	e, _ = e.Ensure(id)
	e.value = value
}

// Clone makes a deep clone of the entry linked list.
func (e *entry[Value]) Clone() *entry[Value] {
	if e == nil {
		return nil
	}

	root := &entry[Value]{
		id:    e.id,
		value: e.value,
	}
	r := root
	e = e.next

	for ; e != nil; r, e = r.next, e.next {
		r.next = &entry[Value]{
			id:    e.id,
			value: e.value,
		}
	}

	return root
}

// AsMap converts entry list into a map.
func (e *entry[Value]) AsMap() map[idsuffix]Value {
	r := map[idsuffix]Value{}
	for ; e != nil; e = e.next {
		r[e.id] = e.value
	}
	return r
}

// equalSuffix is written using binary.LittleEndian to force inlining of the id-s
// and it's written to use binary arithmetic to minimize the branching.
func equalSuffix(a, b idsuffix) bool {
	// This ends up making Load ~20% faster on amd64.
	return binary.LittleEndian.Uint64(a[0:8])^binary.LittleEndian.Uint64(b[0:8])|
		binary.LittleEndian.Uint64(a[8:16])^binary.LittleEndian.Uint64(b[8:16])|
		binary.LittleEndian.Uint64(a[16:24])^binary.LittleEndian.Uint64(b[16:24])|
		uint64(binary.LittleEndian.Uint32(a[24:28])^binary.LittleEndian.Uint32(b[24:28])) == 0
}

// sameSuffix suffix is written using binary.LittleEndian to force inlining of the id-s
// and it's written to use binary arithmetic to minimize the branching.
func sameSuffix(a idsuffix, b storj.NodeID) bool {
	// This approach ends up making Load ~20% faster on amd64.
	return binary.LittleEndian.Uint64(a[0:8])^binary.LittleEndian.Uint64(b[4:12])|
		binary.LittleEndian.Uint64(a[8:16])^binary.LittleEndian.Uint64(b[12:20])|
		binary.LittleEndian.Uint64(a[16:24])^binary.LittleEndian.Uint64(b[20:28])|
		uint64(binary.LittleEndian.Uint32(a[24:28])^binary.LittleEndian.Uint32(b[28:32])) == 0
}
