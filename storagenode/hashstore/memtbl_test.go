// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"testing"

	"github.com/zeebo/assert"

	"storj.io/storj/storagenode/hashstore/platform"
)

func TestMemTbl_ShortCollision(t *testing.T) {
	mmapMatrixTest(t, testMemTbl_ShortCollision)
}
func testMemTbl_ShortCollision(t *testing.T, mmap bool) {
	m := newTestMemTbl(t, MmapCfg{Mmap: mmap, Mlock: true}, tbl_minLogSlots)
	defer m.Close()

	// make two keys that collide on short but are not equal.
	k0, k1 := newShortCollidingKeys()

	// inserting with k0 doesn't return records for k1.
	m.AssertInsert(WithKey(k0))
	m.AssertLookup(k0)
	m.AssertLookupMiss(k1)

	// ensure that the collisions map is empty.
	assert.That(t, len(m.collisions) == 0)

	// same state after reopen.
	m.AssertReopen()
	m.AssertLookup(k0)
	m.AssertLookupMiss(k1)

	// insert with k1 and we should be able to find both still.
	m.AssertInsert(WithKey(k1))
	m.AssertLookup(k0)
	m.AssertLookup(k1)

	// ensure that the collisions map is non-empty.
	assert.That(t, len(m.collisions) > 0)

	// same state after reopen.
	m.AssertReopen()
	m.AssertLookup(k0)
	m.AssertLookup(k1)

	// ensure that looking up a third key that collides is still a miss
	k2 := k1
	k2[0]++
	assert.Equal(t, shortKeyFrom(k2), shortKeyFrom(k1))
	m.AssertLookupMiss(k2)
}

func mmapMatrixTest(t *testing.T, fn func(t *testing.T, mmap bool)) {
	t.Run("mmap=false", func(t *testing.T) {
		fn(t, false)
	})
	if platform.MmapSupported {
		t.Run("mmap=true", func(t *testing.T) {
			fn(t, true)
		})
	}
}

func TestMemTbl_ConstructorSometimesFlushes(t *testing.T) {
	mmapMatrixTest(t, testMemTbl_ConstructorSometimesFlushes)
}

func testMemTbl_ConstructorSometimesFlushes(t *testing.T, mmap bool) {
	ctx := t.Context()
	newTestMemTbl(t, MmapCfg{Mmap: mmap, Mlock: true}, tbl_minLogSlots, WithConstructor(func(tc TblConstructor) {
		// make two keys that collide on short but are not equal.
		k0, k1 := newShortCollidingKeys()

		assertAppend := func(ok bool, err error) {
			assert.NoError(t, err)
			assert.True(t, ok)
		}

		// inserting k1 will require reading the record for k0 because of the collision, and so it
		// requires reading the record, which will error if we don't flush.
		assertAppend(tc.Append(ctx, newRecord(k0)))
		assertAppend(tc.Append(ctx, newRecord(k1)))
	})).Close()
}

func TestMemTbl_LoadWithCollisions(t *testing.T) {
	mmapMatrixTest(t, testMemTbl_LoadWithCollisions)
}

func testMemTbl_LoadWithCollisions(t *testing.T, mmap bool) {
	// Create a new memtbl
	m := newTestMemTbl(t, MmapCfg{Mmap: mmap, Mlock: true}, tbl_minLogSlots)
	defer m.Close()

	// Insert some normal keys
	for i := 0; i < 100; i++ {
		m.AssertInsert()
	}

	// Now insert some keys that will collide on their shortKey
	baseKey := newKey()
	for i := 0; i < 50; i++ {
		collisionKey := baseKey
		collisionKey[0] = byte(i) // Modify a byte that won't affect the shortKey
		assert.Equal(t, shortKeyFrom(collisionKey), shortKeyFrom(baseKey))

		m.AssertInsert(WithKey(collisionKey))
	}

	// Check that the Load() function returns the correct value
	totalSlots := float64(uint64(1) << tbl_minLogSlots)
	assert.Equal(t, m.Load(), 150/totalSlots)

	// Check that after reopen it still returns the correct value
	m.AssertReopen()
	assert.Equal(t, m.Load(), 150/totalSlots)
}

func TestMemTbl_UpdateCollisions(t *testing.T) {
	mmapMatrixTest(t, testMemTbl_UpdateCollisions)
}
func testMemTbl_UpdateCollisions(t *testing.T, mmap bool) {
	ctx := t.Context()
	m := newTestMemTbl(t, MmapCfg{Mmap: mmap, Mlock: true}, tbl_minLogSlots)
	defer m.Close()

	k0, k1 := newShortCollidingKeys()

	r := func(k Key, ts uint32) Record {
		r := newRecord(k)
		r.Expires = NewExpiration(ts, false)
		return r
	}

	badRecord0 := r(k0, 0)
	badRecord0.Created++
	badRecord1 := r(k1, 0)
	badRecord1.Created++

	// insert k0 with ts 1
	m.AssertInsert(WithRecord(r(k0, 1)))
	assert.Equal(t, m.AssertLookup(k0), r(k0, 1))

	// update k0 with ts 2
	m.AssertInsert(WithRecord(r(k0, 2)))
	assert.Equal(t, m.AssertLookup(k0), r(k0, 2))

	// fail to update k0 with non-equal record
	ok, err := m.Insert(ctx, badRecord0)
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Equal(t, m.AssertLookup(k0), r(k0, 2))

	// update k0 with ts 1: should keep ts 2.
	m.AssertInsert(WithRecord(r(k0, 1)))
	assert.Equal(t, m.AssertLookup(k0), r(k0, 2))

	// insert k1 with ts 3
	m.AssertInsert(WithRecord(r(k1, 3)))
	assert.Equal(t, m.AssertLookup(k1), r(k1, 3))

	// update k1 with ts 4
	m.AssertInsert(WithRecord(r(k1, 4)))
	assert.Equal(t, m.AssertLookup(k1), r(k1, 4))

	// fail to update k1 with non-equal record
	ok, err = m.Insert(ctx, badRecord1)
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Equal(t, m.AssertLookup(k1), r(k1, 4))

	// update k1 with ts 3: should keep ts 4.
	m.AssertInsert(WithRecord(r(k1, 3)))
	assert.Equal(t, m.AssertLookup(k1), r(k1, 4))

	// update k0 with ts 5
	m.AssertInsert(WithRecord(r(k0, 5)))
	assert.Equal(t, m.AssertLookup(k0), r(k0, 5))

	// update k1 with ts 6
	m.AssertInsert(WithRecord(r(k1, 6)))
	assert.Equal(t, m.AssertLookup(k1), r(k1, 6))
}

