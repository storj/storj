// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"github.com/shopspring/decimal"
)

// Balance is an entity that holds free credits and coins balance of user.
// Earned by applying of promotional coupon and coins depositing, respectively.
type Balance struct {
	FreeCredits int64           `json:"freeCredits"`
	Coins       decimal.Decimal `json:"coins"` // STORJ token balance from storjscan.
	Credits     decimal.Decimal `json:"credits"`
	// Credits is the balance (in cents) from stripe. This may include the following.
	// 1. legacy Coinpayments deposit.
	// 2. legacy credit for a manual STORJ deposit.
	// 4. bonus manually credited for a storjscan payment once a month before  invoicing.
	// 5. any other adjustment we may have to make from time to time manually to the customer´s STORJ balance.
}
