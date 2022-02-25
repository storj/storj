// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/private/dbutil/cockroachutil"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type transactionToMigrate struct {
	id              string
	amountGob       []byte
	amountNumeric   *int64
	receivedGob     []byte
	receivedNumeric *int64
	status          coinpayments.Status
}

// getTransactionsToMigrate fetches the data from up to limit rows in the
// coinpayments_transactions table which still have gob-encoded big.Float
// values in them. Querying starts at idRangeStart and proceeds in
// lexicographical order by the id column.
func (db *coinPaymentsTransactions) getTransactionsToMigrate(ctx context.Context, idRangeStart string, limit int) (toMigrate []transactionToMigrate, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.QueryContext(ctx, `
		SELECT id, amount_gob, amount_numeric, received_gob, received_numeric, status
		FROM coinpayments_transactions
		WHERE (amount_gob IS NOT NULL OR received_gob IS NOT NULL)
			AND id >= $1::text
		ORDER BY id
		LIMIT $2
	`, idRangeStart, limit)
	if err != nil {
		return nil, Error.New("could not issue transaction migration collection query: %v", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var xactsToMigrate []transactionToMigrate

	for rows.Next() {
		xact := transactionToMigrate{}
		err = rows.Scan(&xact.id, &xact.amountGob, &xact.amountNumeric, &xact.receivedGob, &xact.receivedNumeric, &xact.status)
		if err != nil {
			return nil, Error.New("could not read results of transaction migration collect query: %v", err)
		}
		xactsToMigrate = append(xactsToMigrate, xact)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.Wrap(err)
	}
	return xactsToMigrate, nil
}

// getTransactionsToMigrateWithRetry calls getTransactionsToMigrate in a loop
// until a result is found without any "retry needed" error being returned.
func (db *coinPaymentsTransactions) getTransactionsToMigrateWithRetry(ctx context.Context, idRangeStart string, limit int) (toMigrate []transactionToMigrate, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		toMigrate, err = db.getTransactionsToMigrate(ctx, idRangeStart, limit)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nil, err
		}
		break
	}
	return toMigrate, nil
}

// migrateGobFloatTransaction replaces gob-encoded big.Float values for one
// specific row in the coinpayments_transactions table with plain integers (in
// the base units of the currency for the column). Either the amount_gob or
// received_gob columns, or both, might be non-null, indicating the need for a
// replacement in the corresponding amount_numeric or received_numeric
// columns.
//
// This is implemented as a compare-and-swap, so that if any relevant changes
// occur on the target row since the time that we fetched it, this migration
// will not occur. Instead, wasMigrated will be returned as false, so that a
// future query can select the row for migration again if needed.
func (db *coinPaymentsTransactions) migrateGobFloatTransaction(ctx context.Context, transaction transactionToMigrate) (wasMigrated bool, err error) {
	defer mon.Task()(&ctx)(&err)

	currency := monetary.StorjToken
	args := []interface{}{
		transaction.id,
		transaction.status,
	}
	transactionIDArg := "$1"
	transactionStatusArg := "$2"

	var amountSetNewValue, amountGobOldValue, amountNumericOldValue string
	var receivedSetNewValue, receivedGobOldValue, receivedNumericOldValue string

	if transaction.amountGob == nil {
		amountGobOldValue = "IS NULL"
	} else {
		amount, err := monetaryAmountFromGobEncodedBigFloat(transaction.amountGob, currency)
		if err != nil {
			return false, Error.New("invalid gob-encoded amount in amount_gob column under transaction id %x: %w", transaction.id, err)
		}
		args = append(args, amount.BaseUnits())
		amountSetNewValue = fmt.Sprintf(", amount_numeric = $%d", len(args))
		args = append(args, transaction.amountGob)
		amountGobOldValue = fmt.Sprintf("= $%d::bytea", len(args))
	}

	if transaction.amountNumeric == nil {
		amountNumericOldValue = "IS NULL"
	} else {
		args = append(args, *transaction.amountNumeric)
		amountNumericOldValue = fmt.Sprintf("= $%d", len(args))
	}

	if transaction.receivedGob == nil {
		receivedGobOldValue = "IS NULL"
	} else {
		received, err := monetaryAmountFromGobEncodedBigFloat(transaction.receivedGob, currency)
		if err != nil {
			return false, Error.New("invalid gob-encoded amount in received_gob column under transaction id %x: %w", transaction.id, err)
		}
		args = append(args, received.BaseUnits())
		receivedSetNewValue = fmt.Sprintf(", received_numeric = $%d", len(args))
		args = append(args, transaction.receivedGob)
		receivedGobOldValue = fmt.Sprintf("= $%d::bytea", len(args))
	}

	if transaction.receivedNumeric == nil {
		receivedNumericOldValue = "IS NULL"
	} else {
		args = append(args, *transaction.receivedNumeric)
		receivedNumericOldValue = fmt.Sprintf("= $%d", len(args))
	}

	updateQuery := fmt.Sprintf(`
		UPDATE coinpayments_transactions
		SET amount_gob = NULL, received_gob = NULL%s%s
		WHERE id = %s
			AND status = %s
			AND amount_gob %s
			AND amount_numeric %s
			AND received_gob %s
			AND received_numeric %s
	`,
		amountSetNewValue, receivedSetNewValue,
		transactionIDArg, transactionStatusArg,
		amountGobOldValue, amountNumericOldValue,
		receivedGobOldValue, receivedNumericOldValue)

	result, err := db.db.ExecContext(ctx, updateQuery, args...)
	if err != nil {
		return false, Error.New("failed to update coinpayments_transactions row %x: %w", transaction.id, err)
	}
	// if zero rows were updated, then the row with this ID was changed by
	// something else before this migration got to it. we'll want to try
	// again with the next read query.
	numAffected, err := result.RowsAffected()
	if err != nil {
		return false, Error.New("could not get number of rows affected: %w", err)
	}
	return numAffected == 1, nil
}

