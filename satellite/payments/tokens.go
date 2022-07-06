// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite/payments/monetary"
)

// StorjTokens defines all payments STORJ token related functionality.
//
// architecture: Service
type StorjTokens interface {
	// Deposit creates deposit transaction for specified amount in cents.
	Deposit(ctx context.Context, userID uuid.UUID, amount int64) (*Transaction, error)
	// ListTransactionInfos returns all transactions associated with user.
	ListTransactionInfos(ctx context.Context, userID uuid.UUID) ([]TransactionInfo, error)
	// ListDepositBonuses returns all deposit bonuses associated with user.
	ListDepositBonuses(ctx context.Context, userID uuid.UUID) ([]DepositBonus, error)
}

// DepositWallets exposes all needed functionality to manage token deposit wallets.
//
// architecture: Service
type DepositWallets interface {
	// Claim gets a new crypto wallet and associates it with a user.
	Claim(ctx context.Context, userID uuid.UUID) (blockchain.Address, error)
	// Get returns the crypto wallet address associated with the given user.
	Get(ctx context.Context, userID uuid.UUID) (blockchain.Address, error)
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
	Amount    monetary.Amount
	Rate      decimal.Decimal
	Address   string
	Status    TransactionStatus
	Timeout   time.Duration
	Link      string
	CreatedAt time.Time
}

// TransactionInfo holds transaction data with additional information
// such as links and expiration time.
type TransactionInfo struct {
	ID            TransactionID
	Amount        monetary.Amount
	Received      monetary.Amount
	AmountCents   int64
	ReceivedCents int64
	Address       string
	Status        TransactionStatus
	Link          string
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// DepositBonus defines a bonus received for depositing tokens.
type DepositBonus struct {
	TransactionID TransactionID
	AmountCents   int64
	Percentage    int64
	CreatedAt     time.Time
}
