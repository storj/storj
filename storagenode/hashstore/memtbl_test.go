// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"testing"

	"github.com/zeebo/assert"
)

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