// migrateGobFloatTransactionWithRetry calls migrateGobFloatTransaction in a
// loop until it succeeds without any "retry needed" error being returned.
func (db *coinPaymentsTransactions) migrateGobFloatTransactionWithRetry(ctx context.Context, transaction transactionToMigrate) (wasMigrated bool, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		wasMigrated, err = db.migrateGobFloatTransaction(ctx, transaction)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
		}
		return wasMigrated, err
	}
}

// MigrateGobFloatTransactionRecords is a strictly-temporary task that will,
// with time, eliminate gob-encoded big.Float records from the
// coinpayments_transactions table. It should be called repeatedly, passing back
// nextRangeStart for the next rangeStart parameter, until it encounters an
// error or returns nextRangeStart = "".
func (db *coinPaymentsTransactions) MigrateGobFloatTransactionRecords(ctx context.Context, rangeStart string, limit int) (migrated int, nextRangeStart string, err error) {
	defer mon.Task()(&ctx)(&err)

	xactsToMigrate, err := db.getTransactionsToMigrateWithRetry(ctx, rangeStart, limit)
	if err != nil {
		// some sort of internal error or programming error
		return 0, "", err
	}
	if len(xactsToMigrate) == 0 {
		// all rows are migrated!
		return 0, "", nil
	}
	for _, xact := range xactsToMigrate {
		wasMigrated, err := db.migrateGobFloatTransactionWithRetry(ctx, xact)
		if err != nil {
			// some sort of internal error or programming error
			return migrated, "", err
		}
		if wasMigrated {
			migrated++
		} else if nextRangeStart == "" {
			// Start here with the next call so that we can try again
			// (unless we are already going to start at an earlier point)
			nextRangeStart = xact.id
		}
	}

	// if nextRangeStart != "", then we need to redo some rows, and it's already
	// set appropriately.
	if nextRangeStart == "" {
		// if len(xactsToMigrate) < limit, then this is the last range and we've
		// completed the migration (leave nextRangeStart as "").
		if len(xactsToMigrate) == limit {
			// next time we can start after the last ID we just processed
			nextRangeStart = xactsToMigrate[len(xactsToMigrate)-1].id
		}
	}
	return migrated, nextRangeStart, nil
}

type conversionRateToMigrate struct {
	txID        string
	rateGob     []byte
	rateNumeric *float64
}

// getConversionRatesToMigrate fetches the data from up to limit rows in the
// stripecoinpayments_tx_conversion_rates table which still have gob-encoded
// big.Float values in them. Querying starts at txidRangeStart and proceeds in
// lexicographical order by the tx_id column.
func (db *coinPaymentsTransactions) getConversionRatesToMigrate(ctx context.Context, txidRangeStart string, limit int) (toMigrate []conversionRateToMigrate, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.QueryContext(ctx, `
		SELECT tx_id, rate_gob, rate_numeric
		FROM stripecoinpayments_tx_conversion_rates
		WHERE rate_gob IS NOT NULL
			AND tx_id >= $1::text
		ORDER BY tx_id
		LIMIT $2
	`, txidRangeStart, limit)
	if err != nil {
		return nil, Error.New("could not issue conversion rate migration collection query: %v", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var ratesToMigrate []conversionRateToMigrate

	for rows.Next() {
		rateRecord := conversionRateToMigrate{}
		err = rows.Scan(&rateRecord.txID, &rateRecord.rateGob, &rateRecord.rateNumeric)
		if err != nil {
			return nil, Error.New("could not read results of conversion rate migration collect query: %v", err)
		}
		ratesToMigrate = append(ratesToMigrate, rateRecord)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.Wrap(err)
	}
	return ratesToMigrate, nil
}

// getConversionRatesToMigrateWithRetry calls getConversionRatesToMigrate in a loop
// until a result is found without any "retry needed" error being returned.
func (db *coinPaymentsTransactions) getConversionRatesToMigrateWithRetry(ctx context.Context, idRangeStart string, limit int) (toMigrate []conversionRateToMigrate, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		toMigrate, err = db.getConversionRatesToMigrate(ctx, idRangeStart, limit)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nil, err
		}
		break
	}
	return toMigrate, nil
}

