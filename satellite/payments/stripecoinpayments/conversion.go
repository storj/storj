// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"math"
	"math/big"
)

// convertToCents convert amount to cents with with given rate.
func convertToCents(rate, amount *big.Float) int64 {
	f, _ := new(big.Float).Mul(amount, rate).Float64()
	return int64(math.Floor(f * 100))
}
