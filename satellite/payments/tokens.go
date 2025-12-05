// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"storj.io/common/currency"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
)

// StorjTokens defines all payments STORJ token related functionality.
//
// architecture: Service
type StorjTokens interface {
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
	// Payments returns payments for a particular wallet.
	Payments(ctx context.Context, wallet blockchain.Address, limit int, offset int64) ([]WalletPayment, error)
	// PaymentsWithConfirmations returns payments with confirmations count for a particular wallet.
	PaymentsWithConfirmations(ctx context.Context, wallet blockchain.Address) ([]WalletPaymentWithConfirmations, error)
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
	Amount    currency.Amount
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
	Amount        currency.Amount
	Received      currency.Amount
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

// PaymentStatus indicates payment status.
type PaymentStatus string

const (
	// PaymentStatusConfirmed indicates that payment has required number of confirmations.
	PaymentStatusConfirmed = "confirmed"
	// PaymentStatusPending indicates that payment has not meet confirmation requirements.
	PaymentStatusPending = "pending"
)

// WalletPayment holds storj token payment data.
type WalletPayment struct {
	ChainID     int64              `json:"chainID"`
	From        blockchain.Address `json:"from"`
	To          blockchain.Address `json:"to"`
	TokenValue  currency.Amount    `json:"tokenValue"`
	USDValue    currency.Amount    `json:"usdValue"`
	Status      PaymentStatus      `json:"status"`
	BlockHash   blockchain.Hash    `json:"blockHash"`
	BlockNumber int64              `json:"blockNumber"`
	Transaction blockchain.Hash    `json:"transaction"`
	LogIndex    int                `json:"logIndex"`
	Timestamp   time.Time          `json:"timestamp"`
}

// WalletPaymentWithConfirmations holds storj token payment data with confirmations count.
type WalletPaymentWithConfirmations struct {
	ChainID       int64           `json:"chainID"`
	From          string          `json:"from"`
	To            string          `json:"to"`
	TokenValue    decimal.Decimal `json:"tokenValue"`
	USDValue      decimal.Decimal `json:"usdValue"`
	Status        PaymentStatus   `json:"status"`
	BlockHash     string          `json:"blockHash"`
	BlockNumber   int64           `json:"blockNumber"`
	Transaction   string          `json:"transaction"`
	LogIndex      int             `json:"logIndex"`
	Timestamp     time.Time       `json:"timestamp"`
	Confirmations int64           `json:"confirmations"`
	BonusTokens   decimal.Decimal `json:"bonusTokens"`
}