// migrateGobFloatConversionRate replaces gob-encoded big.Float values for one
// specific row in the stripecoinpayments_tx_conversion_rates table with DOUBLE
// PRECISION values.
//
// This is implemented as a compare-and-swap, so that if any relevant changes
// occur on the target row since the time that we fetched it, this migration
// will not occur. Instead, wasMigrated will be returned as false, so that a
// future query can select the row for migration again if needed.
func (db *coinPaymentsTransactions) migrateGobFloatConversionRate(ctx context.Context, rateRecord conversionRateToMigrate) (wasMigrated bool, err error) {
	defer mon.Task()(&ctx)(&err)

	args := []interface{}{rateRecord.txID}
	transactionIDArg := "$1::text"

	var rateSetNewValue, rateGobOldValue, rateNumericOldValue string
	if rateRecord.rateGob == nil {
		rateGobOldValue = "IS NULL"
	} else {
		var rateBigFloat big.Float
		if err = rateBigFloat.GobDecode(rateRecord.rateGob); err != nil {
			return false, Error.New("invalid gob-encoded rate in stripecoinpayments_tx_conversion_rates table tx_id = %x: %w", rateRecord.txID, err)
		}
		rateDecimal, err := monetary.DecimalFromBigFloat(&rateBigFloat)
		if err != nil {
			return false, Error.New("gob-encoded rate in stripecoinpayments_tx_conversion_rates table (tx_id = %x) cannot be converted to decimal.Decimal: %s: %w", rateRecord.txID, rateBigFloat.String(), err)
		}
		rateFloat64, exact := rateDecimal.Float64()
		if !exact {
			// see comment on exactness in (*coinPaymentsTransactions).LockRate()
			delta, _ := rateDecimal.Sub(decimal.NewFromFloat(rateFloat64)).Float64()
			mon.FloatVal("inexact-float64-exchange-rate-delta").Observe(delta)
		}
		args = append(args, rateFloat64)
		rateSetNewValue = fmt.Sprintf(", rate_numeric = $%d", len(args))
		args = append(args, rateRecord.rateGob)
		rateGobOldValue = fmt.Sprintf("= $%d::bytea", len(args))
	}
	if rateRecord.rateNumeric == nil {
		rateNumericOldValue = "IS NULL"
	} else {
		args = append(args, *rateRecord.rateNumeric)
		rateNumericOldValue = fmt.Sprintf("= $%d", len(args))
	}

	updateQuery := fmt.Sprintf(`
		UPDATE stripecoinpayments_tx_conversion_rates
		SET rate_gob = NULL%s
		WHERE tx_id = %s
			AND rate_gob %s
			AND rate_numeric %s
	`,
		rateSetNewValue,
		transactionIDArg,
		rateGobOldValue,
		rateNumericOldValue,
	)

	result, err := db.db.ExecContext(ctx, updateQuery, args...)
	if err != nil {
		return false, Error.New("failed to update stripecoinpayments_tx_conversion_rates row %x: %w", rateRecord.txID, err)
	}
	// if zero rows were updated, then the row with this ID was changed by
	// something else before this migration got to it. we'll want to try
	// again with the next read query.
	numAffected, err := result.RowsAffected()
	if err != nil {
		return false, Error.New("could not get number of rows affected: %w", err)
	}
	return numAffected == 1, nil
}

// migrateGobFloatConversionRateWithRetry calls migrateGobFloatConversionRate
// in a loop until it succeeds without any "retry needed" error being returned.
func (db *coinPaymentsTransactions) migrateGobFloatConversionRateWithRetry(ctx context.Context, rateRecord conversionRateToMigrate) (wasMigrated bool, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		wasMigrated, err = db.migrateGobFloatConversionRate(ctx, rateRecord)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
		}
		return wasMigrated, err
	}
}

