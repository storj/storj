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
