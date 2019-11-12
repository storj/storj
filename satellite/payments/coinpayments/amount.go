// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"math/big"
)

// Precision is precision amount used to parse currency amount.
// Set enough precision to to able to handle up to 8 digits after point.
const Precision = 32

// parseAmount parses amount string into big.Float with package wide defined precision.
func parseAmount(s string) (*big.Float, error) {
	amount, _, err := big.ParseFloat(s, 10, Precision, big.ToNearestEven)
	return amount, err
}
