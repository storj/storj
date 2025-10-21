// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestTable_BasicOperation(t *testing.T) {
	forAllTables(t, testTable_BasicOperation)
}
func testTable_BasicOperation(t *testing.T, cfg Config) {
	ctx := t.Context()
	tbl := newTestTbl(t, cfg, tbl_minLogSlots)
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
func testTable_OverwriteMergeRecords(t *testing.T, cfg Config) {
	ctx := t.Context()
	tbl := newTestTbl(t, cfg, tbl_minLogSlots)
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
func testTable_RangeExitEarly(t *testing.T, cfg Config) {
	ctx := t.Context()
	h := newTestHashTbl(t, DefaultMmapConfig, tbl_minLogSlots)
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

func TestTable_Full(t *testing.T) {
	forAllTables(t, testTable_Full)
}
func testTable_Full(t *testing.T, cfg Config) {
	ctx := t.Context()
	tbl := newTestTbl(t, cfg, tbl_minLogSlots)
	defer tbl.Close()

	// fill the table completely.
	for i := 0; i < 1<<tbl_minLogSlots; i++ {
		tbl.AssertInsert()
	}
	assert.Equal(t, tbl.Load(), 1.0)

	// inserting a new record should fail.
	ok, err := tbl.Insert(ctx, newRecord(newKey()))
	assert.NoError(t, err)
	assert.False(t, ok)

	// looking up a key that does not exist should fail.
	_, ok, err = tbl.Lookup(ctx, newKey())
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestTable_Load(t *testing.T) {
	forAllTables(t, testTable_Load)
}
func testTable_Load(t *testing.T, cfg Config) {
	ctx := t.Context()

	// Create a new memtbl with the specified size
	tbl := newTestTbl(t, cfg, tbl_minLogSlots)
	defer tbl.Close()

	check := func(expectedLoad float64, deep bool) {
		if deep {
			// Check that the Stats() function's Load field matches
			stats := tbl.Stats()
			assert.Equal(t, stats.Load, expectedLoad)

			// The following conditions only work for the memtbl
			if _, ok := tbl.Tbl.(*MemTbl); ok {
				// Verify load is correct after reopening
				tbl.AssertReopen()
				assert.Equal(t, tbl.Load(), expectedLoad)
			}
		}

		// Check that the Load() function returns the correct value
		assert.Equal(t, tbl.Load(), expectedLoad)
	}

	// insert records until the table is full and check every time, doing a deep check periodically.
	for i := 0; ; i++ {
		ok, err := tbl.Insert(ctx, newRecord(newKey()))
		assert.NoError(t, err)

		if !ok {
			break
		}

		check(float64(i+1)/float64(1<<tbl_minLogSlots), i%1000 == 0)
	}

	// do a final deep check when the table is full.
	check(1.0, true)
}

func TestTable_TrashStats(t *testing.T) {
	forAllTables(t, testTable_TrashStats)
}
func testTable_TrashStats(t *testing.T, cfg Config) {
	tbl := newTestTbl(t, cfg, tbl_minLogSlots)
	defer tbl.Close()

	rec := newRecord(newKey())
	rec.Expires = NewExpiration(1, true)
	tbl.AssertInsertRecord(rec)

	stats := tbl.Stats()
	assert.Equal(t, stats.NumTrash, 1)
	assert.Equal(t, stats.LenTrash, rec.Length)
	assert.Equal(t, stats.AvgTrash, float64(rec.Length))
}

func TestTable_LRecBounds(t *testing.T) {
	forAllTables(t, testTable_LRecBounds)
}
func testTable_LRecBounds(t *testing.T, cfg Config) {
	ctx := t.Context()

	_, err := CreateTable(ctx, nil, tbl_maxLogSlots+1, 0, TableKind_HashTbl, cfg)
	assert.Error(t, err)

	_, err = CreateTable(ctx, nil, tbl_minLogSlots-1, 0, TableKind_HashTbl, cfg)
	assert.Error(t, err)
}

func TestTable_ConstructorAPIAfterClose(t *testing.T) {
	forAllTables(t, testTable_ConstructorAPIAfterClose)
}
func testTable_ConstructorAPIAfterClose(t *testing.T, cfg Config) {
	ctx := t.Context()

	fh, err := os.CreateTemp(t.TempDir(), "tbl")
	assert.NoError(t, err)
	defer func() { _ = fh.Close() }()

	cons, err := CreateTable(ctx, fh, tbl_minLogSlots, 0, TableKind_HashTbl, cfg)
	assert.NoError(t, err)
	defer cons.Cancel()

	ok, err := cons.Append(ctx, newRecord(newKey()))
	assert.NoError(t, err)
	assert.True(t, ok)

	// after cancel, append and done should now error.
	cons.Cancel()

	ok, err = cons.Append(ctx, newRecord(newKey()))
	assert.Error(t, err)
	assert.False(t, ok)

	tbl, err := cons.Done(ctx)
	assert.Error(t, err)
	assert.Nil(t, tbl)
}

func TestTable_ConstructorAPIAfterDone(t *testing.T) {
	forAllTables(t, testTable_ConstructorAPIAfterDone)
}
func testTable_ConstructorAPIAfterDone(t *testing.T, cfg Config) {
	ctx := t.Context()

	fh, err := os.CreateTemp(t.TempDir(), "tbl")
	assert.NoError(t, err)
	defer func() { _ = fh.Close() }()

	cons, err := CreateTable(ctx, fh, tbl_minLogSlots, 0, TableKind_HashTbl, cfg)
	assert.NoError(t, err)
	defer cons.Cancel()

	ok, err := cons.Append(ctx, newRecord(newKey()))
	assert.NoError(t, err)
	assert.True(t, ok)

	// after done, append and done should now error.
	tbl, err := cons.Done(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, tbl)
	defer assertClose(t, tbl)

	ok, err = cons.Append(ctx, newRecord(newKey()))
	assert.Error(t, err)
	assert.False(t, ok)

	tbl, err = cons.Done(ctx)
	assert.Error(t, err)
	assert.Nil(t, tbl)
}

func TestTable_InvalidHeaders(t *testing.T) {
	fh, err := os.CreateTemp(t.TempDir(), "tbl")
	assert.NoError(t, err)
	defer func() { _ = fh.Close() }()

	hdr := TblHeader{
		Created:  1,
		HashKey:  true,
		Kind:     TableKind_HashTbl,
		LogSlots: 4,
	}

	// ensure modifying every byte in the header is an error
	for offset := int64(0); offset < tbl_headerSize; offset++ {
		assert.NoError(t, WriteTblHeader(fh, hdr))
		_, err = fh.WriteAt([]byte{0xde}, offset)
		assert.NoError(t, err)
		_, err = ReadTblHeader(fh)
		assert.Error(t, err)
	}
}

func TestTable_OpenIncorrectKind(t *testing.T) {
	ctx := t.Context()

	h := newTestHashTbl(t, DefaultMmapConfig, tbl_minLogSlots)
	defer h.Close()

	m := newTestMemTbl(t, DefaultMmapConfig, tbl_minLogSlots)
	defer m.Close()

	_, _, err := OpenMemTbl(ctx, h.fh, DefaultMmapConfig)
	assert.Error(t, err)

	_, _, err = OpenHashTbl(ctx, m.fh, DefaultMmapConfig)
	assert.Error(t, err)
}

//
// benchmarks
//

func BenchmarkTable(b *testing.B) {
	forAllTables(b, benchmarkTable)
}
func benchmarkTable(b *testing.B, cfg Config) {
	benchmarkLRecs(b, "Lookup", func(b *testing.B, lrec uint64) {
		tbl := newTestTbl(b, cfg, lrec)
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
				tbl := newTestTbl(b, cfg, lrec)
				defer tbl.Close()
				for i := 0; i < inserts; i++ {
					tbl.AssertInsert()
				}
			}()
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
		b.ReportMetric(float64(time.Since(now))/float64(b.N)/float64(inserts), "ns/key")
	})

	benchmarkLRecs(b, "Compact", func(b *testing.B, lrec uint64) {
		inserts := 1 << lrec / 2

		ctx := b.Context()
		tbl := newTestTbl(b, cfg, lrec)
		defer tbl.Close()

		for i := 0; i < inserts; i++ {
			tbl.AssertInsert()
		}

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			newTestTbl(b, cfg, lrec+1, WithConstructor(func(tc TblConstructor) {
				assert.NoError(b, tbl.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
					ok, err := tc.Append(ctx, rec)
					assert.NoError(b, err)
					assert.That(b, ok)
					return true, nil
				}))
			})).Close()
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
		b.ReportMetric(float64(time.Since(now))/float64(b.N)/float64(inserts), "ns/key")
	})
}
