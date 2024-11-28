// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"sort"
	"testing"

	"github.com/zeebo/assert"
)

func TestRecord_MaxExpiration(t *testing.T) {
	es := []Expiration{
		NewExpiration(10, false),
		NewExpiration(20, false),
		NewExpiration(5, true),
		NewExpiration(15, true),
		NewExpiration(25, true),
		Expiration(0),
	}

	// assert that the slice is in order from smallest to largest.
	assert.That(t, sort.SliceIsSorted(es, func(i, j int) bool {
		return es[j] == MaxExpiration(es[i], es[j]) && es[i] != es[j]
	}))

	// this implies the max between any two elements is the later one.
	for i, ei := range es {
		for _, ej := range es[i:] {
			assert.Equal(t, ej, MaxExpiration(ei, ej))
			assert.Equal(t, ej, MaxExpiration(ej, ei))
		}
	}
}

func TestPage_BasicOperation(t *testing.T) {
	var p page

	var recs []Record
	for i := uint64(0); i < recordsPerPage; i++ {
		rec := newRecord(newKey())
		recs = append(recs, rec)
		p.writeRecord(i, rec)
	}

	for i := uint64(0); i < recordsPerPage; i++ {
		var tmp Record
		p.readRecord(i, &tmp)
		assert.Equal(t, tmp, recs[i])
	}
}
