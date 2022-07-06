// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite/payments/monetary"
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
	// LastBlock returns the highest block known to DB for specified payment status.
	LastBlock(ctx context.Context, status PaymentStatus) (int64, error)
	// DeletePending removes all pending transactions from the DB.
	DeletePending(ctx context.Context) error
}

// PaymentStatus indicates payment status.
type PaymentStatus string

const (
	// PaymentStatusConfirmed indicates that payment has required number of confirmations.
	PaymentStatusConfirmed = "confirmed"
	// PaymentStatusPending indicates that payment has not meet confirmation requirements.
	PaymentStatusPending = "pending"
)

// CachedPayment holds cached data of storjscan payment.
type CachedPayment struct {
	From        blockchain.Address
	To          blockchain.Address
	TokenValue  monetary.Amount
	Status      PaymentStatus
	BlockHash   blockchain.Hash
	BlockNumber int64
	Transaction blockchain.Hash
	LogIndex    int
	Timestamp   time.Time
}
