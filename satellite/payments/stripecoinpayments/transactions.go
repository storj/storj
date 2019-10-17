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
