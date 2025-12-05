// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

// Package nodeidmap implements an optimized version of map for storj.NodeID.
package nodeidmap

import "storj.io/common/storj"

// Map implements a map[storj.NodeID]Value with some useful methods.
type Map[Value any] struct {
	entries map[storj.NodeID]Value
}

// Make creates a new Map.
func Make[Value any]() Map[Value] {
	var x Map[Value]
	x.entries = make(map[storj.NodeID]Value)
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
	m.entries = make(map[storj.NodeID]Value, size)
}

// IsEmpty returns true when there are no entries in the map.
func (m Map[Value]) IsEmpty() bool { return len(m.entries) == 0 }

// Clear clears the map.
func (m Map[Value]) Clear() {
	for key := range m.entries {
		delete(m.entries, key)
	}
}

// Store stores the value at id.
func (m Map[Value]) Store(id storj.NodeID, value Value) {
	m.entries[id] = value
}

// Load loads the value for id.
func (m Map[Value]) Load(id storj.NodeID) (value Value, ok bool) {
	value, ok = m.entries[id]
	return value, ok
}

// Modify modifies the value at id.
func (m Map[Value]) Modify(id storj.NodeID, modify func(old Value, ok bool) Value) {
	old, ok := m.entries[id]
	if !ok {
		var zero Value
		m.entries[id] = modify(zero, false)
		return
	}
	m.entries[id] = modify(old, true)
}

// Range iterates over all the values in the map.
// Callback should return false to stop iteration.
func (m Map[Value]) Range(fn func(k storj.NodeID, v Value) bool) {
	for id, value := range m.entries {
		if ok := fn(id, value); !ok {
			return
		}
	}
}

// Count returns the number of entries in the map.
func (m Map[Value]) Count() (count int) {
	return len(m.entries)
}

// Clone makes a deep clone of the map.
func (m Map[Value]) Clone() Map[Value] {
	r := MakeSized[Value](len(m.entries))
	for id, value := range m.entries {
		r.entries[id] = value
	}
	return r
}

// Add adds xs to the receiver, using combine to add values together
// when those values fall under the same NodeID.
func (m Map[Value]) Add(xs Map[Value], combine func(old, new Value) Value) {
	for id, value := range xs.entries {
		old, ok := m.entries[id]
		if !ok {
			m.entries[id] = value
			continue
		}

		m.entries[id] = combine(old, value)
	}
}

// AsMap converts Map to a regular Go map.
func (m Map[Value]) AsMap() map[storj.NodeID]Value {
	clone := m.Clone()
	return clone.entries
}
