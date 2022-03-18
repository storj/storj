// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGobFloatMigrationCompareAndSwapBehavior(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		transactionsDB := db.StripeCoinPayments().Transactions()
		testTransactions, ok := transactionsDB.(interface {
			ForTestingOnlyInsertGobTransaction(ctx context.Context, tx stripecoinpayments.Transaction) (time.Time, error)
			ForTestingOnlyGetDBHandle() *dbx.DB
		})
		require.Truef(t, ok, "db object of type %T s not a *coinPaymentsTransactions", transactionsDB)

		// make some random records, insert in db
		const numRecords = 100
		asInserted := make([]stripecoinpayments.Transaction, 0, numRecords)
		for x := 0; x < numRecords; x++ {
			tx := stripecoinpayments.Transaction{
				ID:        coinpayments.TransactionID(fmt.Sprintf("transaction%05d", x)),
				Status:    coinpayments.Status(x % 2), // 0 (pending) or 1 (received)
				AccountID: testrand.UUID(),
				Amount:    monetary.AmountFromBaseUnits(testrand.Int63n(1e15), monetary.StorjToken),
				Received:  monetary.AmountFromBaseUnits(testrand.Int63n(1e15), monetary.StorjToken),
				Address:   fmt.Sprintf("%x", testrand.Bytes(20)),
				Key:       fmt.Sprintf("%x", testrand.Bytes(20)),
			}
			createTime, err := testTransactions.ForTestingOnlyInsertGobTransaction(ctx, tx)
			require.NoError(t, err)
			tx.CreatedAt = createTime
			asInserted = append(asInserted, tx)
		}

		// In multiple goroutines, try to change one particular record
		// in the db as fast as we can, while we are trying to migrate
		// that record at the same time. This should (at least
		// sometimes) cause the migration to be retried, because the
		// underlying value changed.

		var (
			amountStoredValue = asInserted[0].Amount
			valueMutex        sync.Mutex
			testDoneYet       = false
			testDoneYetMutex  sync.Mutex

			group    errgroup.Group
			dbHandle = testTransactions.ForTestingOnlyGetDBHandle()
		)

		group.Go(func() error {
			for {
				testDoneYetMutex.Lock()
				areWeDone := testDoneYet
				testDoneYetMutex.Unlock()
				if areWeDone {
					break
				}

				newAmount := monetary.AmountFromBaseUnits(testrand.Int63n(1e15), monetary.StorjToken)
				newAmountGob, err := satellitedb.MonetaryAmountToGobEncodedBigFloat(newAmount)
				if err != nil {
					return err
				}
				result, err := dbHandle.ExecContext(ctx, `
					UPDATE coinpayments_transactions
					SET amount_gob = $1
					WHERE id = $2 AND amount_gob IS NOT NULL
				`, newAmountGob, asInserted[0].ID)
				if err != nil {
					return satellitedb.Error.New("could not set amount_gob: %w", err)
				}
				rowsAffected, err := result.RowsAffected()
				if err != nil {
					return satellitedb.Error.New("could not get rows affected: %w", err)
				}
				if rowsAffected < 1 {
					// migration must have happened!
					break
				}
				valueMutex.Lock()
				amountStoredValue = newAmount
				valueMutex.Unlock()
			}
			return nil
		})

		totalMigrated := 0
		for {
			numMigrated, nextRangeStart, err := transactionsDB.MigrateGobFloatTransactionRecords(ctx, "", numRecords+1)
			totalMigrated += numMigrated
			require.NoError(t, err)
			if nextRangeStart == "" {
				break
			}
		}
		assert.Equal(t, numRecords, totalMigrated)

		testDoneYetMutex.Lock()
		testDoneYet = true
		testDoneYetMutex.Unlock()

		err := group.Wait()
		require.NoError(t, err)

		// the final value as changed by the changer goroutine
		valueMutex.Lock()
		finalValue := amountStoredValue
		valueMutex.Unlock()

		// fetch the numeric value (as migrated) from the db
		row := dbHandle.QueryRowContext(ctx, `
			SELECT amount_gob, amount_numeric
			FROM coinpayments_transactions
			WHERE id = $1
		`, asInserted[0].ID)

		var amountGob []byte
		var amountNumeric int64
		err = row.Scan(&amountGob, &amountNumeric)
		require.NoError(t, err)
		assert.Nil(t, amountGob)

		amountFromDB := monetary.AmountFromBaseUnits(amountNumeric, monetary.StorjToken)
		assert.Truef(t, finalValue.Equal(amountFromDB), "%s != %s", finalValue, amountFromDB)
	})
}
