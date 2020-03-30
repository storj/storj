// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that credit implements payments.CreditsDB.
var _ stripecoinpayments.CreditsDB = (*credit)(nil)

// credit is an implementation of payments.CreditsDB.
//
// architecture: Database
type credit struct {
	db *satelliteDB
}

// InsertCredit inserts credit into the database.
func (credits *credit) InsertCredit(ctx context.Context, credit payments.Credit) (err error) {
	defer mon.Task()(&ctx, credit)(&err)

	_, err = credits.db.Create_Credit(
		ctx,
		dbx.Credit_UserId(credit.UserID[:]),
		dbx.Credit_TransactionId(string(credit.TransactionID[:])),
		dbx.Credit_Amount(credit.Amount),
	)

	return err
}

// GetCredit returns credit by transactionID.
func (credits *credit) GetCredit(ctx context.Context, transactionID coinpayments.TransactionID) (_ payments.Credit, err error) {
	defer mon.Task()(&ctx, transactionID)(&err)

	dbxCredit, err := credits.db.Get_Credit_By_TransactionId(ctx, dbx.Credit_TransactionId(string(transactionID)))
	if err != nil {
		return payments.Credit{}, err
	}

	return fromDBXCredit(dbxCredit)
}

// ListCredits returns all credits of specified user.
func (credits *credit) ListCredits(ctx context.Context, userID uuid.UUID) (_ []payments.Credit, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	dbxCredits, err := credits.db.All_Credit_By_UserId_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.Credit_UserId(userID[:]),
	)
	if err != nil {
		return nil, err
	}

	return creditsFromDbxSlice(dbxCredits)
}

// ListCreditsPaged returns paginated list of user's credits.
func (credits *credit) ListCreditsPaged(ctx context.Context, offset int64, limit int, before time.Time, userID uuid.UUID) (_ payments.CreditsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	var page payments.CreditsPage

	dbxCredits, err := credits.db.Limited_Credit_By_UserId_And_CreatedAt_LessOrEqual_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.Credit_UserId(userID[:]),
		dbx.Credit_CreatedAt(before.UTC()),
		limit+1,
		offset,
	)
	if err != nil {
		return payments.CreditsPage{}, err
	}

	if len(dbxCredits) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit)

		dbxCredits = dbxCredits[:len(dbxCredits)-1]
	}

	page.Credits, err = creditsFromDbxSlice(dbxCredits)
	if err != nil {
		return payments.CreditsPage{}, nil
	}

	return page, nil
}

// InsertCreditsSpending inserts spending into the database.
func (credits *credit) InsertCreditsSpending(ctx context.Context, spending stripecoinpayments.CreditsSpending) (err error) {
	defer mon.Task()(&ctx, spending)(&err)

	id, err := uuid.New()
	if err != nil {
		return err
	}

	_, err = credits.db.Create_CreditsSpending(
		ctx,
		dbx.CreditsSpending_Id(id[:]),
		dbx.CreditsSpending_UserId(spending.UserID[:]),
		dbx.CreditsSpending_ProjectId(spending.ProjectID[:]),
		dbx.CreditsSpending_Amount(spending.Amount),
		dbx.CreditsSpending_Status(int(spending.Status)),
	)

	return err
}

// ListCreditsSpendings returns all spendings of specified user.
func (credits *credit) ListCreditsSpendings(ctx context.Context, userID uuid.UUID) (_ []stripecoinpayments.CreditsSpending, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	dbxSpendings, err := credits.db.All_CreditsSpending_By_UserId_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.CreditsSpending_UserId(userID[:]),
	)
	if err != nil {
		return nil, err
	}

	return creditsSpendingsFromDbxSlice(dbxSpendings)
}

// ApplyCreditsSpending applies spending and updates its status.
func (credits *credit) ApplyCreditsSpending(ctx context.Context, spendingID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = credits.db.Update_CreditsSpending_By_Id(
		ctx,
		dbx.CreditsSpending_Id(spendingID[:]),
		dbx.CreditsSpending_Update_Fields{Status: dbx.CreditsSpending_Status(int(stripecoinpayments.CreditsSpendingStatusApplied))},
	)

	return err
}

