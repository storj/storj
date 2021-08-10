// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"math/big"
	"time"

	pgxerrcode "github.com/jackc/pgerrcode"
	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil/pgerrcode"
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

	amount, err := tx.Amount.AsBigFloat().GobEncode()
	if err != nil {
		return time.Time{}, errs.Wrap(err)
	}
	received, err := tx.Received.AsBigFloat().GobEncode()
	if err != nil {
		return time.Time{}, errs.Wrap(err)
	}

	dbxCPTX, err := db.db.Create_CoinpaymentsTransaction(ctx,
		dbx.CoinpaymentsTransaction_Id(tx.ID.String()),
		dbx.CoinpaymentsTransaction_UserId(tx.AccountID[:]),
		dbx.CoinpaymentsTransaction_Address(tx.Address),
		dbx.CoinpaymentsTransaction_Amount(amount),
		dbx.CoinpaymentsTransaction_Received(received),
		dbx.CoinpaymentsTransaction_Status(tx.Status.Int()),
		dbx.CoinpaymentsTransaction_Key(tx.Key),
		dbx.CoinpaymentsTransaction_Timeout(int(tx.Timeout.Seconds())),
	)
	if err != nil {
		if errCode := pgerrcode.FromError(err); errCode == pgxerrcode.UndefinedColumn {
			// TEMPORARY: fall back to expected new schema to facilitate transition
			return db.insertTransitionShim(ctx, tx)
		}
		return time.Time{}, err
	}
	return dbxCPTX.CreatedAt, nil
}

// insertTransitionShim inserts new coinpayments transaction into DB.
//
// It is to be used only during the transition from gob-encoded 'amount' and
// 'received' columns to 'amount_numeric'/'received_numeric'.
//
// When the transition is complete, this method will go away and Insert()
// will operate only on the _numeric columns.
func (db *coinPaymentsTransactions) insertTransitionShim(ctx context.Context, tx stripecoinpayments.Transaction) (createTime time.Time, err error) {
	row := db.db.DB.QueryRowContext(ctx, db.db.Rebind(`
		INSERT INTO coinpayments_transactions (
			id, user_id, address, amount_numeric, received_numeric, status, key, timeout, created_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, now()
		) RETURNING created_at;
	`), tx.ID.String(), tx.AccountID[:], tx.Address, tx.Amount.BaseUnits(), tx.Received.BaseUnits(), tx.Status.Int(), tx.Key, int(tx.Timeout.Seconds()))
	if err := row.Scan(&createTime); err != nil {
		return time.Time{}, Error.Wrap(err)
	}
	return createTime, nil
}

