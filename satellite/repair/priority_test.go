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
		repair.SegmentHealth(11, 10, 10000, failureRate, 0),
		repair.SegmentHealth(10, 5, 10000, failureRate, 0))
	assert.Less(t,
		repair.SegmentHealth(11, 10, 10000, failureRate, 0),
		repair.SegmentHealth(10, 9, 10000, failureRate, 0))
	assert.Less(t,
		repair.SegmentHealth(10, 10, 10000, failureRate, 0),
		repair.SegmentHealth(9, 9, 10000, failureRate, 0))
	assert.Greater(t,
		repair.SegmentHealth(11, 10, 10000, failureRate, 0),
		repair.SegmentHealth(12, 11, 10000, failureRate, 0))
	assert.Greater(t,
		repair.SegmentHealth(13, 10, 10000, failureRate, 0),
		repair.SegmentHealth(12, 10, 10000, failureRate, 0))
}

func TestSegmentHealthForDecayedSegment(t *testing.T) {
	const failureRate = 0.01
	got := repair.SegmentHealth(9, 10, 10000, failureRate, 0)
	assert.Equal(t, float64(0), got)
}

func TestHighHealthAndLowFailureRate(t *testing.T) {
	const failureRate = 0.00005435
	assert.Less(t,
		repair.SegmentHealth(36, 35, 10000, failureRate, 0),
		math.Inf(1))
	assert.Greater(t,
		repair.SegmentHealth(36, 35, 10000, failureRate, 0),
		repair.SegmentHealth(35, 35, 10000, failureRate, 0))
	assert.Less(t,
		repair.SegmentHealth(60, 29, 10000, failureRate, 0),
		math.Inf(1))
	assert.Greater(t,
		repair.SegmentHealth(61, 29, 10000, failureRate, 0),
		repair.SegmentHealth(60, 29, 10000, failureRate, 0))

	assert.Greater(t,
		repair.SegmentHealth(11, 10, 10000, failureRate, 0),
		repair.SegmentHealth(39, 34, 10000, failureRate, 0))
}

func TestPiecesOutOfPlacementCauseHighPriority(t *testing.T) {
	const failureRate = 0.00005435
	// POPs existence means lower health
	assert.Less(t,
		repair.SegmentHealth(45, 29, 100000, failureRate, 1),
		repair.SegmentHealth(45, 29, 100000, failureRate, 0))
	// more POPs mean lower health than fewer POPs
	assert.Less(t,
		repair.SegmentHealth(45, 29, 100000, failureRate, 2),
		repair.SegmentHealth(45, 29, 100000, failureRate, 1))
	// segments in severe danger have lower health than much more healthy segments with POPs
	assert.Less(t,
		repair.SegmentHealth(30, 29, 100000, failureRate, 0),
		repair.SegmentHealth(50, 29, 100000, failureRate, 1))
	// a segment with POPs is less healthy than a segment without, even when the segment without has
	// fewer healthy pieces, as long as the segment without is not in critical danger
	assert.Less(t,
		repair.SegmentHealth(56, 29, 100000, failureRate, 1),
		repair.SegmentHealth(40, 29, 100000, failureRate, 0))
	// health works as expected when segments have the same (nonzero) number of POPs
	assert.Less(t,
		repair.SegmentHealth(11, 10, 100000, failureRate, 1),
		repair.SegmentHealth(10, 5, 100000, failureRate, 1))
	assert.Less(t,
		repair.SegmentHealth(11, 10, 10000, failureRate, 1),
		repair.SegmentHealth(10, 9, 10000, failureRate, 1))
	assert.Less(t,
		repair.SegmentHealth(10, 10, 10000, failureRate, 1),
		repair.SegmentHealth(9, 9, 10000, failureRate, 1))
}
