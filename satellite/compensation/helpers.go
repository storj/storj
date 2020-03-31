// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"time"

	"github.com/shopspring/decimal"
)

var (
	oneHundred = decimal.NewFromInt(100)
)

// NodeWithheldPercent returns the percentage that should be withheld and if the node is still
// in the withholding period based on its creation date.
func NodeWithheldPercent(withheldPercents []int, nodeCreatedAt, endDate time.Time) (int, bool) {
	for i, withheldPercent := range withheldPercents {
		if nodeCreatedAt.AddDate(0, i+1, 0).After(endDate) {
			return withheldPercent, true
		}
	}
	return 0, false
}

// PercentOf sets v to a percentage of itself. For example if v was 200 and
// percent was 20, v would be set to 40.
func PercentOf(v, percent decimal.Decimal) decimal.Decimal {
	return v.Mul(percent).Div(oneHundred)
}
