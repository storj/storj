// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that *billingDB implements billing.TransactionsDB.
var _ billing.TransactionsDB = (*billingDB)(nil)

// billingDB is billing DB.
//
// architecture: Database
type billingDB struct {
	db *satelliteDB
}

func (db billingDB) Insert(ctx context.Context, tx billing.Transaction) (err error) {
	defer mon.Task()(&ctx)(&err)
	return db.db.CreateNoReturn_BillingTransaction(ctx,
		dbx.BillingTransaction_TxId([]byte(tx.TXID)),
		dbx.BillingTransaction_UserId(tx.AccountID[:]),
		dbx.BillingTransaction_Amount(tx.Amount.BaseUnits()),
		dbx.BillingTransaction_Currency(tx.Amount.Currency().Symbol()),
		dbx.BillingTransaction_Description(tx.Description),
		dbx.BillingTransaction_Type(tx.TXType.Int()),
		dbx.BillingTransaction_Timestamp(tx.Timestamp))
}

func (db billingDB) List(ctx context.Context, userID uuid.UUID) (txs []billing.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTXs, err := db.db.All_BillingTransaction_By_UserId_OrderBy_Desc_Timestamp(ctx,
		dbx.BillingTransaction_UserId(userID[:]))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, dbxTX := range dbxTXs {
		tx, err := fromDBXBillingTransaction(dbxTX)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		txs = append(txs, *tx)
	}

	return txs, nil
}

func (db billingDB) ListType(ctx context.Context, userID uuid.UUID, txType billing.TXType) (txs []billing.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTXs, err := db.db.All_BillingTransaction_By_UserId_And_Type_OrderBy_Desc_Timestamp(ctx,
		dbx.BillingTransaction_UserId(userID[:]),
		dbx.BillingTransaction_Type(txType.Int()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, dbxTX := range dbxTXs {
		tx, err := fromDBXBillingTransaction(dbxTX)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		txs = append(txs, *tx)
	}

	return txs, nil
}

func (db billingDB) ComputeBalance(ctx context.Context, userID uuid.UUID) (_ monetary.Amount, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTXs, err := db.db.All_BillingTransaction_By_UserId_OrderBy_Desc_Timestamp(ctx,
		dbx.BillingTransaction_UserId(userID[:]))
	if err != nil {
		return monetary.Amount{}, Error.Wrap(err)
	}

	var balance int64
	for _, dbxTX := range dbxTXs {
		if dbxTX.Currency == monetary.USDollars.Symbol() {
			balance += dbxTX.Amount
		}
	}

	return monetary.AmountFromBaseUnits(balance, monetary.USDollars), nil
}

// fromDBXBillingTransaction converts *dbx.BillingTransaction to *billing.Transaction.
func fromDBXBillingTransaction(dbxTX *dbx.BillingTransaction) (*billing.Transaction, error) {
	userID, err := uuid.FromBytes(dbxTX.UserId)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &billing.Transaction{
		TXID:        string(dbxTX.TxId),
		AccountID:   userID,
		Amount:      monetary.AmountFromBaseUnits(dbxTX.Amount, monetary.USDollars),
		Description: dbxTX.Description,
		TXType:      billing.TXType(dbxTX.Type),
		Timestamp:   dbxTX.Timestamp,
		CreatedAt:   dbxTX.CreatedAt,
	}, nil
}
