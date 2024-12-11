// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestHashtbl_BasicOperation(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	var keys []Key
	var expLength uint64
	for i := 0; i < 1<<hashtbl_minLogSlots/2; i++ {
		// insert the record.
		r := h.AssertInsert()

		// keep track of the key that was used.
		keys = append(keys, r.Key)
		expLength += uint64(r.Length)

		// we should be able to find it.
		h.AssertLookup(r.Key)

		// reinsert should be fine.
		ok, err := h.Insert(r)
		assert.NoError(t, err)
		assert.True(t, ok)

		// we should still be able to find it.
		h.AssertLookup(r.Key)
	}

	assert.Equal(t, h.Load(), 0.5)
	stats := h.Stats()
	assert.Equal(t, stats.NumSet, 1<<hashtbl_minLogSlots/2)
	assert.Equal(t, stats.LenSet, expLength)

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

	h.Range(func(_ Record, err error) bool {
		assert.Error(t, err)
		return false
	})
}

func TestHashtbl_TrashStats(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	rec := newRecord(newKey())
	rec.Expires = NewExpiration(1, true)
	h.AssertInsertRecord(rec)

	stats := h.Stats()
	assert.Equal(t, stats.NumTrash, 1)
	assert.Equal(t, stats.LenTrash, rec.Length)
	assert.Equal(t, stats.AvgTrash, float64(rec.Length))
}

