// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
)

func randomNodeAlias() metabase.NodeAlias {
	return metabase.NodeAlias(testrand.Intn(65536))
}

func TestNodeAliasExpiringSetAddAndRemove(t *testing.T) {
	// choose 3 random and unique aliases
	alias1 := randomNodeAlias()
	alias2 := alias1
	for alias2 == alias1 {
		alias2 = randomNodeAlias()
	}
	alias3 := alias1
	for alias3 == alias1 || alias3 == alias2 {
		alias3 = randomNodeAlias()
	}

	// add them to a nodeAliasExpiringSet one at a time

	set := newNodeAliasExpiringSet(24 * time.Hour)
	assert.False(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))
	assert.False(t, set.Contains(alias3))

	set.Add(alias1)
	assert.True(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))
	assert.False(t, set.Contains(alias3))

	set.Add(alias2)
	assert.True(t, set.Contains(alias1))
	assert.True(t, set.Contains(alias2))
	assert.False(t, set.Contains(alias3))

	set.Add(alias3)
	assert.True(t, set.Contains(alias1))
	assert.True(t, set.Contains(alias2))
	assert.True(t, set.Contains(alias3))

	// then remove one at a time

	set.Remove(alias2)
	assert.True(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))
	assert.True(t, set.Contains(alias3))

	set.Remove(alias2) // again; should have no effect this time
	assert.True(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))
	assert.True(t, set.Contains(alias3))

	set.Remove(alias1)
	assert.False(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))
	assert.True(t, set.Contains(alias3))

	set.Remove(alias3)
	assert.False(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))
	assert.False(t, set.Contains(alias3))
}

type dummyTime struct {
	time.Time
}

func (dt *dummyTime) Elapse(d time.Duration) {
	dt.Time = dt.Time.Add(d)
}

func (dt *dummyTime) Get() time.Time {
	return dt.Time
}

func TestNodeAliasExpiringSetExpiration(t *testing.T) {
	mockTime := dummyTime{time.Now()}
	set := newNodeAliasExpiringSet(time.Minute)
	set.nowFunc = mockTime.Get

	alias1 := randomNodeAlias()
	alias2 := alias1
	for alias2 == alias1 {
		alias2 = randomNodeAlias()
	}

	set.Add(alias1)
	assert.True(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))

	mockTime.Elapse(30 * time.Second)
	assert.True(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))

	set.Add(alias2)
	assert.True(t, set.Contains(alias1))
	assert.True(t, set.Contains(alias2))

	mockTime.Elapse(31 * time.Second)
	assert.False(t, set.Contains(alias1))
	assert.True(t, set.Contains(alias2))

	mockTime.Elapse(30 * time.Second)
	assert.False(t, set.Contains(alias1))
	assert.False(t, set.Contains(alias2))
}
