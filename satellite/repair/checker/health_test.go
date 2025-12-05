// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
)

func TestNormalized(t *testing.T) {
	k := NormalizedHealth{}
	ctx := testcontext.New(t)
	assert.Less(t,
		k.Calculate(ctx, 11, 10, 0),
		k.Calculate(ctx, 10, 5, 0))
	assert.Less(t,
		k.Calculate(ctx, 11, 10, 0),
		k.Calculate(ctx, 10, 9, 0))
	assert.Less(t,
		k.Calculate(ctx, 10, 10, 0),
		k.Calculate(ctx, 9, 9, 0))
	assert.Greater(t,
		k.Calculate(ctx, 11, 10, 0),
		k.Calculate(ctx, 12, 11, 0))
	assert.Greater(t,
		k.Calculate(ctx, 13, 10, 0),
		k.Calculate(ctx, 12, 10, 0))

	t.Run("test high health and low failure rate", func(t *testing.T) {
		assert.Less(t,
			k.Calculate(ctx, 36, 35, 0),
			math.Inf(1))
		assert.Greater(t,
			k.Calculate(ctx, 36, 35, 0),
			k.Calculate(ctx, 35, 35, 0))
		assert.Less(t,
			k.Calculate(ctx, 60, 29, 0),
			math.Inf(1))
		assert.Greater(t,
			k.Calculate(ctx, 61, 29, 0),
			k.Calculate(ctx, 60, 29, 0))

		assert.Greater(t,
			k.Calculate(ctx, 11, 10, 0),
			k.Calculate(ctx, 39, 34, 0))
	})
	t.Run("oop priority", func(t *testing.T) {
		// POPs existence means lower health
		assert.Less(t,
			k.Calculate(ctx, 45, 29, 1),
			k.Calculate(ctx, 45, 29, 0))
		// more POPs mean lower health than fewer POPs
		assert.Less(t,
			k.Calculate(ctx, 45, 29, 2),
			k.Calculate(ctx, 45, 29, 1))
		// segments in severe danger have lower health than much more healthy segments with POPs
		assert.Less(t,
			k.Calculate(ctx, 30, 29, 0),
			k.Calculate(ctx, 50, 29, 1))
		// a segment with POPs is less healthy than a segment without, even when the segment without has
		// fewer healthy pieces, as long as the segment without is not in critical danger
		assert.Less(t,
			k.Calculate(ctx, 56, 29, 1),
			k.Calculate(ctx, 40, 29, 0))
		// health works as expected when segments have the same (nonzero) number of POPs
		assert.Less(t,
			k.Calculate(ctx, 11, 10, 1),
			k.Calculate(ctx, 10, 5, 1))
		assert.Less(t,
			k.Calculate(ctx, 11, 10, 1),
			k.Calculate(ctx, 10, 9, 1))
		assert.Less(t,
			k.Calculate(ctx, 10, 10, 1),
			k.Calculate(ctx, 9, 9, 1))
	})
}
