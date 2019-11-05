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
	// ListTransactionInfos returns all transaction associated with user.
	ListTransactionInfos(ctx context.Context, userID uuid.UUID) ([]TransactionInfo, error)
}

// TransactionStatus defines allowed statuses
// for deposit transactions.
type TransactionStatus string

// String returns string representation of transaction status.
func (status TransactionStatus) String() string {
	return string(status)
}

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

// String returns string representation of transaction id.
func (id TransactionID) String() string {
	return string(id)
}

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

// TransactionInfo holds transaction data with additional information
// such as links and expiration time.
type TransactionInfo struct {
	ID        TransactionID
	Amount    big.Float
	Received  big.Float
	Address   string
	Status    TransactionStatus
	Link      string
	ExpiresAt time.Time
	CreatedAt time.Time
}
