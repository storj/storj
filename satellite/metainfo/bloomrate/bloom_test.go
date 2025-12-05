// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomrate

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestBloomRate(t *testing.T) {
	br := NewBloomRate(10, 3, rate.Every(time.Second), 3)

	now := time.Now()
	key1 := []byte("key1")
	key2 := []byte("key2")

	assert(t, br.Allow(now, key1))
	assert(t, br.Allow(now, key1))
	assert(t, br.Allow(now, key1))
	assert(t, !br.Allow(now, key1))

	assert(t, br.Allow(now, key2))
	assert(t, br.Allow(now, key2))
	assert(t, br.Allow(now, key2))
	assert(t, !br.Allow(now, key2))
}