func TestMemTbl_MMAPWithUnalignedEntries(t *testing.T) {
	ctx := t.Context()

	var keys []Key
	m := newTestMemTbl(t, MmapCfg{Mmap: true, Mlock: true}, tbl_minLogSlots, withEntries(t, 1, &keys))
	defer m.Close()

	for i := 0; i < 128; i++ {
		keys = append(keys, m.AssertInsert().Key)

		for _, key := range keys {
			m.AssertLookup(key)
		}

		expect := keys
		assert.NoError(t, m.Range(ctx, func(ctx context.Context, r Record) (bool, error) {
			assert.Equal(t, r.Key, expect[0])
			expect = expect[1:]
			return true, nil
		}))
		assert.Equal(t, len(expect), 0)
	}
}

func TestMemTbl_OpenUnaligned(t *testing.T) {
	mmapMatrixTest(t, testMemTbl_OpenUnaligned)
}
func testMemTbl_OpenUnaligned(t *testing.T, mmap bool) {
	// create a table with 128 records (8192 bytes of data) so that if we're in mmap mode it has
	// some pages to read.
	m := newTestMemTbl(t, MmapCfg{Mmap: mmap, Mlock: true}, tbl_minLogSlots, withEntries(t, 128, nil))
	defer m.Close()

	// insert a new record.
	var keys []Key
	keys = append(keys, m.AssertInsert().Key)

	// unalign the table by every possible amount, reopen, and ensure we can still read all the keys
	// including new ones we add after it realigns.
	for i := 0; i < RecordSize; i++ {
		_, err := m.fh.Write(make([]byte, i+1))
		assert.NoError(t, err)

		m.AssertReopen()

		for _, key := range keys {
			m.AssertLookup(key)
		}
		keys = append(keys, m.AssertInsert().Key)
	}
}

func TestMemTbl_ConstructorFull(t *testing.T) {
	mmapMatrixTest(t, testMemTbl_ConstructorFull)
}
func testMemTbl_ConstructorFull(t *testing.T, mmap bool) {
	ctx := t.Context()

	var keys []Key
	m := newTestMemTbl(t, MmapCfg{Mmap: mmap, Mlock: true}, tbl_minLogSlots, withFilledTable(t, &keys))
	defer m.Close()

	// ensure we can read all of the keys we inserted.
	for _, key := range keys {
		m.AssertLookup(key)
	}

	// ensure we can't insert any more keys.
	ok, err := m.Insert(ctx, newRecord(newKey()))
	assert.NoError(t, err)
	assert.False(t, ok)

	// reopen the table.
	m.AssertReopen()

	// ensure we can still read all of the keys we inserted.
	for _, key := range keys {
		m.AssertLookup(key)
	}

	// ensure we can't insert any more keys.
	ok, err = m.Insert(ctx, newRecord(newKey()))
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestMemTbl_ReopenWithCorruptRecord(t *testing.T) {
	mmapMatrixTest(t, testMemTbl_ReopenWithCorruptRecord)
}
func testMemTbl_ReopenWithCorruptRecord(t *testing.T, mmap bool) {
	m := newTestMemTbl(t, MmapCfg{Mmap: mmap, Mlock: true}, tbl_minLogSlots)
	defer m.Close()

	k0 := m.AssertInsert().Key
	k1 := m.AssertInsert().Key

	// ensure we can read both keys
	m.AssertLookup(k0)
	m.AssertLookup(k1)

	// corrupt the record.
	_, err := m.fh.WriteAt(make([]byte, RecordSize), tbl_headerSize)
	assert.NoError(t, err)

	// reopen the table.
	m.AssertReopen()

	// ensure we can't read the corrupted record, but can read the other one.
	m.AssertLookupMiss(k0)
	m.AssertLookup(k1)
}

func TestMemTbl_ReopenWithTooManyEntries(t *testing.T) {
	m := newTestMemTbl(t, MmapCfg{}, tbl_minLogSlots, withFilledTable(t, nil))
	defer m.Close()

	// add a record to the end of the log file directly.
	var buf [RecordSize]byte
	rec := newRecord(newKey())
	rec.WriteTo(&buf)

	_, err := m.fh.Write(buf[:])
	assert.NoError(t, err)

	_, _, err = OpenMemTbl(t.Context(), m.fh, MmapCfg{})
	assert.Error(t, err)
}
