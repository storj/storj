// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards

import (
	"fmt"
)

type Cent = int

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
