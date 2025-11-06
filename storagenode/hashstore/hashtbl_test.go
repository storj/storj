// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeebo/assert"
)

func TestHashTbl_TrashStats(t *testing.T) {
	h := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h.Close()

	rec := newRecord(newKey())
	rec.Expires = NewExpiration(1, true)
	h.AssertInsertRecord(rec)

	stats := h.Stats()
	assert.Equal(t, stats.NumTrash, 1)
	assert.Equal(t, stats.LenTrash, rec.Length)
	assert.Equal(t, stats.AvgTrash, float64(rec.Length))
}

func TestHashTbl_Full(t *testing.T) {
	ctx := t.Context()

	h := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h.Close()

	// fill the table completely.
	for range 1 << tbl_minLogSlots {
		h.AssertInsert()
	}
	assert.Equal(t, h.Load(), 1.0)

	// inserting a new record should fail.
	ok, err := h.Insert(ctx, newRecord(newKey()))
	assert.NoError(t, err)
	assert.False(t, ok)

	// looking up a key that does not exist should fail.
	_, ok, err = h.Lookup(ctx, newKey())
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestHashTbl_LostPage(t *testing.T) {
	ctx := t.Context()

	h := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h.Close()

	// create two keys that collide at the end of the first page.
	k0 := newKeyAt(h.HashTbl, 0, recordsPerPage-1, 0)
	k1 := newKeyAt(h.HashTbl, 0, recordsPerPage-1, 1)

	// insert the first key.
	ok, err := h.Insert(ctx, newRecord(k0))
	assert.NoError(t, err)
	assert.True(t, ok)
	h.AssertLookup(k0)

	// collide it with the second key.
	ok, err = h.Insert(ctx, newRecord(k1))
	assert.NoError(t, err)
	assert.True(t, ok)
	h.AssertLookup(k1)

	// zero out the first page manually and invalidate the page cache.
	_, err = h.fh.WriteAt(make([]byte, pageSize), tbl_headerSize) // offset=headerSize to skip the header page.
	assert.NoError(t, err)

	// we should still be able to read the second key.
	h.AssertLookup(k1)

	// the first key, though, is gone.
	_, ok, err = h.Lookup(ctx, k0)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestHashTbl_SmallFileSizes(t *testing.T) {
	ctx := t.Context()

	fh, err := os.Create(filepath.Join(t.TempDir(), "tmp"))
	assert.NoError(t, err)
	defer func() { _ = fh.Close() }()

	_, _, err = OpenHashTbl(ctx, fh, defaultMmap())
	assert.Error(t, err)

	assert.NoError(t, fh.Truncate(tbl_headerSize))
	_, _, err = OpenHashTbl(ctx, fh, defaultMmap())
	assert.Error(t, err)

	assert.NoError(t, fh.Truncate(tbl_headerSize+(pageSize-1)))
	_, _, err = OpenHashTbl(ctx, fh, defaultMmap())
	assert.Error(t, err)
}

func TestHashTbl_LRecBounds(t *testing.T) {
	ctx := t.Context()

	_, err := CreateHashTbl(ctx, nil, tbl_maxLogSlots+1, 0, defaultMmap())
	assert.Error(t, err)

	_, err = CreateHashTbl(ctx, nil, tbl_minLogSlots-1, 0, defaultMmap())
	assert.Error(t, err)
}

func TestHashTbl_GrowthRetainsOrder(t *testing.T) {
	h0 := newTestHashTbl(t, defaultMmap(), 14)
	defer h0.Close()

	h1 := newTestHashTbl(t, defaultMmap(), 15)
	defer h1.Close()

	for range 1000 {
		k := newKey()
		k0 := h0.slotForKey(&k)
		k1 := h1.slotForKey(&k)
		assert.True(t, k1 == 2*k0 || k1 == 2*k0+1)
	}
}

func TestHashTbl_Wraparound(t *testing.T) {
	h := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h.Close()

	// insert a bunch of keys that collide into the last slot.
	var keys []Key
	for i := range 10 {
		k := newKeyAt(h.HashTbl, 1<<tbl_minLogSlots/recordsPerPage-1, recordsPerPage-1, uint8(i))
		keys = append(keys, k)
		h.AssertInsertRecord(newRecord(k))
	}

	// make sure we can read all of them.
	for _, k := range keys {
		h.AssertLookup(k)
	}
}

func TestHashTbl_ResizeDoesNotBiasEstimate(t *testing.T) {
	ctx := t.Context()

	h0 := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h0.Close()

	for range 1 << tbl_minLogSlots / 2 {
		h0.AssertInsert()
	}

	h1 := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots+1)
	defer h1.Close()

	assert.NoError(t, h0.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
		ok, err := h1.Insert(ctx, rec)
		assert.That(t, ok)
		assert.NoError(t, err)
		return true, nil
	}))

	h1.AssertReopen()
	t.Logf("%v", h1.Load())
	assert.That(t, h1.Load() >= 0.1)
	assert.That(t, h1.Load() <= 0.3)
}

