// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"math/big"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensure that coinpaymentsTransactions implements stripecoinpayments.TransactionsDB.
var _ stripecoinpayments.TransactionsDB = (*coinPaymentsTransactions)(nil)

// applyBalanceIntentState defines states of the apply balance intents.
type applyBalanceIntentState int

const (
	// apply balance intent waits to be applied.
	applyBalanceIntentStateUnapplied applyBalanceIntentState = 0
	// transaction which balance intent points to has been consumed.
	applyBalanceIntentStateConsumed applyBalanceIntentState = 1
)

// Int returns intent state as int.
func (intent applyBalanceIntentState) Int() int {
	return int(intent)
}

// coinPaymentsTransactions is CoinPayments transactions DB.
//
// architecture: Database
type coinPaymentsTransactions struct {
	db *satelliteDB
}

// Insert inserts new coinpayments transaction into DB.
func (db *coinPaymentsTransactions) Insert(ctx context.Context, tx stripecoinpayments.Transaction) (createTime time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxCPTX, err := db.db.Create_CoinpaymentsTransaction(ctx,
		dbx.CoinpaymentsTransaction_Id(tx.ID.String()),
		dbx.CoinpaymentsTransaction_UserId(tx.AccountID[:]),
		dbx.CoinpaymentsTransaction_Address(tx.Address),
		dbx.CoinpaymentsTransaction_Status(tx.Status.Int()),
		dbx.CoinpaymentsTransaction_Key(tx.Key),
		dbx.CoinpaymentsTransaction_Timeout(int(tx.Timeout.Seconds())),
		dbx.CoinpaymentsTransaction_Create_Fields{
			AmountNumeric:   dbx.CoinpaymentsTransaction_AmountNumeric(tx.Amount.BaseUnits()),
			ReceivedNumeric: dbx.CoinpaymentsTransaction_ReceivedNumeric(tx.Received.BaseUnits()),
		},
	)
	if err != nil {
		return time.Time{}, err
	}
	return dbxCPTX.CreatedAt, nil
}

// Update updates status and received for set of transactions.
func (db *coinPaymentsTransactions) Update(ctx context.Context, updates []stripecoinpayments.TransactionUpdate, applies coinpayments.TransactionIDList) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(updates) == 0 {
		return nil
	}

	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, update := range updates {
			_, err = tx.Update_CoinpaymentsTransaction_By_Id(ctx,
				dbx.CoinpaymentsTransaction_Id(update.TransactionID.String()),
				dbx.CoinpaymentsTransaction_Update_Fields{
					ReceivedNumeric: dbx.CoinpaymentsTransaction_ReceivedNumeric(update.Received.BaseUnits()),
					ReceivedGob:     dbx.CoinpaymentsTransaction_ReceivedGob_Null(),
					Status:          dbx.CoinpaymentsTransaction_Status(update.Status.Int()),
				},
			)
			if err != nil {
				return err
			}
		}

		for _, txID := range applies {
			query := db.db.Rebind(`INSERT INTO stripecoinpayments_apply_balance_intents ( tx_id, state, created_at )
			VALUES ( ?, ?, ? ) ON CONFLICT DO NOTHING`)
			_, err = tx.Tx.ExecContext(ctx, query, txID.String(), applyBalanceIntentStateUnapplied.Int(), db.db.Hooks.Now().UTC())
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

// Consume marks transaction as consumed, so it won't participate in apply account balance loop.
func (db *coinPaymentsTransactions) Consume(ctx context.Context, id coinpayments.TransactionID) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := db.db.Rebind(`
		WITH intent AS (
			SELECT tx_id, state FROM stripecoinpayments_apply_balance_intents WHERE tx_id = ?
		), updated AS (
			UPDATE stripecoinpayments_apply_balance_intents AS ints
				SET
					state = ?
				FROM intent
				WHERE intent.tx_id = ints.tx_id  AND ints.state = ?
			RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM intent) AS intent_exists, EXISTS(SELECT 1 FROM updated) AS intent_consumed;
	`)

	row := db.db.QueryRowContext(ctx, query, id, applyBalanceIntentStateConsumed, applyBalanceIntentStateUnapplied)

	var exists, consumed bool
	if err = row.Scan(&exists, &consumed); err != nil {
		return err
	}

	if !exists {
		return errs.New("can not consume transaction without apply balance intent")
	}
	if !consumed {
		return stripecoinpayments.ErrTransactionConsumed
	}

	return err
}

