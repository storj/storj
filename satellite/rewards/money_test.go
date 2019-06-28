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
		Amount   rewards.USD
		Expected string
	}

	tests := []Test{
		{rewards.Cents(1), "0.01"},
		{rewards.Cents(100), "1.00"},
		{rewards.Cents(101), "1.01"},
		{rewards.Cents(110), "1.10"},
		{rewards.Cents(123456789), "1234567.89"},

		{rewards.Cents(-1), "-0.01"},
		{rewards.Cents(-100), "-1.00"},
		{rewards.Cents(-101), "-1.01"},
		{rewards.Cents(-110), "-1.10"},
		{rewards.Cents(-123456789), "-1234567.89"},
	}

	for _, test := range tests {
		s := test.Amount.String()
		assert.Equal(t, test.Expected, s, test.Amount.Cents())
	}
}
