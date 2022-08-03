// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/monetary"
)

// TransactionStatus indicates transaction status.
type TransactionStatus string

const (
	// TransactionStatusPending indicates that status of this transaction is pending.
	TransactionStatusPending = "pending"
	// TransactionStatusCancelled indicates that status of this transaction is cancelled.
	TransactionStatusCancelled = "cancelled"
	// TransactionStatusComplete indicates that status of this transaction is complete.
	TransactionStatusComplete = "complete"
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
	// Insert inserts the provided transaction.
	Insert(ctx context.Context, tx Transaction) (txID int64, err error)
	// InsertBatch inserts the provided transactions.
	// Only transactions that increase the user balance can be inserted using batching.
	InsertBatch(ctx context.Context, billingTXs []Transaction) (err error)
	// UpdateStatus updates the status of the transaction.
	UpdateStatus(ctx context.Context, txID int64, status TransactionStatus) error
	// UpdateMetadata updates the metadata of the transaction.
	UpdateMetadata(ctx context.Context, txID int64, metadata []byte) error
	// LastTransaction returns the timestamp of the last known transaction for given source and type.
	LastTransaction(ctx context.Context, txSource string, txType TransactionType) (time.Time, error)
	// List returns all transactions for the specified user.
	List(ctx context.Context, userID uuid.UUID) ([]Transaction, error)
	// GetBalance returns the current usable balance for the specified user.
	GetBalance(ctx context.Context, userID uuid.UUID) (int64, error)
}

// Transaction defines billing related transaction info that is stored in the DB.
type Transaction struct {
	ID          int64
	UserID      uuid.UUID
	Amount      monetary.Amount
	Description string
	Source      string
	Status      TransactionStatus
	Type        TransactionType
	Metadata    []byte
	Timestamp   time.Time
	CreatedAt   time.Time
}
