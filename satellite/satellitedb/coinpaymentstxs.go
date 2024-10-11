// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/common/currency"
	"storj.io/common/uuid"
	"storj.io/storj/private/slices2"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensure that coinpaymentsTransactions implements stripecoinpayments.TransactionsDB.
var _ stripe.TransactionsDB = (*coinPaymentsTransactions)(nil)

// coinPaymentsTransactions is CoinPayments transactions DB.
//
// architecture: Database
type coinPaymentsTransactions struct {
	db dbx.Methods
}

// GetLockedRate returns locked conversion rate for transaction or error if non exists.
func (db *coinPaymentsTransactions) GetLockedRate(ctx context.Context, id coinpayments.TransactionID) (rate decimal.Decimal, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxRate, err := db.db.Get_StripecoinpaymentsTxConversionRate_By_TxId(ctx,
		dbx.StripecoinpaymentsTxConversionRate_TxId(id.String()),
	)
	if err != nil {
		return decimal.Decimal{}, err
	}

	rate = decimal.NewFromFloat(dbxRate.RateNumeric)
	return rate, nil
}

// ListAccount returns all transaction for specific user.
func (db *coinPaymentsTransactions) ListAccount(ctx context.Context, userID uuid.UUID) (_ []stripe.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTXs, err := db.db.All_CoinpaymentsTransaction_By_UserId_OrderBy_Desc_CreatedAt(ctx,
		dbx.CoinpaymentsTransaction_UserId(userID[:]),
	)
	if err != nil {
		return nil, err
	}

	txs, err := slices2.Convert(dbxTXs, fromDBXCoinpaymentsTransaction)
	return txs, Error.Wrap(err)
}

// TestInsert inserts new coinpayments transaction into DB.
func (db *coinPaymentsTransactions) TestInsert(ctx context.Context, tx stripe.Transaction) (createTime time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxCPTX, err := db.db.Create_CoinpaymentsTransaction(ctx,
		dbx.CoinpaymentsTransaction_Id(tx.ID.String()),
		dbx.CoinpaymentsTransaction_UserId(tx.AccountID[:]),
		dbx.CoinpaymentsTransaction_Address(tx.Address),
		dbx.CoinpaymentsTransaction_AmountNumeric(tx.Amount.BaseUnits()),
		dbx.CoinpaymentsTransaction_ReceivedNumeric(tx.Received.BaseUnits()),
		dbx.CoinpaymentsTransaction_Status(tx.Status.Int()),
		dbx.CoinpaymentsTransaction_Key(tx.Key),
		dbx.CoinpaymentsTransaction_Timeout(int(tx.Timeout.Seconds())),
	)
	if err != nil {
		return time.Time{}, err
	}
	return dbxCPTX.CreatedAt, nil
}

// TestLockRate locks conversion rate for transaction.
func (db *coinPaymentsTransactions) TestLockRate(ctx context.Context, id coinpayments.TransactionID, rate decimal.Decimal) (err error) {
	defer mon.Task()(&ctx)(&err)

	rateFloat, exact := rate.Float64()
	if !exact {
		// It's not clear at the time of writing whether this
		// inexactness will ever be something we need to worry about.
		// According to the example in the API docs for
		// coinpayments.net, exchange rates are given to 24 decimal
		// places (!!), which is several digits more precision than we
		// can represent exactly in IEEE754 double-precision floating
		// point. However, that might not matter, since an exchange rate
		// that is correct to ~15 decimal places multiplied by a precise
		// monetary.Amount should give results that are correct to
		// around 15 decimal places still. At current exchange rates,
		// for example, a USD transaction would need to have a value of
		// more than $1,000,000,000,000 USD before a calculation using
		// this "inexact" rate would get the equivalent number of BTC
		// wrong by a single satoshi (10^-8 BTC).
		//
		// We could avoid all of this by preserving the exact rates as
		// given by our provider, but this would involve either (a)
		// abuse of the SQL schema (e.g. storing rates as decimal values
		// in VARCHAR), (b) storing rates in a way that is opaque to the
		// db engine (e.g. gob-encoding, decimal coefficient with
		// separate exponents), or (c) adding support for parameterized
		// types like NUMERIC to dbx. None of those are very ideal
		// either.
		delta, _ := rate.Sub(decimal.NewFromFloat(rateFloat)).Float64()
		mon.FloatVal("inexact-float64-exchange-rate-delta").Observe(delta)
	}

	_, err = db.db.Create_StripecoinpaymentsTxConversionRate(ctx,
		dbx.StripecoinpaymentsTxConversionRate_TxId(id.String()),
		dbx.StripecoinpaymentsTxConversionRate_RateNumeric(rateFloat),
	)
	return Error.Wrap(err)
}

// fromDBXCoinpaymentsTransaction converts *dbx.CoinpaymentsTransaction to stripecoinpayments.Transaction.
func fromDBXCoinpaymentsTransaction(dbxCPTX *dbx.CoinpaymentsTransaction) (stripe.Transaction, error) {
	userID, err := uuid.FromBytes(dbxCPTX.UserId)
	if err != nil {
		return stripe.Transaction{}, errs.Wrap(err)
	}

	// TODO: the currency here should be passed in to this function or stored
	//  in the database.

	return stripe.Transaction{
		ID:        coinpayments.TransactionID(dbxCPTX.Id),
		AccountID: userID,
		Address:   dbxCPTX.Address,
		Amount:    currency.AmountFromBaseUnits(dbxCPTX.AmountNumeric, currency.StorjToken),
		Received:  currency.AmountFromBaseUnits(dbxCPTX.ReceivedNumeric, currency.StorjToken),
		Status:    coinpayments.Status(dbxCPTX.Status),
		Key:       dbxCPTX.Key,
		Timeout:   time.Second * time.Duration(dbxCPTX.Timeout),
		CreatedAt: dbxCPTX.CreatedAt,
	}, nil
}