func TestHashtbl_Full(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	// fill the table completely.
	for i := 0; i < 1<<hashtbl_minLogSlots; i++ {
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
	const lrec = 14 // 16k records (256 pages)

	h := newTestHashtbl(t, lrec)
	defer h.Close()

	// create two keys that collide at the end of the first page.
	k0 := newKeyAt(h.HashTbl, 0, recordsPerPage-1, 0)
	k1 := newKeyAt(h.HashTbl, 0, recordsPerPage-1, 1)

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
	_, err = h.fh.WriteAt(make([]byte, pageSize), pageSize) // offset=pSize to skip the header page.
	assert.NoError(t, err)

	// we should still be able to read the second key.
	h.AssertLookup(k1)

	// the first key, though, is gone.
	_, ok, err = h.Lookup(k0)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestHashtbl_SmallFileSizes(t *testing.T) {
	fh, err := os.Create(filepath.Join(t.TempDir(), "tmp"))
	assert.NoError(t, err)
	defer func() { _ = fh.Close() }()

	_, err = OpenHashtbl(fh)
	assert.Error(t, err)

	assert.NoError(t, fh.Truncate(pageSize))
	_, err = OpenHashtbl(fh)
	assert.Error(t, err)

	assert.NoError(t, fh.Truncate(pageSize+(pageSize-1)))
	_, err = OpenHashtbl(fh)
	assert.Error(t, err)
}

func TestHashtbl_OverwriteMergeRecords(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	// create a new record with a non-zero expiration.
	rec := newRecord(newKey())
	rec.Expires = NewExpiration(1, true)

	// insert the record.
	ok, err := h.Insert(rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record.
	got, ok, err := h.Lookup(rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got, rec)

	// set the expiration to 0 and overwrite. this should be allowed.
	rec.Expires = 0
	ok, err = h.Insert(rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with no expiration because that's a larger expiration.
	got, ok, err = h.Lookup(rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got, rec)

	// we should not be able to overwrite the record with a different log file.
	rec.Log++
	_, err = h.Insert(rec)
	assert.Error(t, err)

	// we should be able to try to overwrite the record with a smaller expiration
	rec.Log--
	rec.Expires = NewExpiration(2, true)
	ok, err = h.Insert(rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with the larger expiration.
	got2, ok, err := h.Lookup(rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got2, got)
}

func TestHashtbl_RangeExitEarly(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	// insert some records to range over.
	for i := 0; i < 100; i++ {
		h.AssertInsert()
	}

	// only iterate through 10 records and then exit early.
	n := 0
	h.Range(func(r Record, err error) bool {
		n++
		return n < 10
	})
}

func TestHashtbl_LRecBounds(t *testing.T) {
	_, err := CreateHashtbl(nil, hashtbl_maxLogSlots+1, 0)
	assert.Error(t, err)

	_, err = CreateHashtbl(nil, hashtbl_minLogSlots-1, 0)
	assert.Error(t, err)
}

func TestHashtbl_GrowthRetainsOrder(t *testing.T) {
	h0 := newTestHashtbl(t, 14)
	defer h0.Close()

	h1 := newTestHashtbl(t, 15)
	defer h1.Close()

	for i := 0; i < 1000; i++ {
		k := newKey()
		k0 := h0.slotForKey(&k)
		k1 := h1.slotForKey(&k)
		assert.True(t, k1 == 2*k0 || k1 == 2*k0+1)
	}
}

func TestHashtbl_Wraparound(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	// insert a bunch of keys that collide into the last slot.
	var keys []Key
	for i := 0; i < 10; i++ {
		k := newKeyAt(h.HashTbl, 1<<hashtbl_minLogSlots/recordsPerPage-1, recordsPerPage-1, uint8(i))
		keys = append(keys, k)
		h.AssertInsertRecord(newRecord(k))
	}

	// make sure we can read all of them.
	for _, k := range keys {
		h.AssertLookup(k)
	}
}

func TestHashtbl_ResizeDoesNotBiasEstimate(t *testing.T) {
	h0 := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h0.Close()

	for i := 0; i < 1<<hashtbl_minLogSlots/2; i++ {
		h0.AssertInsert()
	}

	h1 := newTestHashtbl(t, hashtbl_minLogSlots+1)
	defer h1.Close()

	h0.Range(func(rec Record, err error) bool {
		assert.NoError(t, err)
		ok, err := h1.Insert(rec)
		assert.That(t, ok)
		assert.NoError(t, err)
		return true
	})

	h1.AssertReopen()
	t.Logf("%v", h1.Load())
	assert.That(t, h1.Load() >= 0.1)
	assert.That(t, h1.Load() <= 0.3)
}

func TestHashtbl_RandomDistributionOfSequentialKeys(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	// load keys into the hash table that would be sequential with no hashing.
	var k Key
	for i := 0; i < 1<<hashtbl_minLogSlots/8; i++ {
		binary.BigEndian.PutUint64(k[0:8], uint64(i)<<(64-hashtbl_minLogSlots))
		h.AssertInsertRecord(newRecord(k))
	}

	// ensure no page is empty. the probability of any page being empty with a random distribution
	// is less than 2^50.
	var p page
	for offset := int64(pageSize); offset < int64(hashtblSize(hashtbl_minLogSlots)); offset += pageSize {
		_, err := h.fh.ReadAt(p[:], offset)
		assert.NoError(t, err)
		if p == (page{}) {
			t.Fatal("empty page found")
		}
	}
}

func TestHashtbl_EstimateWithNonuniformTable(t *testing.T) {
	h := newTestHashtbl(t, hashtbl_minLogSlots)
	defer h.Close()

	// completely fill the table.
	for i := 0; i < 1<<hashtbl_minLogSlots; i++ {
		h.AssertInsert()
	}

	// overwrite the first half of the table with zeros.
	_, err := h.fh.WriteAt(make([]byte, (hashtblSize(hashtbl_minLogSlots)-pageSize)/2), pageSize)
	assert.NoError(t, err)

	// the load should be around 0.5 after recomputing the estimates.
	h.AssertReopen()
	t.Logf("%v", h.Load())
	assert.That(t, h.Load() >= 0.4)
	assert.That(t, h.Load() <= 0.6)
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
			keys = append(keys, rec.Key)
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

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			func() {
				h := newTestHashtbl(b, lrec)
				defer h.Close()
				for i := 0; i < inserts; i++ {
					h.AssertInsert()
				}
			}()
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
		var recs []Record
		h.Range(func(rec Record, err error) bool {
			assert.NoError(b, err)
			recs = append(recs, rec)
			return true
		})

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			h := newTestHashtbl(b, lrec+1)
			flush, _, err := h.ExpectOrdered()
			assert.NoError(b, err)

			for _, rec := range recs {
				h.AssertInsertRecord(rec)
			}

			assert.NoError(b, flush())
			h.Close()
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
	})
}
