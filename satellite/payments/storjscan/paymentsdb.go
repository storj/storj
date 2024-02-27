// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/currency"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite/payments"
)

// ErrNoPayments represents err when there is no payments in the DB.
var ErrNoPayments = errs.New("no payments in the database")

// PaymentsDB is storjscan payments DB interface.
//
// architecture: Database
type PaymentsDB interface {
	// InsertBatch inserts list of payments into DB.
	InsertBatch(ctx context.Context, payments []CachedPayment) error
	// List returns list of all storjscan payments order by block number and log index desc mainly for testing.
	List(ctx context.Context) ([]CachedPayment, error)
	// ListWallet returns list of storjscan payments order by block number and log index desc.
	ListWallet(ctx context.Context, wallet blockchain.Address, limit int, offset int64) ([]CachedPayment, error)
	// LastBlocks returns the highest blocks known to the DB for each chain with the given payment status.
	LastBlocks(ctx context.Context, status payments.PaymentStatus) (map[int64]int64, error)
	// DeletePending removes all pending transactions from the DB.
	DeletePending(ctx context.Context) error
	// ListConfirmed returns list of confirmed storjscan payments greater than the given timestamp from the specified source and chain.
	ListConfirmed(ctx context.Context, source string, chainID, blockNumber int64, logIndex int) ([]CachedPayment, error)
}

// CachedPayment holds cached data of storjscan payment.
type CachedPayment struct {
	ChainID     int64                  `json:"chainID"`
	From        blockchain.Address     `json:"from"`
	To          blockchain.Address     `json:"to"`
	TokenValue  currency.Amount        `json:"tokenValue"`
	USDValue    currency.Amount        `json:"usdValue"`
	Status      payments.PaymentStatus `json:"status"`
	BlockHash   blockchain.Hash        `json:"blockHash"`
	BlockNumber int64                  `json:"blockNumber"`
	Transaction blockchain.Hash        `json:"transaction"`
	LogIndex    int                    `json:"logIndex"`
	Timestamp   time.Time              `json:"timestamp"`
}
