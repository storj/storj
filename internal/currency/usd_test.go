// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package currency_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/currency"
)

func TestCentDollarString(t *testing.T) {
	type Test struct {
		Amount   currency.USD
		Expected string
	}

	tests := []Test{
		{currency.Cents(1), "0.01"},
		{currency.Cents(100), "1.00"},
		{currency.Cents(101), "1.01"},
		{currency.Cents(110), "1.10"},
		{currency.Cents(123456789), "1234567.89"},

		{currency.Cents(-1), "-0.01"},
		{currency.Cents(-100), "-1.00"},
		{currency.Cents(-101), "-1.01"},
		{currency.Cents(-110), "-1.10"},
		{currency.Cents(-123456789), "-1234567.89"},
	}

	for _, test := range tests {
		s := test.Amount.String()
		assert.Equal(t, test.Expected, s, test.Amount.Cents())
	}
}