// ListCreditsSpendingsPaged returns paginated list of user's spendings.
func (credits *credit) ListCreditsSpendingsPaged(ctx context.Context, status int, offset int64, limit int, before time.Time) (_ stripecoinpayments.CreditsSpendingsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	var page stripecoinpayments.CreditsSpendingsPage

	dbxSpendings, err := credits.db.Limited_CreditsSpending_By_CreatedAt_LessOrEqual_And_Status_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.CreditsSpending_CreatedAt(before.UTC()),
		dbx.CreditsSpending_Status(status),
		limit+1,
		offset,
	)
	if err != nil {
		return stripecoinpayments.CreditsSpendingsPage{}, err
	}

	if len(dbxSpendings) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit)

		dbxSpendings = dbxSpendings[:len(dbxSpendings)-1]
	}

	page.Spendings, err = creditsSpendingsFromDbxSlice(dbxSpendings)
	if err != nil {
		return stripecoinpayments.CreditsSpendingsPage{}, nil
	}

	return page, nil
}

// Balance returns difference between earned for deposit and spent on invoices credits.
func (credits *credit) Balance(ctx context.Context, userID uuid.UUID) (balance int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var creditsAmount, creditsSpendingsAmount int64

	allCredits, err := credits.ListCredits(ctx, userID)
	if err != nil {
		return 0, err
	}

	allSpendings, err := credits.ListCreditsSpendings(ctx, userID)
	if err != nil {
		return 0, err
	}

	for i := range allCredits {
		creditsAmount += allCredits[i].Amount
	}

	for j := range allSpendings {
		creditsSpendingsAmount += allSpendings[j].Amount
	}

	balance = creditsAmount - creditsSpendingsAmount
	return balance, nil
}

// fromDBXCredit converts *dbx.Credit to *payments.Credit.
func fromDBXCredit(dbxCredit *dbx.Credit) (credit payments.Credit, err error) {
	credit.TransactionID = coinpayments.TransactionID(dbxCredit.TransactionId)
	credit.UserID, err = dbutil.BytesToUUID(dbxCredit.UserId)
	if err != nil {
		return payments.Credit{}, err
	}

	credit.Created = dbxCredit.CreatedAt
	credit.Amount = dbxCredit.Amount

	return credit, nil
}

// creditsFromDbxSlice is used for creating []payments.CreditsDB entities from autogenerated []dbx.CreditsDB struct.
func creditsFromDbxSlice(creditsDbx []*dbx.Credit) (_ []payments.Credit, err error) {
	var credits = make([]payments.Credit, 0)
	var errors []error

	// Generating []dbo from []dbx and collecting all errors
	for _, creditDbx := range creditsDbx {
		credit, err := fromDBXCredit(creditDbx)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		credits = append(credits, credit)
	}

	return credits, errs.Combine(errors...)
}

// fromDBXCreditsSpending converts *dbx.Spending to *payments.Spending.
func fromDBXSpending(dbxSpending *dbx.CreditsSpending) (spending stripecoinpayments.CreditsSpending, err error) {
	spending.UserID, err = dbutil.BytesToUUID(dbxSpending.UserId)
	if err != nil {
		return stripecoinpayments.CreditsSpending{}, err
	}

	spending.ProjectID, err = dbutil.BytesToUUID(dbxSpending.ProjectId)
	if err != nil {
		return stripecoinpayments.CreditsSpending{}, err
	}

	spending.Status = stripecoinpayments.CreditsSpendingStatus(dbxSpending.Status)
	spending.Created = dbxSpending.CreatedAt
	spending.Amount = dbxSpending.Amount
	spendingID, err := dbutil.BytesToUUID(dbxSpending.Id)
	if err != nil {
		return stripecoinpayments.CreditsSpending{}, err
	}

	spending.ID = spendingID

	return spending, nil
}

// creditsSpendingsFromDbxSlice is used for creating []payments.CreditSpendings entities from autogenerated []dbx.CreditsSpending struct.
func creditsSpendingsFromDbxSlice(spendingsDbx []*dbx.CreditsSpending) (_ []stripecoinpayments.CreditsSpending, err error) {
	var spendings = make([]stripecoinpayments.CreditsSpending, 0)
	var errors []error

	// Generating []dbo from []dbx and collecting all errors
	for _, spendingDbx := range spendingsDbx {
		spending, err := fromDBXSpending(spendingDbx)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		spendings = append(spendings, spending)
	}

	return spendings, errs.Combine(errors...)
}
