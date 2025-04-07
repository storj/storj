// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"testing"

	"github.com/zeebo/assert"
)

func TestMemTbl_ShortCollision(t *testing.T) {
	m := newTestMemTbl(t, tbl_minLogSlots)
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
}

func TestMemTbl_ConstructorSometimesFlushes(t *testing.T) {
	newTestMemTbl(t, tbl_minLogSlots, WithConstructor(func(tc TblConstructor) {
		// make two keys that collide on short but are not equal.
		k0, k1 := newShortCollidingKeys()

		assertAppend := func(ok bool, err error) {
			assert.NoError(t, err)
			assert.True(t, ok)
		}

		// inserting k1 will require reading the record for k0 because of the collision, and so it
		// requires reading the record, which will error if we don't flush.
		assertAppend(tc.Append(context.Background(), newRecord(k0)))
		assertAppend(tc.Append(context.Background(), newRecord(k1)))
	})).Close()
}

func TestMemtbl_LoadWithCollisions(t *testing.T) {
	// Create a new memtbl
	m := newTestMemTbl(t, tbl_minLogSlots)
	defer m.Close()

	totalSlots := uint64(1) << tbl_minLogSlots

	// Insert some normal keys
	normalCount := 1000
	for i := 0; i < normalCount; i++ {
		m.AssertInsert()
	}

	// Now insert some keys that will collide on their shortKey
	collisionCount := 50
	baseKey := newKey()
	for i := 0; i < collisionCount; i++ {
		collisionKey := baseKey
		// Modify a byte that won't affect the shortKey
		collisionKey[0] = byte(i)
		m.AssertInsert(WithKey(collisionKey))
	}

	// The total inserted record count should be normalCount + collisionCount
	totalInserted := normalCount + collisionCount

	// Calculate the expected load
	expectedLoad := float64(totalInserted) / float64(totalSlots)

	// Check that the Load() function returns the correct value
	assert.Equal(t, m.Load(), expectedLoad)
}

func TestMemtbl_UpdateCollisions(t *testing.T) {
	ctx := context.Background()
	m := newTestMemTbl(t, tbl_minLogSlots)
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