// MigrateGobFloatConversionRateRecords is a strictly-temporary task that will,
// with time, eliminate gob-encoded big.Float records from the
// stripecoinpayments_tx_conversion_rates table. It should be called repeatedly,
// passing back nextRangeStart for the next rangeStart parameter, until it
// encounters an error or returns nextRangeStart = "".
func (db *coinPaymentsTransactions) MigrateGobFloatConversionRateRecords(ctx context.Context, rangeStart string, limit int) (migrated int, nextRangeStart string, err error) {
	defer mon.Task()(&ctx)(&err)

	ratesToMigrate, err := db.getConversionRatesToMigrateWithRetry(ctx, rangeStart, limit)
	if err != nil {
		// some sort of internal error or programming error
		return 0, "", err
	}
	if len(ratesToMigrate) == 0 {
		// all rows are migrated!
		return 0, "", nil
	}
	for _, rateRecord := range ratesToMigrate {
		wasMigrated, err := db.migrateGobFloatConversionRateWithRetry(ctx, rateRecord)
		if err != nil {
			// some sort of internal error or programming error
			return migrated, "", err
		}
		if wasMigrated {
			migrated++
		} else if nextRangeStart == "" {
			// Start here with the next call so that we can try again
			// (unless we are already going to start at an earlier point)
			nextRangeStart = rateRecord.txID
		}
	}
	// if nextRangeStart != "", then we need to redo some rows, and it's already
	// set appropriately.
	if nextRangeStart == "" {
		// if len(ratesToMigrate) < limit, then this is the last range and we've
		// completed the migration (leave nextRangeStart as "").
		if len(ratesToMigrate) == limit {
			// next time we can start after the last ID we just processed
			nextRangeStart = ratesToMigrate[len(ratesToMigrate)-1].txID
		}
	}
	return migrated, nextRangeStart, nil
}

// MonetaryAmountToGobEncodedBigFloat converts a monetary.Amount to a gob-encoded
// big.Float.
func MonetaryAmountToGobEncodedBigFloat(amount monetary.Amount) ([]byte, error) {
	asString := amount.AsDecimal().String()
	asBigFloat, ok := big.NewFloat(0).SetString(asString)
	if !ok {
		return nil, Error.New("failed to assign %q to a big.Float", asString)
	}
	gobEncoded, err := asBigFloat.GobEncode()
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return gobEncoded, nil
}

func (db *coinPaymentsTransactions) ForTestingOnlyInsertGobTransaction(ctx context.Context, tx stripecoinpayments.Transaction) (createdAt time.Time, err error) {
	amountGob, err := MonetaryAmountToGobEncodedBigFloat(tx.Amount)
	if err != nil {
		return time.Time{}, err
	}
	receivedGob, err := MonetaryAmountToGobEncodedBigFloat(tx.Received)
	if err != nil {
		return time.Time{}, err
	}
	record, err := db.db.Create_CoinpaymentsTransaction(ctx,
		dbx.CoinpaymentsTransaction_Id(tx.ID.String()),
		dbx.CoinpaymentsTransaction_UserId(tx.AccountID[:]),
		dbx.CoinpaymentsTransaction_Address(tx.Address),
		dbx.CoinpaymentsTransaction_Status(tx.Status.Int()),
		dbx.CoinpaymentsTransaction_Key(tx.Key),
		dbx.CoinpaymentsTransaction_Timeout(int(tx.Timeout.Seconds())),
		dbx.CoinpaymentsTransaction_Create_Fields{
			AmountGob:   dbx.CoinpaymentsTransaction_AmountGob(amountGob),
			ReceivedGob: dbx.CoinpaymentsTransaction_ReceivedGob(receivedGob),
		})
	return record.CreatedAt, Error.Wrap(err)
}

func (db *coinPaymentsTransactions) ForTestingOnlyInsertGobConversionRate(ctx context.Context, txID coinpayments.TransactionID, rate decimal.Decimal) error {
	gobEncoded, err := rate.BigFloat().GobEncode()
	if err != nil {
		return Error.Wrap(err)
	}
	_, err = db.db.Create_StripecoinpaymentsTxConversionRate(ctx,
		dbx.StripecoinpaymentsTxConversionRate_TxId(txID.String()),
		dbx.StripecoinpaymentsTxConversionRate_Create_Fields{
			RateGob: dbx.StripecoinpaymentsTxConversionRate_RateGob(gobEncoded),
		})
	return Error.Wrap(err)
}

func (db *coinPaymentsTransactions) ForTestingOnlyGetDBHandle() *dbx.DB {
	return db.db.DB
}
