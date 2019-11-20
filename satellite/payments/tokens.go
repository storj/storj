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
	Deposit(ctx context.Context, userID uuid.UUID, amount *TokenAmount) (*Transaction, error)
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
	Amount    TokenAmount
	Received  TokenAmount
	Address   string
	Status    TransactionStatus
	Timeout   time.Duration
	CreatedAt time.Time
}

// TransactionInfo holds transaction data with additional information
// such as links and expiration time.
type TransactionInfo struct {
	ID            TransactionID
	Amount        TokenAmount
	Received      TokenAmount
	AmountCents   int64
	ReceivedCents int64
	Address       string
	Status        TransactionStatus
	Link          string
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// TokenAmount is a wrapper type for STORJ token amount.
// Uses big.Float as inner representation. Precision is set to 32
// so it can properly handle 8 digits after point which is STORJ token
// decimal set.
type TokenAmount struct {
	inner big.Float
}

// precision defines STORJ token precision.
const precision = 32

// NewTokenAmount creates new zeroed TokenAmount with fixed precision.
func NewTokenAmount() *TokenAmount {
	return &TokenAmount{inner: *new(big.Float).SetPrec(precision)}
}

// BigFloat returns inner representation of TokenAmount.
func (amount *TokenAmount) BigFloat() *big.Float {
	f := new(big.Float).Set(&amount.inner)
	return f
}

// String representation of TokenValue.
func (amount *TokenAmount) String() string {
	return amount.inner.Text('f', -1)
}

// ParseTokenAmount parses string representing floating point and returns
// TokenAmount.
func ParseTokenAmount(s string) (*TokenAmount, error) {
	inner, _, err := big.ParseFloat(s, 10, precision, big.ToNearestEven)
	if err != nil {
		return nil, err
	}
	return &TokenAmount{inner: *inner}, nil
}

// TokenAmountFromBigFloat converts big.Float to TokenAmount.
func TokenAmountFromBigFloat(f *big.Float) *TokenAmount {
	inner := (*f).SetMode(big.ToNearestEven).SetPrec(precision)
	return &TokenAmount{inner: *inner}
}
