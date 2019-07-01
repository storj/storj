// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package currency

import (
	"fmt"
)

// USD describes USD currency.
type USD struct {
	cents int64
}

// Dollars converts dollars to USD amount.
func Dollars(dollars int64) USD {
	return USD{dollars * 100}
}

// Cents converts cents to USD amount.
func Cents(cents int64) USD {
	return USD{cents}
}

// Cents returns amount in cents.
func (usd USD) Cents() int64 { return usd.cents }

// String returns the value in dollars.
func (usd USD) String() string {
	if usd.cents < 0 {
		return fmt.Sprintf("-%d.%02d", -usd.cents/100, -usd.cents%100)
	}
	return fmt.Sprintf("%d.%02d", usd.cents/100, usd.cents%100)
}
