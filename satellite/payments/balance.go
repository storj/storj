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
	Coins       decimal.Decimal `json:"coins"`
}
