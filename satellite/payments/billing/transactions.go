// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import (
	"context"
	"fmt"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/monetary"
)

// TXType is a type wrapper for transaction types.
type TXType int

const (
	// Storjscan defines transactions which are originated from storjscan payment wallets.
	Storjscan TXType = iota
	// Stripe defines transactions which are Stripe processed credits and/or debits.
	Stripe
	// Coinpayments defines transactions which are originated from coinpayments.
	Coinpayments
)

// Int returns int representation of transaction type.
func (t TXType) Int() int {
	return int(t)
}

// String returns string representation of transaction type.
func (t TXType) String() string {
	switch t {
	case Storjscan:
		return "Storjscan"
	case Stripe:
		return "Stripe"
	case Coinpayments:
		return "Coinpayments"
	default:
		return fmt.Sprintf("%d", int(t))
	}
}

// TransactionsDB is an interface which defines functionality
// of DB which stores billing transactions.
//
// architecture: Database
type TransactionsDB interface {
	// Insert inserts the provided transaction.
	Insert(ctx context.Context, tx Transaction) error
	// List returns all transactions for the specified user.
	List(ctx context.Context, userID uuid.UUID) ([]Transaction, error)
	// ListType returns all transactions of a given type for the specified user.
	ListType(ctx context.Context, userID uuid.UUID, txType TXType) ([]Transaction, error)
	// ComputeBalance returns the current usable balance for the specified user.
	ComputeBalance(ctx context.Context, userID uuid.UUID) (monetary.Amount, error)
}

// Transaction defines billing related transaction info that is stored in the DB.
type Transaction struct {
	TXID        string
	AccountID   uuid.UUID
	Amount      monetary.Amount
	Description string
	TXType      TXType
	Timestamp   time.Time
	CreatedAt   time.Time
}
