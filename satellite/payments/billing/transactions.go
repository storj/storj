// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/currency"
	"storj.io/common/uuid"
)

// TransactionStatus indicates transaction status.
type TransactionStatus string

// ErrInsufficientFunds represents err when a user balance is too low for some transaction.
var ErrInsufficientFunds = errs.New("Insufficient funds for this transaction")

// ErrNoWallet represents err when there is no wallet in the DB.
var ErrNoWallet = errs.New("wallet does not exists")

// ErrNoTransactions represents err when there is no billing transactions in the DB.
var ErrNoTransactions = errs.New("no transactions in the database")

const (
	// TransactionStatusPending indicates that status of this transaction is pending.
	TransactionStatusPending = "pending"
	// TransactionStatusCompleted indicates that status of this transaction is complete.
	TransactionStatusCompleted = "complete"
	// TransactionStatusFailed indicates that status of this transaction is failed.
	TransactionStatusFailed = "failed"
)

// TransactionType indicates transaction type.
type TransactionType string

const (
	// TransactionTypeCredit indicates that type of this transaction is credit.
	TransactionTypeCredit = "credit"
	// TransactionTypeDebit indicates that type of this transaction is debit.
	TransactionTypeDebit = "debit"
	// TransactionTypeUnknown indicates that type of this transaction is unknown.
	TransactionTypeUnknown = "unknown"
)

// TransactionsDB is an interface which defines functionality
// of DB which stores billing transactions.
//
// architecture: Database
type TransactionsDB interface {
	// Insert inserts the provided primary transaction along with zero or more
	// supplemental transactions that. This is NOT intended for bulk insertion,
	// but rather to provide an atomic commit of one or more _related_
	// transactions.
	Insert(ctx context.Context, primaryTx Transaction, supplementalTx ...Transaction) (txIDs []int64, err error)
	// FailPendingInvoiceTokenPayments marks all specified pending invoice token payments as failed, and refunds the pending charges.
	FailPendingInvoiceTokenPayments(ctx context.Context, txIDs ...int64) error
	// CompletePendingInvoiceTokenPayments updates the status of the pending invoice token payment to complete.
	CompletePendingInvoiceTokenPayments(ctx context.Context, txIDs ...int64) error
	// UpdateMetadata updates the metadata of the transaction.
	UpdateMetadata(ctx context.Context, txID int64, metadata []byte) error
	// LastTransaction returns the timestamp and metadata of the last known transaction for given source and type.
	LastTransaction(ctx context.Context, txSource string, txType TransactionType) (time.Time, []byte, error)
	// List returns all transactions for the specified user.
	List(ctx context.Context, userID uuid.UUID) ([]Transaction, error)
	// ListSource returns all transactions for the specified user and source.
	ListSource(ctx context.Context, userID uuid.UUID, txSource string) ([]Transaction, error)
	// GetBalance returns the current usable balance for the specified user.
	GetBalance(ctx context.Context, userID uuid.UUID) (currency.Amount, error)
}

// PaymentType is an interface which defines functionality required for all billing payment types. Payment types can
// include but are not limited to Bitcoin, Ether, credit or debit card, ACH transfer, or even physical transfer of live
// goats. In each case, a source, type, and method to get new transactions must be defined by the service, though
// metadata specific to each payment type is also supported (i.e. goat hair type).
type PaymentType interface {
	// Sources the supported sources of the payment type
	Sources() []string
	// Type the type of the payment
	Type() TransactionType
	// GetNewTransactions returns new transactions for a given source that occurred after the provided last transaction received.
	GetNewTransactions(ctx context.Context, source string, lastTransactionTime time.Time, metadata []byte) ([]Transaction, error)
}

// Well-known PaymentType sources.
const (
	StripeSource            = "stripe"
	StorjScanEthereumSource = "ethereum"
	StorjScanZkSyncSource   = "zkSync"
	StorjScanBonusSource    = "storjscanbonus"
)

// SourceChainIDs are some well known chain IDs for the above sources.
var SourceChainIDs = map[string][]int64{
	StorjScanEthereumSource: {1, 4, 5, 1337, 11155111},
	StorjScanZkSyncSource:   {300, 324},
}

// Transaction defines billing related transaction info that is stored in the DB.
type Transaction struct {
	ID          int64
	UserID      uuid.UUID
	Amount      currency.Amount
	Description string
	Source      string
	Status      TransactionStatus
	Type        TransactionType
	Metadata    []byte
	Timestamp   time.Time
	CreatedAt   time.Time
}

// CalculateBonusAmount calculates bonus for given currency amount and bonus rate.
func CalculateBonusAmount(amount currency.Amount, bonusRate int64) currency.Amount {
	bonusUnits := amount.BaseUnits() * bonusRate / 100
	return currency.AmountFromBaseUnits(bonusUnits, amount.Currency())
}

func prepareBonusTransaction(bonusRate int64, source string, transaction Transaction) (Transaction, bool) {
	// Bonus transactions only apply when enabled (i.e. positive rate) and
	// for StorjScan transactions.
	switch {
	case bonusRate <= 0:
		return Transaction{}, false
	case source != StorjScanEthereumSource && source != StorjScanZkSyncSource:
		return Transaction{}, false
	case transaction.Type != TransactionTypeCredit:
		// This is defensive. Storjscan shouldn't provide "debit" transactions.
		return Transaction{}, false
	}

	return Transaction{
		UserID:      transaction.UserID,
		Amount:      CalculateBonusAmount(transaction.Amount, bonusRate),
		Description: fmt.Sprintf("STORJ Token Bonus (%d%%)", bonusRate),
		Source:      StorjScanBonusSource,
		Status:      TransactionStatusCompleted,
		Type:        TransactionTypeCredit,
		Timestamp:   transaction.Timestamp,
		Metadata:    append([]byte(nil), transaction.Metadata...),
	}, true
}
