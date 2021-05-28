// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/coinpayments"
)

// Credits exposes all needed functionality to manage credits.
//
// architecture: Service
type Credits interface {
	// Create attaches a credit for payment account.
	Create(ctx context.Context, credit Credit) (err error)

	// ListByUserID return list of all credits of specified payment account.
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]Credit, error)
}

// Credit is an entity that holds bonus balance of user, earned by depositing with storj coins.
type Credit struct {
	UserID        uuid.UUID                  `json:"userId"`
	Amount        int64                      `json:"credit"`
	TransactionID coinpayments.TransactionID `json:"transactionId"`
	Created       time.Time                  `json:"created"`
}

// CreditsPage holds set of credits and indicates if
// there are more credits to fetch.
type CreditsPage struct {
	Credits    []Credit
	Next       bool
	NextOffset int64
}
