// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/rewards"
)

func TestCentDollarString(t *testing.T) {
	type Test struct {
		Cent     rewards.CentX
		Expected string
	}

	tests := []Test{
		{1, "0.01"},
		{100, "1.00"},
		{101, "1.01"},
		{110, "1.10"},
		{123456789, "1234567.89"},

		{-1, "-0.01"},
		{-100, "-1.00"},
		{-101, "-1.01"},
		{-110, "-1.10"},
		{-123456789, "-1234567.89"},
	}

	for _, test := range tests {
		s := test.Cent.DollarsString()
		assert.Equal(t, test.Expected, s, int(test.Cent))
	}
}