// Update updates status and received for set of transactions.
func (db *coinPaymentsTransactions) Update(ctx context.Context, updates []stripecoinpayments.TransactionUpdate, applies coinpayments.TransactionIDList) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(updates) == 0 {
		return nil
	}

	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, update := range updates {
			receivedGob, err := update.Received.AsBigFloat().GobEncode()
			if err != nil {
				return errs.Wrap(err)
			}

			_, err = tx.Update_CoinpaymentsTransaction_By_Id(ctx,
				dbx.CoinpaymentsTransaction_Id(update.TransactionID.String()),
				dbx.CoinpaymentsTransaction_Update_Fields{
					Received: dbx.CoinpaymentsTransaction_Received(receivedGob),
					Status:   dbx.CoinpaymentsTransaction_Status(update.Status.Int()),
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

	if err != nil {
		if errCode := pgerrcode.FromError(err); errCode == pgxerrcode.UndefinedColumn {
			// TEMPORARY: fall back to expected new schema to facilitate transition
			return db.updateTransitionShim(ctx, updates, applies)
		}
	}
	return err
}

// updateTransitionShim updates status and received for set of transactions.
//
// It is to be used only during the transition from gob-encoded 'amount' and
// 'received' columns to 'amount_numeric'/'received_numeric'. During the
// transition, the gob-encoded columns will still exist but under a different
// name ('amount_gob'/'received_gob'). If the _numeric column value for a given
// row is non-null, it takes precedence over the corresponding _gob column.
//
// When the transition is complete, this method will go away and
// Update() will operate only on the _numeric columns.
func (db *coinPaymentsTransactions) updateTransitionShim(ctx context.Context, updates []stripecoinpayments.TransactionUpdate, applies coinpayments.TransactionIDList) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(updates) == 0 {
		return nil
	}

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, update := range updates {
			query := db.db.Rebind(`
				UPDATE coinpayments_transactions
				SET
					received_gob = NULL,
					received_numeric = ?,
					status = ?
				WHERE id = ?
			`)
			_, err := tx.Tx.ExecContext(ctx, query, update.Received.BaseUnits(), update.Status.Int(), update.TransactionID.String())
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

	buff, err := rate.BigFloat().GobEncode()
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = db.db.Create_StripecoinpaymentsTxConversionRate(ctx,
		dbx.StripecoinpaymentsTxConversionRate_TxId(id.String()),
		dbx.StripecoinpaymentsTxConversionRate_Rate(buff))

	if err != nil {
		if errCode := pgerrcode.FromError(err); errCode == pgxerrcode.UndefinedColumn {
			// TEMPORARY: fall back to expected new schema to facilitate transition
			return db.lockRateTransitionShim(ctx, id, rate)
		}
	}
	return Error.Wrap(err)
}

// lockRateTransitionShim locks conversion rate for transaction.
//
// It is to be used only during the transition from the gob-encoded 'rate'
// column to 'rate_numeric'.
//
// When the transition is complete, this method will go away and
// LockRate() will operate only on the _numeric column.
func (db *coinPaymentsTransactions) lockRateTransitionShim(ctx context.Context, id coinpayments.TransactionID, rate decimal.Decimal) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().UTC()
	query := db.db.Rebind(`
		INSERT INTO stripecoinpayments_tx_conversion_rates ( tx_id, rate_numeric, created_at ) VALUES ( ?, ?, ? )
	`)

	_, err = db.db.DB.ExecContext(ctx, query, id.String(), rate.String(), now)
	return Error.Wrap(err)
}

// GetLockedRate returns locked conversion rate for transaction or error if non exists.
func (db *coinPaymentsTransactions) GetLockedRate(ctx context.Context, id coinpayments.TransactionID) (_ decimal.Decimal, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxRate, err := db.db.Get_StripecoinpaymentsTxConversionRate_By_TxId(ctx,
		dbx.StripecoinpaymentsTxConversionRate_TxId(id.String()),
	)
	if err != nil {
		if errCode := pgerrcode.FromError(err); errCode == pgxerrcode.UndefinedColumn {
			// TEMPORARY: fall back to expected new schema to facilitate transition
			return db.getLockedRateTransitionShim(ctx, id)
		}
		return decimal.Decimal{}, err
	}

	var rateF big.Float
	if err = rateF.GobDecode(dbxRate.Rate); err != nil {
		return decimal.Decimal{}, errs.Wrap(err)
	}
	rate, err := monetary.DecimalFromBigFloat(&rateF)
	if err != nil {
		return decimal.Decimal{}, errs.Wrap(err)
	}

	return rate, nil
}

// getLockedRateTransitionShim returns locked conversion rate for transaction
// or error if none exists.
//
// It is to be used only during the transition from the gob-encoded 'rate'
// column to 'rate_numeric'. During the transition, the gob-encoded column will
// still exist but under a different name ('rate_gob'). If rate_numeric for a
// given row is non-null, it takes precedence over rate_gob.
//
// When the transition is complete, this method will go away and
// GetLockedRate() will operate only on rate_numeric.
func (db *coinPaymentsTransactions) getLockedRateTransitionShim(ctx context.Context, id coinpayments.TransactionID) (_ decimal.Decimal, err error) {
	defer mon.Task()(&ctx)(&err)

	var rateGob []byte
	var rateNumeric *string
	query := db.db.Rebind(`
		SELECT rate_gob, rate_numeric
		FROM stripecoinpayments_tx_conversion_rates
		WHERE tx_id = ?
	`)
	row := db.db.DB.QueryRowContext(ctx, query, id.String())
	err = row.Scan(&rateGob, &rateNumeric)
	if err != nil {
		return decimal.Decimal{}, Error.Wrap(err)
	}

	if rateNumeric == nil {
		// This row does not have a numeric rate value yet
		var rateF big.Float
		if err = rateF.GobDecode(rateGob); err != nil {
			return decimal.Decimal{}, Error.Wrap(err)
		}
		rate, err := monetary.DecimalFromBigFloat(&rateF)
		return rate, Error.Wrap(err)
	}
	rate, err := decimal.NewFromString(*rateNumeric)
	return rate, Error.Wrap(err)
}

// ListAccount returns all transaction for specific user.
func (db *coinPaymentsTransactions) ListAccount(ctx context.Context, userID uuid.UUID) (_ []stripecoinpayments.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTXs, err := db.db.All_CoinpaymentsTransaction_By_UserId_OrderBy_Desc_CreatedAt(ctx,
		dbx.CoinpaymentsTransaction_UserId(userID[:]),
	)
	if err != nil {
		if errCode := pgerrcode.FromError(err); errCode == pgxerrcode.UndefinedColumn {
			// TEMPORARY: fall back to expected new schema to facilitate transition
			return db.listAccountTransitionShim(ctx, userID)
		}
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

// listAccountTransitionShim returns all transaction for specific user.
//
// It is to be used only during the transition from gob-encoded 'amount' and
// 'received' columns to 'amount_numeric'/'received_numeric'. During the
// transition, the gob-encoded columns will still exist but under a different
// name ('amount_gob'/'received_gob'). If the _numeric column value for a given
// row is non-null, it takes precedence over the corresponding _gob column.
//
// When the transition is complete, this method will go away and ListAccount()
// will operate only on the _numeric columns.
func (db *coinPaymentsTransactions) listAccountTransitionShim(ctx context.Context, userID uuid.UUID) (_ []stripecoinpayments.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)

	query := db.db.Rebind(`
		SELECT
			id,
			user_id,
			address,
			amount_gob,
			amount_numeric,
			received_gob,
			received_numeric,
			status,
			key,
			timeout,
			created_at
		FROM coinpayments_transactions
		WHERE user_id = ?
		ORDER BY created_at DESC
	`)
	rows, err := db.db.DB.QueryContext(ctx, query, userID[:])
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var txs []stripecoinpayments.Transaction
	for rows.Next() {
		var tx stripecoinpayments.Transaction
		var amountGob, receivedGob []byte
		var amountNumeric, receivedNumeric *int64
		var timeoutSeconds int
		err := rows.Scan(&tx.ID, &tx.AccountID, &tx.Address, &amountGob, &amountNumeric, &receivedGob, &receivedNumeric, &tx.Status, &tx.Key, &timeoutSeconds, &tx.CreatedAt)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		tx.Timeout = time.Second * time.Duration(timeoutSeconds)

		if amountNumeric == nil {
			tx.Amount, err = monetaryAmountFromGobEncodedBigFloat(amountGob, monetary.StorjToken)
			if err != nil {
				return nil, Error.New("amount column: %v", err)
			}
		} else {
			tx.Amount = monetary.AmountFromBaseUnits(*amountNumeric, monetary.StorjToken)
		}
		if receivedNumeric == nil {
			tx.Received, err = monetaryAmountFromGobEncodedBigFloat(receivedGob, monetary.StorjToken)
			if err != nil {
				return nil, Error.New("received column: %v", err)
			}
		} else {
			tx.Received = monetary.AmountFromBaseUnits(*receivedNumeric, monetary.StorjToken)
		}
		txs = append(txs, tx)
	}

	if err = rows.Err(); err != nil {
		return nil, err
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
				amount,
				received,
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
		if errCode := pgerrcode.FromError(err); errCode == pgxerrcode.UndefinedColumn {
			// TEMPORARY: fall back to expected new schema to facilitate transition
			return db.listPendingTransitionShim(ctx, offset, limit, before)
		}
		return stripecoinpayments.TransactionsPage{}, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var page stripecoinpayments.TransactionsPage

	for rows.Next() {
		var id, address string
		var userID uuid.UUID
		var amountB, receivedB []byte
		var status int
		var key string
		var createdAt time.Time

		err := rows.Scan(&id, &userID, &address, &amountB, &receivedB, &status, &key, &createdAt)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, err
		}

		// TODO: the currency here should be passed in to this function or stored
		//  in the database.
		currency := monetary.StorjToken

		amount, err := monetaryAmountFromGobEncodedBigFloat(amountB, currency)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, err
		}
		received, err := monetaryAmountFromGobEncodedBigFloat(receivedB, currency)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, err
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

// listPendingTransitionShim returns paginated list of pending transactions.
//
// It is to be used only during the transition from gob-encoded 'amount' and
// 'received' columns to 'amount_numeric'/'received_numeric'. During the
// transition, the gob-encoded columns will still exist but under a different
// name ('amount_gob'/'received_gob'). If the _numeric column value for a given
// row is non-null, it takes precedence over the corresponding _gob column.
//
// When the transition is complete, this method will go away and ListPending()
// will operate only on the _numeric columns.
func (db *coinPaymentsTransactions) listPendingTransitionShim(ctx context.Context, offset int64, limit int, before time.Time) (_ stripecoinpayments.TransactionsPage, err error) {
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
			return stripecoinpayments.TransactionsPage{}, err
		}

		if amountNumeric == nil {
			// 'amount' in this row hasn't yet been updated to a numeric value
			amount, err = monetaryAmountFromGobEncodedBigFloat(amountGob, monetary.StorjToken)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.Wrap(err)
			}
		} else {
			amount = monetary.AmountFromBaseUnits(*amountNumeric, monetary.StorjToken)
		}
		if receivedNumeric == nil {
			// 'received' in this row hasn't yet been updated to a numeric value
			received, err = monetaryAmountFromGobEncodedBigFloat(receivedGob, monetary.StorjToken)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.Wrap(err)
			}
		} else {
			received = monetary.AmountFromBaseUnits(*receivedNumeric, monetary.StorjToken)
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
				txs.amount,
				txs.received,
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
		if errCode := pgerrcode.FromError(err); errCode == pgxerrcode.UndefinedColumn {
			return db.listUnappliedTransitionShim(ctx, offset, limit, before)
		}
		return stripecoinpayments.TransactionsPage{}, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var page stripecoinpayments.TransactionsPage

	for rows.Next() {
		var id, address string
		var userID uuid.UUID
		var amountB, receivedB []byte
		var status int
		var key string
		var createdAt time.Time

		err := rows.Scan(&id, &userID, &address, &amountB, &receivedB, &status, &key, &createdAt)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, err
		}

		// TODO: the currency here should be passed in to this function or stored
		//  in the database.
		currency := monetary.StorjToken

		amount, err := monetaryAmountFromGobEncodedBigFloat(amountB, currency)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, Error.New("amount column: %v", err)
		}
		received, err := monetaryAmountFromGobEncodedBigFloat(receivedB, currency)
		if err != nil {
			return stripecoinpayments.TransactionsPage{}, Error.New("received column: %v", err)
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

// listUnappliedTransitionShim returns TransactionsPage with a pending or
// completed status, that should be applied to account balance.
//
// It is to be used only during the transition from gob-encoded 'amount' and
// 'received' columns to 'amount_numeric'/'received_numeric'. During the
// transition, the gob-encoded columns will still exist but under a different
// name ('amount_gob'/'received_gob'). If the _numeric column value for a given
// row is non-null, it takes precedence over the corresponding _gob column.
//
// When the transition is complete, this method will go away and
// ListUnapplied() will operate only on the _numeric columns.
func (db *coinPaymentsTransactions) listUnappliedTransitionShim(ctx context.Context, offset int64, limit int, before time.Time) (_ stripecoinpayments.TransactionsPage, err error) {
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

		var amount, received monetary.Amount
		if amountNumeric == nil {
			// 'amount' in this row hasn't yet been updated to a numeric value
			amount, err = monetaryAmountFromGobEncodedBigFloat(amountGob, monetary.StorjToken)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.Wrap(err)
			}
		} else {
			amount = monetary.AmountFromBaseUnits(*amountNumeric, monetary.StorjToken)
		}
		if receivedNumeric == nil {
			// 'received' in this row hasn't yet been updated to a numeric value
			received, err = monetaryAmountFromGobEncodedBigFloat(receivedGob, monetary.StorjToken)
			if err != nil {
				return stripecoinpayments.TransactionsPage{}, Error.Wrap(err)
			}
		} else {
			received = monetary.AmountFromBaseUnits(*receivedNumeric, monetary.StorjToken)
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

	amount, err := monetaryAmountFromGobEncodedBigFloat(dbxCPTX.Amount, currency)
	if err != nil {
		return nil, Error.New("amount column: %v", err)
	}
	received, err := monetaryAmountFromGobEncodedBigFloat(dbxCPTX.Received, currency)
	if err != nil {
		return nil, Error.New("received column: %v", err)
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

// DebugPerformBigFloatTransition performs the schema changes expected as part
// of Step 2 of the transition away from gob-encoded big.Float columns in the
// database.
//
// This is for testing purposes only, to ensure that no data is lost and that
// code still works after the transition.
func (db *coinPaymentsTransactions) DebugPerformBigFloatTransition(ctx context.Context) error {
	_, err := db.db.DB.ExecContext(ctx, `
		ALTER TABLE coinpayments_transactions ALTER COLUMN amount DROP NOT NULL;
		ALTER TABLE coinpayments_transactions ALTER COLUMN received DROP NOT NULL;
		ALTER TABLE coinpayments_transactions RENAME COLUMN amount TO amount_gob;
		ALTER TABLE coinpayments_transactions RENAME COLUMN received TO received_gob;
		ALTER TABLE coinpayments_transactions ADD COLUMN amount_numeric INT8;
		ALTER TABLE coinpayments_transactions ADD COLUMN received_numeric INT8;
		ALTER TABLE stripecoinpayments_tx_conversion_rates ALTER COLUMN rate DROP NOT NULL;
		ALTER TABLE stripecoinpayments_tx_conversion_rates RENAME COLUMN rate TO rate_gob;
		ALTER TABLE stripecoinpayments_tx_conversion_rates ADD COLUMN rate_numeric NUMERIC(20, 8);
	`)
	return Error.Wrap(err)
}
