// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"math/big"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensure that coinpaymentsTransaction implements stripecoinpayments.TransactionsDB.
var _ stripecoinpayments.TransactionsDB = (*coinpaymentsTransactions)(nil)

// coinpaymentsTransactions is Coinpayments transactions DB.
//
// architecture: Database
type coinpaymentsTransactions struct {
	db *dbx.DB
}

// Insert inserts new coinpayments transaction into DB.
func (db *coinpaymentsTransactions) Insert(ctx context.Context, tx stripecoinpayments.Transaction) (*stripecoinpayments.Transaction, error) {
	amount, err := tx.Amount.GobEncode()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	received, err := tx.Received.GobEncode()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	dbxCPTX, err := db.db.Create_CoinpaymentsTransaction(ctx,
		dbx.CoinpaymentsTransaction_Id(tx.ID.String()),
		dbx.CoinpaymentsTransaction_UserId(tx.AccountID[:]),
		dbx.CoinpaymentsTransaction_Address(tx.Address),
		dbx.CoinpaymentsTransaction_Amount(amount),
		dbx.CoinpaymentsTransaction_Received(received),
		dbx.CoinpaymentsTransaction_Status(tx.Status.Int()),
		dbx.CoinpaymentsTransaction_Key(tx.Key),
	)
	if err != nil {
		return nil, err
	}

	return fromDBXCoinpaymentsTransaction(dbxCPTX)
}

// fromDBXCoinpaymentsTransaction converts *dbx.CoinpaymentsTransaction to *stripecoinpayments.Transaction.
func fromDBXCoinpaymentsTransaction(dbxCPTX *dbx.CoinpaymentsTransaction) (*stripecoinpayments.Transaction, error) {
	userID, err := bytesToUUID(dbxCPTX.UserId)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var amount, received big.Float
	if err := amount.GobDecode(dbxCPTX.Amount); err != nil {
		return nil, errs.Wrap(err)
	}
	if err := received.GobDecode(dbxCPTX.Received); err != nil {
		return nil, errs.Wrap(err)
	}

	return &stripecoinpayments.Transaction{
		ID:        coinpayments.TransactionID(dbxCPTX.Id),
		AccountID: userID,
		Address:   dbxCPTX.Address,
		Amount:    amount,
		Received:  received,
		Status:    coinpayments.Status(dbxCPTX.Status),
		Key:       dbxCPTX.Key,
		CreatedAt: dbxCPTX.CreatedAt,
	}, nil
}
