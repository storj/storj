// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"runtime"
	"sort"
	"testing"

	"github.com/zeebo/assert"
)

func TestRecord_MaxExpiration(t *testing.T) {
	es := []expiration{
		newExpiration(10, false),
		newExpiration(20, false),
		newExpiration(5, true),
		newExpiration(15, true),
		newExpiration(25, true),
		expiration(0),
	}

	// assert that the slice is in order from smallest to largest.
	assert.That(t, sort.SliceIsSorted(es, func(i, j int) bool {
		return es[j] == maxExpiration(es[i], es[j]) && es[i] != es[j]
	}))

	// this implies the max between any two elements is the later one.
	for i, ei := range es {
		for _, ej := range es[i:] {
			assert.Equal(t, ej, maxExpiration(ei, ej))
			assert.Equal(t, ej, maxExpiration(ej, ei))
		}
	}

}

func TestPage_BasicOperation(t *testing.T) {
	var p page

	var recs []record
	for i := uint64(0); i < rPerP; i++ {
		rec := newRecord(newKey())
		recs = append(recs, rec)
		p.writeRecord(i, rec)
	}

	for i := uint64(0); i < rPerP; i++ {
		var tmp record
		p.readRecord(i, &tmp)
		assert.Equal(t, tmp, recs[i])
	}
}

//
// benchmarks
//

func BenchmarkChecksum(b *testing.B) {
	b.ReportAllocs()

	var rec record
	var h uint64
	for i := 0; i < b.N; i++ {
		h += rec.computeChecksum()
	}
	runtime.KeepAlive(h)
}
