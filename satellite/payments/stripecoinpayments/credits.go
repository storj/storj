// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

// CreditsDB is an interface for managing credits table.
//
// architecture: Database
type CreditsDB interface {
	// InsertCredit inserts credit to user's credit balance into the database.
	InsertCredit(ctx context.Context, credit payments.Credit) error
	// GetCredit returns credit by transactionID.
	GetCredit(ctx context.Context, transactionID coinpayments.TransactionID) (_ payments.Credit, err error)
	// ListCredits returns all credits of specific user.
	ListCredits(ctx context.Context, userID uuid.UUID) ([]payments.Credit, error)
	// ListCreditsPaged returns all credits of specific user.
	ListCreditsPaged(ctx context.Context, offset int64, limit int, before time.Time, userID uuid.UUID) (payments.CreditsPage, error)

	// InsertCreditsSpending inserts spending to user's spending list into the database.
	InsertCreditsSpending(ctx context.Context, spending CreditsSpending) error
	// ListCreditsSpendings returns spending received for concrete deposit.
	ListCreditsSpendings(ctx context.Context, userID uuid.UUID) ([]CreditsSpending, error)
	// ListCreditsSpendingsPaged returns all spending of specific user.
	ListCreditsSpendingsPaged(ctx context.Context, status int, offset int64, limit int, before time.Time) (CreditsSpendingsPage, error)
	// ApplyCreditsSpending updated spending's status.
	ApplyCreditsSpending(ctx context.Context, spendingID uuid.UUID) (err error)

	// Balance returns difference between all credits and creditsSpendings of specific user.
	Balance(ctx context.Context, userID uuid.UUID) (int64, error)
}

// ensures that credits implements payments.Credits.
var _ payments.Credits = (*credits)(nil)

// credits is an implementation of payments.Credits.
//
// architecture: Service
type credits struct {
	service *Service
}

// CreditsSpending is an entity that holds funds been used from Accounts bonus credit balance.
// Status shows if spending have been used to pay for invoice already or not.
type CreditsSpending struct {
	ID        uuid.UUID             `json:"id"`
	ProjectID uuid.UUID             `json:"projectId"`
	UserID    uuid.UUID             `json:"userId"`
	Amount    int64                 `json:"amount"`
	Status    CreditsSpendingStatus `json:"status"`
	Created   time.Time             `json:"created"`
}

// CreditsSpendingsPage holds set of creditsSpendings and indicates if
// there are more creditsSpendings to fetch.
type CreditsSpendingsPage struct {
	Spendings  []CreditsSpending
	Next       bool
	NextOffset int64
}

// CreditsSpendingStatus indicates the state of the creditsSpending.
type CreditsSpendingStatus int

const (
	// CreditsSpendingStatusUnapplied is a default creditsSpending state.
	CreditsSpendingStatusUnapplied CreditsSpendingStatus = 0
	// CreditsSpendingStatusApplied status indicates that spending was applied.
	CreditsSpendingStatusApplied CreditsSpendingStatus = 1
)

// Create attaches a credit for payment account.
func (credits *credits) Create(ctx context.Context, credit payments.Credit) (err error) {
	defer mon.Task()(&ctx, credit)(&err)

	return Error.Wrap(credits.service.db.Credits().InsertCredit(ctx, credit))
}

// ListByUserID return list of all credits of specified payment account.
func (credits *credits) ListByUserID(ctx context.Context, userID uuid.UUID) (_ []payments.Credit, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	creditsList, err := credits.service.db.Credits().ListCredits(ctx, userID)

	return creditsList, Error.Wrap(err)
}