// LockRate locks conversion rate for transaction.
func (db *coinPaymentsTransactions) LockRate(ctx context.Context, id coinpayments.TransactionID, rate decimal.Decimal) (err error) {
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
		dbx.StripecoinpaymentsTxConversionRate_Create_Fields{
			RateNumeric: dbx.StripecoinpaymentsTxConversionRate_RateNumeric(rateFloat),
		})
	return Error.Wrap(err)
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

	if dbxRate.RateNumeric == nil {
		// This row does not have a numeric rate value yet
		var rateF big.Float
		if err = rateF.GobDecode(dbxRate.RateGob); err != nil {
			return decimal.Decimal{}, Error.Wrap(err)
		}
		rate, err = monetary.DecimalFromBigFloat(&rateF)
		return rate, Error.Wrap(err)
	}

	rate = decimal.NewFromFloat(*dbxRate.RateNumeric)
	return rate, nil
}

// ListAccount returns all transaction for specific user.
func (db *coinPaymentsTransactions) ListAccount(ctx context.Context, userID uuid.UUID) (_ []stripecoinpayments.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTXs, err := db.db.All_CoinpaymentsTransaction_By_UserId_OrderBy_Desc_CreatedAt(ctx,
		dbx.CoinpaymentsTransaction_UserId(userID[:]),
	)
	if err != nil {
		return nil, err
	}

	var txs []stripecoinpayments.Transaction
	for _, dbxTX := range dbxTXs {
		tx, err := fromDBXCoinpaymentsTransaction(dbxTX)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		txs = append(txs, *tx)
	}

	return txs, nil
}

// ListPending returns paginated list of pending transactions.
func (db *coinPaymentsTransactions) ListPending(ctx context.Context, offset int64, limit int, before time.Time) (_ stripecoinpayments.TransactionsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	query := db.db.Rebind(`SELECT
				id,
				user_id,
				address,
				amount_gob,
				amount_numeric,
				received_gob,
				received_numeric,
				status,
				key,
				created_at
			FROM coinpayments_transactions
			WHERE status IN (?,?)
			AND created_at <= ?
			ORDER by created_at DESC
			LIMIT ? OFFSET ?`)

	rows, err := db.db.QueryContext(ctx, query, coinpayments.StatusPending, coinpayments.StatusReceived, before, limit+1, offset)
	if err != nil {
		return stripecoinpayments.TransactionsPage{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var page stripecoinpayments.TransactionsPage

	for rows.Next() {
		var id, address string
		var userID uuid.UUID
		var amountGob, receivedGob []byte
		var amountNumeric, receivedNumeric *int64
		var amount, received monetary.Amount
		var status int
		var key string
		var createdAt time.Time

		err := rows.Scan(&id, &userID, &address, &amountGob, &amountNumeric, &receivedGob, &receivedNumeric, &status, &key, &createdAt)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, Error.Wrap(err)
		}

		// TODO: the currency here should be passed in to this function or stored
		//  in the database.
		currency := monetary.StorjToken

		if amountNumeric == nil {
			// 'amount' in this row hasn't yet been updated to a numeric value
			amount, err = monetaryAmountFromGobEncodedBigFloat(amountGob, currency)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.New("invalid gob encoding in amount_gob under transaction id %x: %v", id, err)
			}
		} else {
			amount = monetary.AmountFromBaseUnits(*amountNumeric, currency)
		}
		if receivedNumeric == nil {
			// 'received' in this row hasn't yet been updated to a numeric value
			received, err = monetaryAmountFromGobEncodedBigFloat(receivedGob, currency)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.New("invalid gob encoding in received_gob under transaction id %x: %v", id, err)
			}
		} else {
			received = monetary.AmountFromBaseUnits(*receivedNumeric, currency)
		}

		page.Transactions = append(page.Transactions,
			stripecoinpayments.Transaction{
				ID:        coinpayments.TransactionID(id),
				AccountID: userID,
				Address:   address,
				Amount:    amount,
				Received:  received,
				Status:    coinpayments.Status(status),
				Key:       key,
				CreatedAt: createdAt,
			},
		)
	}

	if err = rows.Err(); err != nil {
		return stripecoinpayments.TransactionsPage{}, err
	}

	if len(page.Transactions) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit)
		page.Transactions = page.Transactions[:len(page.Transactions)-1]
	}

	return page, nil
}

