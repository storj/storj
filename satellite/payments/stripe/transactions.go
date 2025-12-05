// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"storj.io/common/currency"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/coinpayments"
)

// TransactionsDB is an interface which defines functionality
// of DB which stores coinpayments transactions.
//
// architecture: Database
type TransactionsDB interface {
	// GetLockedRate returns locked conversion rate for transaction or error if non exists.
	GetLockedRate(ctx context.Context, id coinpayments.TransactionID) (decimal.Decimal, error)
	// ListAccount returns all transaction for specific user.
	ListAccount(ctx context.Context, userID uuid.UUID) ([]Transaction, error)
	// TestInsert inserts new coinpayments transaction into DB.
	TestInsert(ctx context.Context, tx Transaction) (time.Time, error)
	// TestLockRate locks conversion rate for transaction.
	TestLockRate(ctx context.Context, id coinpayments.TransactionID, rate decimal.Decimal) error
}

// Transaction defines coinpayments transaction info that is stored in the DB.
type Transaction struct {
	ID        coinpayments.TransactionID
	AccountID uuid.UUID
	Address   string
	Amount    currency.Amount
	Received  currency.Amount
	Status    coinpayments.Status
	Key       string
	Timeout   time.Duration
	CreatedAt time.Time
}
