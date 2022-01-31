// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSegmentHealth(t *testing.T) {
	const failureRate = 0.01
	assert.Less(t,
		SegmentHealth(11, 10, 10000, failureRate),
		SegmentHealth(10, 5, 10000, failureRate))
	assert.Less(t,
		SegmentHealth(11, 10, 10000, failureRate),
		SegmentHealth(10, 9, 10000, failureRate))
	assert.Less(t,
		SegmentHealth(10, 10, 10000, failureRate),
		SegmentHealth(9, 9, 10000, failureRate))
	assert.Greater(t,
		SegmentHealth(11, 10, 10000, failureRate),
		SegmentHealth(12, 11, 10000, failureRate))
	assert.Greater(t,
		SegmentHealth(13, 10, 10000, failureRate),
		SegmentHealth(12, 10, 10000, failureRate))
}

func TestSegmentHealthForDecayedSegment(t *testing.T) {
	const failureRate = 0.01
	got := SegmentHealth(9, 10, 10000, failureRate)
	assert.Equal(t, float64(0), got)
}

func TestHighHealthAndLowFailureRate(t *testing.T) {
	const failureRate = 0.00005435
	assert.Less(t,
		SegmentHealth(36, 35, 10000, failureRate), math.Inf(1))
	assert.Greater(t,
		SegmentHealth(36, 35, 10000, failureRate),
		SegmentHealth(35, 35, 10000, failureRate))
	assert.Less(t,
		SegmentHealth(60, 29, 10000, failureRate), math.Inf(1))
	assert.Greater(t,
		SegmentHealth(61, 29, 10000, failureRate),
		SegmentHealth(60, 29, 10000, failureRate))

	assert.Greater(t,
		SegmentHealth(11, 10, 10000, failureRate),
		SegmentHealth(39, 34, 10000, failureRate))
}
