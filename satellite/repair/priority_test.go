// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBetaI(t *testing.T) {
	// check a few places where betaI has some easily representable values
	assert.Equal(t, 0.0, betaI(0.5, 5, 0))
	assert.Equal(t, 0.0, betaI(1, 3, 0))
	assert.Equal(t, 0.0, betaI(8, 10, 0))
	assert.Equal(t, 0.0, betaI(8, 10, 0))
	assert.InDelta(t, 0.5, betaI(0.5, 0.5, 0.5), epsilon)
	assert.InDelta(t, 1.0/3.0, betaI(0.5, 0.5, 0.25), epsilon)
	assert.InDelta(t, 0.488, betaI(1, 3, 0.2), epsilon)
}

func BenchmarkBetaI(b *testing.B) {
	for i := 0; i < b.N; i++ {
		assert.InDelta(b, 1.0/3.0, betaI(0.5, 0.5, 0.25), epsilon)
	}
}

func TestSegmentDanger(t *testing.T) {
	const failureRate = 0.01
	assert.Greater(t,
		SegmentDanger(11, 10, failureRate),
		SegmentDanger(10, 5, failureRate))
	assert.Greater(t,
		SegmentDanger(11, 10, failureRate),
		SegmentDanger(10, 9, failureRate))
	assert.Greater(t,
		SegmentDanger(10, 10, failureRate),
		SegmentDanger(9, 9, failureRate))
	assert.Less(t,
		SegmentDanger(11, 10, failureRate),
		SegmentDanger(12, 11, failureRate))
}

func TestSegmentHealth(t *testing.T) {
	const failureRate = 0.01
	assert.Less(t,
		SegmentHealth(11, 10, failureRate),
		SegmentHealth(10, 5, failureRate))
	assert.Less(t,
		SegmentHealth(11, 10, failureRate),
		SegmentHealth(10, 9, failureRate))
	assert.Less(t,
		SegmentHealth(10, 10, failureRate),
		SegmentHealth(9, 9, failureRate))
	assert.Greater(t,
		SegmentHealth(11, 10, failureRate),
		SegmentHealth(12, 11, failureRate))
}

func TestSegmentHealthForDecayedSegment(t *testing.T) {
	const failureRate = 0.01
	assert.True(t, math.IsNaN(SegmentHealth(9, 10, failureRate)))
}
