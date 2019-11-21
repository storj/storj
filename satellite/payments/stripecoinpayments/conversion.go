// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"math"
	"math/big"
)

// convertToCents convert amount to cents with given rate.
func convertToCents(rate, amount *big.Float) int64 {
	f, _ := new(big.Float).Mul(amount, rate).Float64()
	return int64(math.Round(f * 100))
}

// convertFromCents convert amount in cents to big.Float with given rate.
func convertFromCents(rate *big.Float, amount int64) *big.Float {
	a := new(big.Float).SetInt64(amount)
	a = a.Quo(a, new(big.Float).SetInt64(100))
	return new(big.Float).Quo(a, rate)
}
