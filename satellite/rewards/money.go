// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards

import (
	"fmt"
)

// USD describes USD currency in cents.
type USD int

// USDFromDollars converts USD credit amounts to cents.
func USDFromDollars(dollars int) USD {
	return USD(dollars * 100)
}

// String returns the value in dollars.
func (cents USD) String() string {
	if cents < 0 {
		return fmt.Sprintf("-%d.%02d", -cents/100, -cents%100)
	}
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}
