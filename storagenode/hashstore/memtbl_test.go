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

func TestMemtbl_BasicOperation(t *testing.T) {
	ctx := context.Background()
	m := newTestMemtbl(t, tbl_minLogSlots)
	defer m.Close()

	var keys []Key
	var expLength uint64
	for i := 0; i < 1<<tbl_minLogSlots/2; i++ {
		// insert the record.
		r := m.AssertInsert()

		// keep track of the key that was used.
		keys = append(keys, r.Key)
		expLength += uint64(r.Length)

		// we should be able to find it.
		m.AssertLookup(r.Key)

		// reinsert should be fine.
		ok, err := m.Insert(ctx, r)
		assert.NoError(t, err)
		assert.True(t, ok)

		// we should still be able to find it.
		m.AssertLookup(r.Key)
	}

	assert.Equal(t, m.Load(), 0.5)
	stats := m.Stats()
	assert.Equal(t, stats.NumSet, 1<<tbl_minLogSlots/2)
	assert.Equal(t, stats.LenSet, expLength)

	// reopen the hash table and search again
	m.AssertReopen()
	defer m.Close()

	// shuffle the keys so that reads are not in the same order this time
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	for _, k := range keys {
		assert.Equal(t, m.AssertLookup(k), newRecord(k))
	}

	// insert, lookup, and range should fail after close.
	m.Close()

	_, _, err := m.Lookup(ctx, newKey())
	assert.Error(t, err)

	_, err = m.Insert(ctx, newRecord(newKey()))
	assert.Error(t, err)

	assert.Error(t, m.Range(ctx, func(context.Context, Record) (bool, error) {
		panic("should not be called")
	}))
}

func TestMemtbl_OverwriteMergeRecords(t *testing.T) {
	ctx := context.Background()
	m := newTestMemtbl(t, tbl_minLogSlots)
	defer m.Close()

	// create a new record with a non-zero expiration.
	rec := newRecord(newKey())
	rec.Expires = NewExpiration(1, true)

	// insert the record.
	ok, err := m.Insert(ctx, rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record.
	got, ok, err := m.Lookup(ctx, rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got, rec)

	// set the expiration to 0 and overwrite. this should be allowed.
	rec.Expires = 0
	ok, err = m.Insert(ctx, rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with no expiration because that's a larger expiration.
	got, ok, err = m.Lookup(ctx, rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got, rec)

	// we should not be able to overwrite the record with a different log file.
	rec.Log++
	_, err = m.Insert(ctx, rec)
	assert.Error(t, err)

	// we should be able to try to overwrite the record with a smaller expiration
	rec.Log--
	rec.Expires = NewExpiration(2, true)
	ok, err = m.Insert(ctx, rec)
	assert.NoError(t, err)
	assert.True(t, ok)

	// we should get back the record with the larger expiration.
	got2, ok, err := m.Lookup(ctx, rec.Key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, got2, got)
}

func TestMemtbl_ShortCollision(t *testing.T) {
	m := newTestMemtbl(t, tbl_minLogSlots)
	defer m.Close()

	// make two keys that collide on short but are not equal.
	k0 := newKey()
	k1 := k0
	k1[len(k1)-1]++
	assert.Equal(t, *(*shortKey)(k0[:]), *(*shortKey)(k1[:]))
	assert.NotEqual(t, k0, k1)

	// inserting with k0 doesn't return records for k1.
	m.AssertInsert(WithKey(k0))
	m.AssertLookup(k0)
	m.AssertLookupMiss(k1)

	// same state after reopen.
	m.AssertReopen()
	m.AssertLookup(k0)
	m.AssertLookupMiss(k1)

	// insert with k1 and we should be able to find both still.
	m.AssertInsert(WithKey(k1))
	m.AssertLookup(k0)
	m.AssertLookup(k1)

	// same state after reopen.
	m.AssertReopen()
	m.AssertLookup(k0)
	m.AssertLookup(k1)
}

//
// benchmarks
//

func BenchmarkMemtbl(b *testing.B) {
	benchmarkLRecs(b, "Lookup", func(b *testing.B, lrec uint64) {
		m := newTestMemtbl(b, lrec)
		defer m.Close()

		var keys []Key
		for i := 0; i < 1<<lrec/2; i++ {
			rec := m.AssertInsert()
			keys = append(keys, rec.Key)
		}

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			m.AssertLookup(keys[mwc.Intn(len(keys))])
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
				m := newTestMemtbl(b, lrec)
				defer m.Close()
				for i := 0; i < inserts; i++ {
					m.AssertInsert()
				}
			}()
		}

		b.ReportMetric(float64(b.N)*float64(inserts)/time.Since(now).Seconds(), "keys/sec")
	})

	benchmarkLRecs(b, "Compact", func(b *testing.B, lrec uint64) {
		inserts := 1 << lrec / 2

		ctx := context.Background()
		m := newTestMemtbl(b, lrec)
		defer m.Close()

		for i := 0; i < inserts; i++ {
			m.AssertInsert()
		}
		var recs []Record
		assert.NoError(b, m.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
			recs = append(recs, rec)
			return true, nil
		}))

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			h := newTestMemtbl(b, lrec+1)
			flush, _, err := h.ExpectOrdered(ctx)
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
