// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package repair_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/repair"
)

func TestSegmentHealth(t *testing.T) {
	const failureRate = 0.01
	assert.Less(t,
		repair.SegmentHealth(11, 10, 10000, failureRate),
		repair.SegmentHealth(10, 5, 10000, failureRate))
	assert.Less(t,
		repair.SegmentHealth(11, 10, 10000, failureRate),
		repair.SegmentHealth(10, 9, 10000, failureRate))
	assert.Less(t,
		repair.SegmentHealth(10, 10, 10000, failureRate),
		repair.SegmentHealth(9, 9, 10000, failureRate))
	assert.Greater(t,
		repair.SegmentHealth(11, 10, 10000, failureRate),
		repair.SegmentHealth(12, 11, 10000, failureRate))
	assert.Greater(t,
		repair.SegmentHealth(13, 10, 10000, failureRate),
		repair.SegmentHealth(12, 10, 10000, failureRate))
}

func TestSegmentHealthForDecayedSegment(t *testing.T) {
	const failureRate = 0.01
	got := repair.SegmentHealth(9, 10, 10000, failureRate)
	assert.Equal(t, float64(0), got)
}

func TestHighHealthAndLowFailureRate(t *testing.T) {
	const failureRate = 0.00005435
	assert.Less(t,
		repair.SegmentHealth(36, 35, 10000, failureRate), math.Inf(1))
	assert.Greater(t,
		repair.SegmentHealth(36, 35, 10000, failureRate),
		repair.SegmentHealth(35, 35, 10000, failureRate))
	assert.Less(t,
		repair.SegmentHealth(60, 29, 10000, failureRate), math.Inf(1))
	assert.Greater(t,
		repair.SegmentHealth(61, 29, 10000, failureRate),
		repair.SegmentHealth(60, 29, 10000, failureRate))

	assert.Greater(t,
		repair.SegmentHealth(11, 10, 10000, failureRate),
		repair.SegmentHealth(39, 34, 10000, failureRate))
}
