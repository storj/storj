// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

// Currency is a type wrapper for defined currencies.
type Currency string

const (
	// CurrencyUSD defines USD.
	CurrencyUSD Currency = "USD"
	// CurrencyLTCT defines LTCT, coins used for testing purpose.
	CurrencyLTCT Currency = "LTCT"
	// CurrencySTORJ defines STORJ tokens.
	CurrencySTORJ Currency = "STORJ"
)

// String returns Currency string representation.
func (c Currency) String() string {
	return string(c)
}
