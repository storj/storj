// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"math/big"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// StorjTokens defines all payments STORJ token related functionality.
type StorjTokens interface {
	// Deposit creates deposit transaction for specified amount.
	Deposit(ctx context.Context, userID uuid.UUID, amount big.Float) (*Transaction, error)
}

// TransactionStatus defines allowed statuses
// for deposit transactions.
type TransactionStatus string

const (
	// TransactionStatusPaid is a transaction which successfully received required funds.
	TransactionStatusPaid TransactionStatus = "paid"
	// TransactionStatusPending is a transaction which accepts funds.
	TransactionStatusPending TransactionStatus = "pending"
	// TransactionStatusCancelled is a transaction that is cancelled and no longer accepting new funds.
	TransactionStatusCancelled TransactionStatus = "cancelled"
)

// TransactionID is a transaction ID type.
type TransactionID []byte

// Transaction defines deposit transaction which
// accepts user funds on a specific wallet address.
type Transaction struct {
	ID        TransactionID
	AccountID uuid.UUID
	Amount    big.Float
	Received  big.Float
	Address   string
	Status    TransactionStatus
	CreatedAt time.Time
}