// ListUnapplied returns TransactionsPage with a pending or completed status, that should be applied to account balance.
func (db *coinPaymentsTransactions) ListUnapplied(ctx context.Context, offset int64, limit int, before time.Time) (_ stripecoinpayments.TransactionsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	query := db.db.Rebind(`SELECT
				txs.id,
				txs.user_id,
				txs.address,
				txs.amount_gob,
				txs.amount_numeric,
				txs.received_gob,
				txs.received_numeric,
				txs.status,
				txs.key,
				txs.created_at
			FROM coinpayments_transactions as txs
			INNER JOIN stripecoinpayments_apply_balance_intents as ints
			ON txs.id = ints.tx_id
			WHERE txs.status >= ?
			AND txs.created_at <= ?
			AND ints.state = ?
			ORDER by txs.created_at DESC
			LIMIT ? OFFSET ?`)

	rows, err := db.db.QueryContext(ctx, query, coinpayments.StatusReceived, before, applyBalanceIntentStateUnapplied, limit+1, offset)
	if err != nil {
		return stripecoinpayments.TransactionsPage{}, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var page stripecoinpayments.TransactionsPage

	for rows.Next() {
		var id, address string
		var userID uuid.UUID
		var amountGob, receivedGob []byte
		var amountNumeric, receivedNumeric *int64
		var status int
		var key string
		var createdAt time.Time

		err := rows.Scan(&id, &userID, &address, &amountGob, &amountNumeric, &receivedGob, &receivedNumeric, &status, &key, &createdAt)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, err
		}

		// TODO: the currency here should be passed in to this function or stored
		//  in the database.
		currency := monetary.StorjToken

		var amount, received monetary.Amount
		if amountNumeric == nil {
			// 'amount' in this row hasn't yet been updated to a numeric value
			amount, err = monetaryAmountFromGobEncodedBigFloat(amountGob, currency)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.New("invalid gob encoding in amount_gob under transaction id %x: %v", id, err)
			}
		} else {
			amount = monetary.AmountFromBaseUnits(*amountNumeric, currency)
		}
		if receivedNumeric == nil {
			// 'received' in this row hasn't yet been updated to a numeric value
			received, err = monetaryAmountFromGobEncodedBigFloat(receivedGob, currency)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.New("invalid gob encoding in received_gob under transaction id %x: %v", id, err)
			}
		} else {
			received = monetary.AmountFromBaseUnits(*receivedNumeric, currency)
		}

		page.Transactions = append(page.Transactions,
			stripecoinpayments.Transaction{
				ID:        coinpayments.TransactionID(id),
				AccountID: userID,
				Address:   address,
				Amount:    amount,
				Received:  received,
				Status:    coinpayments.Status(status),
				Key:       key,
				CreatedAt: createdAt,
			},
		)
	}

	if err = rows.Err(); err != nil {
		return stripecoinpayments.TransactionsPage{}, err
	}

	if len(page.Transactions) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit)
		page.Transactions = page.Transactions[:len(page.Transactions)-1]
	}

	return page, nil
}

// fromDBXCoinpaymentsTransaction converts *dbx.CoinpaymentsTransaction to *stripecoinpayments.Transaction.
func fromDBXCoinpaymentsTransaction(dbxCPTX *dbx.CoinpaymentsTransaction) (*stripecoinpayments.Transaction, error) {
	userID, err := uuid.FromBytes(dbxCPTX.UserId)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// TODO: the currency here should be passed in to this function or stored
	//  in the database.
	currency := monetary.StorjToken

	var amount, received monetary.Amount

	if dbxCPTX.AmountNumeric == nil {
		amount, err = monetaryAmountFromGobEncodedBigFloat(dbxCPTX.AmountGob, currency)
		if err != nil {
			return nil, Error.New("amount column: %v", err)
		}
	} else {
		amount = monetary.AmountFromBaseUnits(*dbxCPTX.AmountNumeric, currency)
	}
	if dbxCPTX.ReceivedNumeric == nil {
		received, err = monetaryAmountFromGobEncodedBigFloat(dbxCPTX.ReceivedGob, currency)
		if err != nil {
			return nil, Error.New("received column: %v", err)
		}
	} else {
		received = monetary.AmountFromBaseUnits(*dbxCPTX.ReceivedNumeric, currency)
	}

	return &stripecoinpayments.Transaction{
		ID:        coinpayments.TransactionID(dbxCPTX.Id),
		AccountID: userID,
		Address:   dbxCPTX.Address,
		Amount:    amount,
		Received:  received,
		Status:    coinpayments.Status(dbxCPTX.Status),
		Key:       dbxCPTX.Key,
		Timeout:   time.Second * time.Duration(dbxCPTX.Timeout),
		CreatedAt: dbxCPTX.CreatedAt,
	}, nil
}

func monetaryAmountFromGobEncodedBigFloat(encoded []byte, currency *monetary.Currency) (_ monetary.Amount, err error) {
	var bf big.Float
	if err := bf.GobDecode(encoded); err != nil {
		return monetary.Amount{}, Error.Wrap(err)
	}
	return monetary.AmountFromBigFloat(&bf, currency)
}
