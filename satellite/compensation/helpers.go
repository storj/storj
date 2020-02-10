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

func NodeEscrowPercent(escrowPercents []int, nodeCreatedAt, endDate time.Time) (int, bool) {
	for i, escrowPercent := range escrowPercents {
		if nodeCreatedAt.AddDate(0, i+1, 0).After(endDate) {
			return escrowPercent, true
		}
	}
	return 0, false
}

// PercentOf sets v to a percentage of itself. For example if v was 200 and
// percent was 20, v would be set to 40.
func PercentOf(v, percent decimal.Decimal) decimal.Decimal {
	return v.Mul(percent).Div(oneHundred)
}
