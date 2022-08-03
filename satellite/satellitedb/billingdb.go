// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgxutil"
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

func (db billingDB) Insert(ctx context.Context, billingTX billing.Transaction) (txID int64, err error) {
	defer mon.Task()(&ctx)(&err)
	balance, err := db.GetBalance(ctx, billingTX.UserID)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	if balance+billingTX.Amount.BaseUnits() < 0 {
		return 0, billing.ErrInsufficientFunds
	}

	var dbxTX *dbx.BillingTransaction

	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		rows, err := tx.Update_BillingBalance_By_UserId_And_Balance(ctx,
			dbx.BillingBalance_UserId(billingTX.UserID[:]),
			dbx.BillingBalance_Balance(balance),
			dbx.BillingBalance_Update_Fields{
				Balance: dbx.BillingBalance_Balance(balance + billingTX.Amount.BaseUnits()),
			})
		if err != nil {
			return Error.Wrap(err)
		}
		if rows == nil {
			// Try an insert here, in case the user never had a record in the table.
			// If the user already had a record, and the balance was not as expected,
			// the insert will fail anyways.
			err = tx.CreateNoReturn_BillingBalance(ctx,
				dbx.BillingBalance_UserId(billingTX.UserID[:]),
				dbx.BillingBalance_Balance(balance+billingTX.Amount.BaseUnits()))
			if err != nil {
				return Error.Wrap(err)
			}
		}

		dbxTX, err = tx.Create_BillingTransaction(ctx,
			dbx.BillingTransaction_UserId(billingTX.UserID[:]),
			dbx.BillingTransaction_Amount(billingTX.Amount.BaseUnits()),
			dbx.BillingTransaction_Currency(monetary.USDollars.Symbol()),
			dbx.BillingTransaction_Description(billingTX.Description),
			dbx.BillingTransaction_Source(billingTX.Source),
			dbx.BillingTransaction_Status(string(billingTX.Status)),
			dbx.BillingTransaction_Type(string(billingTX.Type)),
			dbx.BillingTransaction_Metadata(handleMetaDataZeroValue(billingTX.Metadata)),
			dbx.BillingTransaction_Timestamp(billingTX.Timestamp))
		return err
	})
	if err != nil {
		return 0, err
	}
	if dbxTX == nil {
		return 0, err
	}
	return dbxTX.Id, err
}

func (db billingDB) InsertBatch(ctx context.Context, billingTXs []billing.Transaction) (err error) {
	err = pgxutil.Conn(ctx, db.db, func(conn *pgx.Conn) error {
		var batch pgx.Batch
		for _, billingTX := range billingTXs {
			statement := db.db.Rebind(`INSERT INTO billing_transactions ( user_id, amount, currency, description, source, status, type, metadata, timestamp, created_at ) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW() )`)
			batch.Queue(statement, billingTX.UserID.Bytes(), billingTX.Amount.BaseUnits(), monetary.USDollars.Symbol(), billingTX.Description, billingTX.Source,
				billingTX.Status, billingTX.Type, handleMetaDataZeroValue(billingTX.Metadata), billingTX.Timestamp)
			if err != nil {
				db.db.log.Warn("invalid metadata skipped in query ", zap.String("statement query", statement))
			}
			statement = db.db.Rebind(`INSERT INTO billing_balances ( user_id, balance, last_updated ) VALUES ( $1, $2, NOW() ) ON CONFLICT (user_id) DO UPDATE SET balance = billing_balances.balance + $2 WHERE billing_balances.user_id = $1`)
			batch.Queue(statement, billingTX.UserID.Bytes(), billingTX.Amount.BaseUnits())
		}
		results := conn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, results.Close()) }()

		var errGroup errs.Group
		for i := 0; i < batch.Len(); i++ {
			_, err := results.Exec()
			errGroup.Add(err)
		}

		return errGroup.Err()
	})
	return err
}
func (db billingDB) UpdateStatus(ctx context.Context, txID int64, status billing.TransactionStatus) (err error) {
	defer mon.Task()(&ctx)(&err)
	return db.db.UpdateNoReturn_BillingTransaction_By_Id(ctx, dbx.BillingTransaction_Id(txID), dbx.BillingTransaction_Update_Fields{
		Status: dbx.BillingTransaction_Status(string(status)),
	})
}

func (db billingDB) UpdateMetadata(ctx context.Context, txID int64, newMetadata []byte) (err error) {

	dbxTX, err := db.db.Get_BillingTransaction_Metadata_By_Id(ctx, dbx.BillingTransaction_Id(txID))
	if err != nil {
		return Error.Wrap(err)
	}

	updatedMetadata, err := updateMetadata(dbxTX.Metadata, newMetadata)
	if err != nil {
		return Error.Wrap(err)
	}

	return db.db.UpdateNoReturn_BillingTransaction_By_Id(ctx, dbx.BillingTransaction_Id(txID), dbx.BillingTransaction_Update_Fields{
		Metadata: dbx.BillingTransaction_Metadata(updatedMetadata),
	})
}

func (db billingDB) LastTransaction(ctx context.Context, txSource string, txType billing.TransactionType) (_ time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	lastTransactionTime, err := db.db.First_BillingTransaction_Timestamp_By_Source_And_Type_OrderBy_Desc_Timestamp(
		ctx,
		dbx.BillingTransaction_Source(txSource),
		dbx.BillingTransaction_Type(string(txType)))
	if err != nil {
		return time.Time{}, Error.Wrap(err)
	}

	return lastTransactionTime.Timestamp, nil
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

func (db billingDB) GetBalance(ctx context.Context, userID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxBilling, err := db.db.Get_BillingBalance_Balance_By_UserId(ctx,
		dbx.BillingBalance_UserId(userID[:]))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, Error.Wrap(err)
	}

	return dbxBilling.Balance, nil
}

// fromDBXBillingTransaction converts *dbx.BillingTransaction to *billing.Transaction.
func fromDBXBillingTransaction(dbxTX *dbx.BillingTransaction) (*billing.Transaction, error) {
	userID, err := uuid.FromBytes(dbxTX.UserId)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &billing.Transaction{
		ID:          dbxTX.Id,
		UserID:      userID,
		Amount:      monetary.AmountFromBaseUnits(dbxTX.Amount, monetary.USDollars),
		Description: dbxTX.Description,
		Source:      dbxTX.Source,
		Status:      billing.TransactionStatus(dbxTX.Status),
		Type:        billing.TransactionType(dbxTX.Type),
		Metadata:    dbxTX.Metadata,
		Timestamp:   dbxTX.Timestamp,
		CreatedAt:   dbxTX.CreatedAt,
	}, nil
}

func updateMetadata(oldMetaData []byte, newMetaData []byte) ([]byte, error) {
	var updatedMetadata map[string]interface{}

	err := json.Unmarshal(oldMetaData, &updatedMetadata)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(handleMetaDataZeroValue(newMetaData), &updatedMetadata)
	if err != nil {
		return nil, err
	}

	return json.Marshal(updatedMetadata)
}

func handleMetaDataZeroValue(metaData []byte) []byte {
	if metaData != nil {
		return metaData
	}
	return []byte(`{}`)
}
