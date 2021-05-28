// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestBytesAreWithinProportion(t *testing.T) {
	f := stripecoinpayments.BytesAreWithinProportion
	assert.False(t, f(uuid.UUID{0}, 0.0))

	assert.False(t, f(uuid.UUID{255}, 0.25))
	assert.False(t, f(uuid.UUID{192}, 0.25))
	assert.False(t, f(uuid.UUID{128}, 0.25))
	assert.False(t, f(uuid.UUID{64}, 0.25))
	assert.True(t, f(uuid.UUID{63, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, 0.25))
	assert.True(t, f(uuid.UUID{32}, 0.25))

	assert.False(t, f(uuid.UUID{129}, 0.5))
	assert.False(t, f(uuid.UUID{128}, 0.5))
	assert.True(t, f(uuid.UUID{127, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, 0.5))
	assert.True(t, f(uuid.UUID{127}, 0.5))

	assert.False(t, f(uuid.UUID{255}, 0.75))
	assert.False(t, f(uuid.UUID{192}, 0.75))
	assert.True(t, f(uuid.UUID{191, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, 0.75))
	assert.True(t, f(uuid.UUID{128}, 0.75))
	assert.True(t, f(uuid.UUID{64}, 0.75))
	assert.True(t, f(uuid.UUID{32}, 0.75))

	assert.True(t, f(uuid.UUID{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, 1.0))
}
