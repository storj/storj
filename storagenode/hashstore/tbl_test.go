// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestTable_BasicOperation(t *testing.T) {
	forAllTables(t, testTable_BasicOperation)
}
func testTable_BasicOperation(t *testing.T) {
	ctx := context.Background()
	tbl := newTestTable(t, tbl_minLogSlots)
	defer tbl.Close()

	var keys []Key
	var expLength uint64
	for i := 0; i < 1<<tbl_minLogSlots/2; i++ {
		// insert the record.
		r := tbl.AssertInsert()

		// keep track of the key that was used.
		keys = append(keys, r.Key)
		expLength += uint64(r.Length)

		// we should be able to find it.
		tbl.AssertLookup(r.Key)

		// reinsert should be fine.
		ok, err := tbl.Insert(ctx, r)
		assert.NoError(t, err)
		assert.True(t, ok)

		// we should still be able to find it.
		tbl.AssertLookup(r.Key)
	}

	assert.Equal(t, tbl.Load(), 0.5)
	stats := tbl.Stats()
	assert.Equal(t, stats.NumSet, 1<<tbl_minLogSlots/2)
	assert.Equal(t, stats.LenSet, expLength)

	// reopen the hash table and search again
	tbl.AssertReopen()
	defer tbl.Close()

	// shuffle the keys so that reads are not in the same order this time
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	for _, k := range keys {
		assert.Equal(t, tbl.AssertLookup(k), newRecord(k))
	}

	// insert, lookup, and range should fail after close.
	tbl.Close()

	_, _, err := tbl.Lookup(ctx, newKey())
	assert.Error(t, err)

	_, err = tbl.Insert(ctx, newRecord(newKey()))
	assert.Error(t, err)

	assert.Error(t, tbl.Range(ctx, func(context.Context, Record) (bool, error) {
		panic("should not be called")
	}))
}

func TestTable_OverwriteRecords(t *testing.T) {
	forAllTables(t, testTable_OverwriteMergeRecords)
}
func testTable_OverwriteMergeRecords(t *testing.T) {
	ctx := context.Background()
	tbl := newTestTable(t, tbl_minLogSlots)
	defer tbl.Close()

	// create a new record with a non-zero expiration.
	rec := newRecord(newKey())
	rec.Expires = NewExpiration(1, true)

	// insert the record.
	ok, err := tbl.Insert(ctx, rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record.
	got, ok, err := tbl.Lookup(ctx, rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got, rec)

	// set the expiration to 0 and overwrite. this should be allowed.
	rec.Expires = 0
	ok, err = tbl.Insert(ctx, rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with no expiration because that's a larger expiration.
	got, ok, err = tbl.Lookup(ctx, rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got, rec)

	// we should not be able to overwrite the record with a different log file.
	rec.Log++
	_, err = tbl.Insert(ctx, rec)
	assert.Error(t, err)

	// we should be able to try to overwrite the record with a smaller expiration
	rec.Log--
	rec.Expires = NewExpiration(2, true)
	ok, err = tbl.Insert(ctx, rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with the larger expiration.
	got2, ok, err := tbl.Lookup(ctx, rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got2, got)
}

func TestTable_RangeExitEarly(t *testing.T) {
	forAllTables(t, testTable_RangeExitEarly)
}
func testTable_RangeExitEarly(t *testing.T) {
	ctx := context.Background()
	h := newTestHashtbl(t, tbl_minLogSlots)
	defer h.Close()

	// insert some records to range over.
	for i := 0; i < 100; i++ {
		h.AssertInsert()
	}

	// only iterate through 10 records and then exit early.
	n := 0
	assert.NoError(t, h.Range(ctx, func(ctx context.Context, r Record) (bool, error) {
		n++
		return n < 10, nil
	}))
}

func TestTable_ExpectOrdered(t *testing.T) {
	forAllTables(t, testTable_ExpectOrdered)
}
func testTable_ExpectOrdered(t *testing.T) {
	ctx := context.Background()
	tbl := newTestTable(t, tbl_minLogSlots)
	defer tbl.Close()

	commit, done, err := tbl.ExpectOrdered(ctx)
	assert.NoError(t, err)
	defer done()

	// write records until an automatic flush happens
	var recs []Record
	for {
		rec := tbl.AssertInsert()
		recs = append(recs, rec)
		size, err := fileSize(tbl.Handle())
		assert.NoError(t, err)
		if size > headerSize {
			break
		}
	}

	// write some more records and ensure they aren't immediately visible
	for i := 0; i < 100; i++ {
		rec := tbl.AssertInsert()
		tbl.AssertLookupMiss(rec.Key)
		recs = append(recs, rec)
	}

	// after a commit, all of them should be visible
	assert.NoError(t, commit())
	for _, rec := range recs {
		tbl.AssertLookup(rec.Key)
	}
}

//
// benchmarks
//

func BenchmarkTable(b *testing.B) {
	forAllTables(b, benchmarkTable)
}
func benchmarkTable(b *testing.B) {
	benchmarkLRecs(b, "Lookup", func(b *testing.B, lrec uint64) {
		tbl := newTestTable(b, lrec)
		defer tbl.Close()

		var keys []Key
		for i := 0; i < 1<<lrec/2; i++ {
			rec := tbl.AssertInsert()
			keys = append(keys, rec.Key)
		}

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			tbl.AssertLookup(keys[mwc.Intn(len(keys))])
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
				tbl := newTestTable(b, lrec)
				defer tbl.Close()
				for i := 0; i < inserts; i++ {
					tbl.AssertInsert()
				}
			}()
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
	})

	benchmarkLRecs(b, "Compact", func(b *testing.B, lrec uint64) {
		inserts := 1 << lrec / 2

		ctx := context.Background()
		tbl := newTestTable(b, lrec)
		defer tbl.Close()

		for i := 0; i < inserts; i++ {
			tbl.AssertInsert()
		}
		var recs []Record
		assert.NoError(b, tbl.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
			recs = append(recs, rec)
			return true, nil
		}))

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			tbl := newTestTable(b, lrec+1)
			flush, _, err := tbl.ExpectOrdered(ctx)
			assert.NoError(b, err)

			for _, rec := range recs {
				tbl.AssertInsertRecord(rec)
			}

			assert.NoError(b, flush())
			tbl.Close()
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
	})
}
