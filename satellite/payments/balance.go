// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/shopspring/decimal"

	"storj.io/common/uuid"
)

// Balances exposes needed functionality for managing customer balances.
type Balances interface {
	// ApplyCredit applies a credit of `amount` to the user's stripe balance with a description of `desc`.
	ApplyCredit(ctx context.Context, userID uuid.UUID, amount int64, desc, idempotencyKey string) (*Balance, error)
	// Get returns the customer balance.
	Get(ctx context.Context, userID uuid.UUID) (Balance, error)
	// ListTransactions returns a list of transactions on the customer's balance.
	ListTransactions(ctx context.Context, userID uuid.UUID) ([]BalanceTransaction, error)
}

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

// BalanceTransaction represents a single transaction affecting a customer balance.
type BalanceTransaction struct {
	ID          string
	Amount      int64
	Description string
}

// PackagePlan is an amount to charge a user one time in exchange for credit of greater value.
// Price and Credit are in cents USD.
type PackagePlan struct {
	Price  int64
	Credit int64
}
