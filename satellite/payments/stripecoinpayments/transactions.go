// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"math/big"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments/coinpayments"
)

// TransactionsDB is an interface which defines functionality
// of DB which stores coinpayments transactions.
//
// architecture: Database
type TransactionsDB interface {
	// Insert inserts new coinpayments transaction into DB.
	Insert(ctx context.Context, tx Transaction) (*Transaction, error)
	// Update updates status and received for set of transactions.
	Update(ctx context.Context, updates []TransactionUpdate, applies coinpayments.TransactionIDList) error
	// Consume marks transaction as consumed, so it won't participate in apply account balance loop.
	Consume(ctx context.Context, id coinpayments.TransactionID) error
	// ListPending returns TransactionsPage with pending transactions.
	ListPending(ctx context.Context, offset int64, limit int, before time.Time) (TransactionsPage, error)
	// List Unapplied returns TransactionsPage with transactions completed transaction that should be applied to account balance.
	ListUnapplied(ctx context.Context, offset int64, limit int, before time.Time) (TransactionsPage, error)
}

// Transaction defines coinpayments transaction info that is stored in the DB.
type Transaction struct {
	ID        coinpayments.TransactionID
	AccountID uuid.UUID
	Address   string
	Amount    big.Float
	Received  big.Float
	Status    coinpayments.Status
	Key       string
	CreatedAt time.Time
}

// TransactionUpdate holds transaction update info.
type TransactionUpdate struct {
	TransactionID coinpayments.TransactionID
	Status        coinpayments.Status
	Received      big.Float
}

// TransactionsPage holds set of transaction and indicates if
// there are more transactions to fetch.
type TransactionsPage struct {
	Transactions []Transaction
	Next         bool
	NextOffset   int64
}

// IDList returns transaction id list of page's transactions.
func (page *TransactionsPage) IDList() coinpayments.TransactionIDList {
	var list coinpayments.TransactionIDList
	for _, tx := range page.Transactions {
		list = append(list, tx.ID)
	}
	return list
}
