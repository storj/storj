// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"storj.io/storj/satellite/payments/monetary"
)

// CurrencySymbol is a symbol for a currency as recognized by coinpayments.net.
type CurrencySymbol string

var (
	// CurrencyLTCT defines LTCT, coins used for testing purpose.
	CurrencyLTCT = monetary.NewCurrency("LTCT test coins", "LTCT", 8)

	// currencySymbols maps known currency objects to the currency symbols
	// as recognized on coinpayments.net. In many cases, the currency's own
	// idea of its symbol (currency.Symbol()) will be the same as this
	// CurrencySymbol, but we should probably not count on that always being
	// the case.
	currencySymbols = map[*monetary.Currency]CurrencySymbol{
		monetary.USDollars:  "USD",
		monetary.StorjToken: "STORJ",
		monetary.Bitcoin:    "BTC",
		CurrencyLTCT:        "LTCT",
	}
)
