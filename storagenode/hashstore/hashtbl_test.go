// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"math/rand"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestHashtbl_BasicOperation(t *testing.T) {
	const lrec = 14 // 16k records

	h := newTestHashtbl(t, lrec)
	defer h.Close()

	var keys []Key
	var expLength uint64
	for i := 0; i < 1<<lrec/2; i++ {
		// insert the record.
		r := h.AssertInsert()

		// keep track of the key that was used.
		keys = append(keys, r.key)
		expLength += uint64(r.length)

		// we should be able to find it.
		h.AssertLookup(r.key)

		// reinsert should be fine.
		ok, err := h.Insert(r)
		assert.NoError(t, err)
		assert.True(t, ok)

		// we should still be able to find it.
		h.AssertLookup(r.key)
	}

	assert.Equal(t, h.Load(), 0.5)
	nset, length := h.Estimates()
	assert.Equal(t, nset, 1<<lrec/2)
	assert.Equal(t, length, expLength)

	// reopen the hash table and search again
	h.AssertReopen()
	defer h.Close()

	// shuffle the keys so that reads are not in the same order this time
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	for _, k := range keys {
		assert.Equal(t, h.AssertLookup(k), newRecord(k))
	}

	// insert, lookup, and range should fail after close.
	h.Close()

	_, _, err := h.Lookup(newKey())
	assert.Error(t, err)

	_, err = h.Insert(newRecord(newKey()))
	assert.Error(t, err)

	h.Range(func(_ record, err error) bool {
		assert.Error(t, err)
		return false
	})
}

func TestHashtbl_Full(t *testing.T) {
	const lrec = 10 // 1k records

	h := newTestHashtbl(t, lrec)
	defer h.Close()

	// fill the table completely.
	for i := 0; i < 1<<lrec; i++ {
		h.AssertInsert()
	}
	assert.Equal(t, h.Load(), 1.0)

	// inserting a new record should fail.
	ok, err := h.Insert(newRecord(newKey()))
	assert.NoError(t, err)
	assert.False(t, ok)

	// looking up a key that does not exist should fail.
	_, ok, err = h.Lookup(newKey())
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestHashtbl_LostPage(t *testing.T) {
	const lrec = 8 // 256 records (4 pages)

	assert.Equal(t, 1<<lrec, 4*rPerP) // ensure it's 4 pages.

	h := newTestHashtbl(t, lrec)
	defer h.Close()

	// create two keys that collide at the end of the first page.
	k0 := Key{0: rPerP - 1}
	k1 := Key{0: rPerP - 1, 31: 1}

	// ensure the index for the first key is the last record of the first page.
	pi0, ri0 := h.index(keyIndex(&k0))
	assert.Equal(t, pi0, 0)
	assert.Equal(t, ri0, rPerP-1)

	// ensure the index for the second key is the same.
	pi1, ri1 := h.index(keyIndex(&k1))
	assert.Equal(t, pi1, 0)
	assert.Equal(t, ri1, rPerP-1)

	// insert the first key.
	ok, err := h.Insert(newRecord(k0))
	assert.NoError(t, err)
	assert.True(t, ok)
	h.AssertLookup(k0)

	// collide it with the second key.
	ok, err = h.Insert(newRecord(k1))
	assert.NoError(t, err)
	assert.True(t, ok)
	h.AssertLookup(k1)

	// zero out the first page manually and invalidate the page cache.
	_, err = h.fh.WriteAt(make([]byte, pSize), 0)
	assert.NoError(t, err)
	h.invalidatePageCache()

	// we should still be able to read the second key.
	h.AssertLookup(k1)

	// the first key, though, is gone.
	_, ok, err = h.Lookup(k0)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestHashtbl_LRecTooSmall(t *testing.T) {
	_, err := newHashTbl(nil, 5, false)
	assert.Error(t, err)
}

func TestHashtbl_OverwriteMergeRecords(t *testing.T) {
	h := newTestHashtbl(t, 10)
	defer h.Close()

	// create a new record with a non-zero expiration.
	rec := newRecord(newKey())
	rec.expires = newExpiration(1, true)

	// insert the record.
	ok, err := h.Insert(rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record.
	got, ok, err := h.Lookup(rec.key)
	assert.NoError(t, err)
	assert.True(t, ok)
	got.checksum = rec.checksum // ignore the checksum field for equality.
	assert.Equal(t, got, rec)

	// set the expiration to 0 and overwrite. this should be allowed.
	rec.expires = 0
	ok, err = h.Insert(rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with no expiration because that's a larger expiration.
	got, ok, err = h.Lookup(rec.key)
	assert.NoError(t, err)
	assert.True(t, ok)
	got.checksum = rec.checksum // ignore the checksum field for equality.
	assert.Equal(t, got, rec)

	// we should not be able to overwrite the record with a different log file.
	rec.log++
	_, err = h.Insert(rec)
	assert.Error(t, err)

	// we should be able to try to overwrite the record with a smaller expiration
	rec.log--
	rec.expires = newExpiration(2, true)
	ok, err = h.Insert(rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with the larger expiration.
	got2, ok, err := h.Lookup(rec.key)
	assert.NoError(t, err)
	assert.True(t, ok)
	got2.checksum = got.checksum // ignore the checksum field for equality.
	assert.Equal(t, got2, got)
}

func TestHashtbl_RangeExitEarly(t *testing.T) {
	h := newTestHashtbl(t, 10)

	// insert some records to range over.
	for i := 0; i < 100; i++ {
		h.AssertInsert()
	}

	// only iterate through 10 records and then exit early.
	n := 0
	h.Range(func(r record, err error) bool {
		n++
		return n < 10
	})
}

//
// benchmarks
//

func BenchmarkHashtbl(b *testing.B) {
	benchmarkLRecs(b, "Lookup", func(b *testing.B, lrec uint64) {
		h := newTestHashtbl(b, lrec)
		defer h.Close()

		var keys []Key
		for i := 0; i < 1<<lrec/2; i++ {
			rec := h.AssertInsert()
			keys = append(keys, rec.key)
		}

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			h.AssertLookup(keys[mwc.Intn(len(keys))])
		}

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "keys/sec")
	})

	benchmarkLRecs(b, "Insert", func(b *testing.B, lrec uint64) {
		inserts := 1 << lrec / 2

		h := newTestHashtbl(b, lrec)
		defer h.Close()

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			assert.NoError(b, h.fh.Truncate(0))
			assert.NoError(b, h.fh.Truncate(1<<lrec*rSize))
			h.AssertReopen()

			for i := 0; i < inserts; i++ {
				h.AssertInsert()
			}
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
	})

	benchmarkLRecs(b, "Compact", func(b *testing.B, lrec uint64) {
		inserts := 1 << lrec / 2

		h := newTestHashtbl(b, lrec)
		defer h.Close()

		for i := 0; i < inserts; i++ {
			h.AssertInsert()
		}
		var recs []record
		h.Range(func(rec record, err error) bool {
			assert.NoError(b, err)
			recs = append(recs, rec)
			return true
		})

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			h := newTestHashtbl(b, lrec+1)
			for _, rec := range recs {
				h.AssertInsertRecord(rec)
			}
			h.Close()
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
	})
}
