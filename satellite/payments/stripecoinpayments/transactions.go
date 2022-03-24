// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/monetary"
)

// ErrTransactionConsumed is thrown when trying to consume already consumed transaction.
var ErrTransactionConsumed = errs.New("error transaction already consumed")

// TransactionsDB is an interface which defines functionality
// of DB which stores coinpayments transactions.
//
// architecture: Database
type TransactionsDB interface {
	// Insert inserts new coinpayments transaction into DB.
	Insert(ctx context.Context, tx Transaction) (time.Time, error)
	// Update updates status and received for set of transactions.
	Update(ctx context.Context, updates []TransactionUpdate, applies coinpayments.TransactionIDList) error
	// Consume marks transaction as consumed, so it won't participate in apply account balance loop.
	Consume(ctx context.Context, id coinpayments.TransactionID) error
	// LockRate locks conversion rate for transaction.
	LockRate(ctx context.Context, id coinpayments.TransactionID, rate decimal.Decimal) error
	// GetLockedRate returns locked conversion rate for transaction or error if non exists.
	GetLockedRate(ctx context.Context, id coinpayments.TransactionID) (decimal.Decimal, error)
	// ListAccount returns all transaction for specific user.
	ListAccount(ctx context.Context, userID uuid.UUID) ([]Transaction, error)
	// ListPending returns TransactionsPage with pending transactions.
	ListPending(ctx context.Context, offset int64, limit int, before time.Time) (TransactionsPage, error)
	// ListUnapplied returns TransactionsPage with completed transaction that should be applied to account balance.
	ListUnapplied(ctx context.Context, offset int64, limit int, before time.Time) (TransactionsPage, error)
}

// Transaction defines coinpayments transaction info that is stored in the DB.
type Transaction struct {
	ID        coinpayments.TransactionID
	AccountID uuid.UUID
	Address   string
	Amount    monetary.Amount
	Received  monetary.Amount
	Status    coinpayments.Status
	Key       string
	Timeout   time.Duration
	CreatedAt time.Time
}

// TransactionUpdate holds transaction update info.
type TransactionUpdate struct {
	TransactionID coinpayments.TransactionID
	Status        coinpayments.Status
	Received      monetary.Amount
}

// TransactionsPage holds set of transaction and indicates if
// there are more transactions to fetch.
type TransactionsPage struct {
	Transactions []Transaction
	Next         bool
	NextOffset   int64
}

// IDList returns transaction id list of page's transactions.
func (page *TransactionsPage) IDList() TransactionAndUserList {
	var ids = make(TransactionAndUserList)
	for _, tx := range page.Transactions {
		ids[tx.ID] = tx.AccountID
	}
	return ids
}

// CreationTimes returns a map of creation times of page's transactions.
func (page *TransactionsPage) CreationTimes() map[coinpayments.TransactionID]time.Time {
	creationTimes := make(map[coinpayments.TransactionID]time.Time)
	for _, tx := range page.Transactions {
		creationTimes[tx.ID] = tx.CreatedAt
	}
	return creationTimes
}

// TransactionAndUserList is a composite type for storing userID and txID.
type TransactionAndUserList map[coinpayments.TransactionID]uuid.UUID

// IDList returns transaction id list.
func (idMap TransactionAndUserList) IDList() coinpayments.TransactionIDList {
	var list coinpayments.TransactionIDList
	for transactionID := range idMap {
		list = append(list, transactionID)
	}
	return list
}