func TestHashTbl_RandomDistributionOfSequentialKeys(t *testing.T) {
	h := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h.Close()

	// load keys into the hash table that would be sequential with no hashing.
	var k Key
	for i := range 1 << tbl_minLogSlots / 8 {
		binary.BigEndian.PutUint64(k[0:8], uint64(i)<<(64-tbl_minLogSlots))
		h.AssertInsertRecord(newRecord(k))
	}

	// ensure no 4096 byte page is empty. the probability of any page being empty with a random distribution
	// is less than 2^50.
	var p [4096]byte
	for offset := int64(tbl_headerSize); offset < int64(hashtblSize(tbl_minLogSlots)); offset += int64(len(p)) {
		_, err := h.fh.ReadAt(p[:], offset)
		assert.NoError(t, err)
		if p == ([4096]byte{}) {
			t.Fatal("empty page found")
		}
	}
}

func TestHashTbl_EstimateWithNonuniformTable(t *testing.T) {
	h := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h.Close()

	// completely fill the table.
	for range 1 << tbl_minLogSlots {
		h.AssertInsert()
	}

	// overwrite the first half of the table with zeros.
	_, err := h.fh.WriteAt(make([]byte, (hashtblSize(tbl_minLogSlots)-tbl_headerSize)/2), tbl_headerSize)
	assert.NoError(t, err)

	// the load should be around 0.5 after recomputing the estimates. it's hard to get a good value
	// for the load that won't fail sometimes, but the probability of all of the pages sampled are
	// in the first (or second) half is ~1/2^2048 so this should never fail.
	h.AssertReopen()
	t.Logf("%v", h.Load())
	assert.That(t, h.Load() != 0)
	assert.That(t, h.Load() != 1)
}

func TestMmapCache(t *testing.T) {
	data := make([]byte, tbl_headerSize+RecordSize)
	c := newMmapCache(data)

	exp := newRecord(newKey())
	got := Record{}

	assert.NoError(t, c.WriteRecord(0, &exp))
	ok, err := c.ReadRecord(0, &got)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, exp, got)

	data[tbl_headerSize]++
	ok, err = c.ReadRecord(0, &got)
	assert.NoError(t, err)
	assert.False(t, ok)

	assert.Error(t, c.WriteRecord(1, &exp))
	ok, err = c.ReadRecord(1, &got)
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestHashTbl_IncorrectLogSlots(t *testing.T) {
	ctx := t.Context()

	h := newTestHashTbl(t, defaultMmap(), tbl_minLogSlots)
	defer h.Close()

	assert.NoError(t, h.fh.Truncate(int64(hashtblSize(tbl_minLogSlots+1))))

	_, _, err := OpenHashTbl(ctx, h.fh, defaultMmap())
	assert.Error(t, err)
}
