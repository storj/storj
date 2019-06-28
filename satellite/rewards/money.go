// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards

import (
	"fmt"
)

type Cent = int
type CentX int

// ToCents converts USD credit amounts to cents.
func ToCents(dollars int) Cent {
	return Cent(dollars * 100)
}

// ToDollars converts credit amounts in cents to USD.
func ToDollars(cents Cent) string {
	if cents < 0 {
		return fmt.Sprintf("-%d.%02d", -cents/100, -cents%100)
	}
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// CentFromDollar converts USD credit amounts to cents.
func CentFromDollar(dollars int) CentX {
	return CentX(dollars * 100)
}

// DollarsString returns the value in dollars.
func (cent CentX) DollarsString() string {
	if cent < 0 {
		return fmt.Sprintf("-%d.%02d", -cent/100, -cent%100)
	}
	return fmt.Sprintf("%d.%02d", cent/100, cent%100)
}
