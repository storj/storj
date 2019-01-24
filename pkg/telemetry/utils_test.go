// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJitter_NegativeDuration(t *testing.T) {
	duration := time.Duration(-1)
	expected := time.Duration(1)

	actual := jitter(duration)

	assert.Equal(t, expected, actual)
}
