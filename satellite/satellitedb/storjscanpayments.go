// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/currency"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/slices2"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

var _ storjscan.PaymentsDB = (*storjscanPayments)(nil)

// storjscanPayments implements storjscan.DB.
type storjscanPayments struct {
	db *satelliteDB
}

// InsertBatch inserts list of payments in a single transaction.
func (storjscanPayments *storjscanPayments) InsertBatch(ctx context.Context, payments []storjscan.CachedPayment) (err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		chainIDs      = make([]int64, 0, len(payments))
		blockHashes   = make([][]byte, 0, len(payments))
		blockNumbers  = make([]int64, 0, len(payments))
		transactions  = make([][]byte, 0, len(payments))
		logIndexes    = make([]int32, 0, len(payments))
		fromAddresses = make([][]byte, 0, len(payments))
		toAddresses   = make([][]byte, 0, len(payments))
		tokenValues   = make([]int64, 0, len(payments))
		usdValues     = make([]int64, 0, len(payments))
		statuses      = make([]string, 0, len(payments))
		timestamps    = make([]time.Time, 0, len(payments))

		createdAt = time.Now()
	)
	for i := range payments {
		payment := payments[i]
		chainIDs = append(chainIDs, payment.ChainID)
		blockHashes = append(blockHashes, payment.BlockHash[:])
		blockNumbers = append(blockNumbers, payment.BlockNumber)
		transactions = append(transactions, payment.Transaction[:])
		logIndexes = append(logIndexes, int32(payment.LogIndex))
		fromAddresses = append(fromAddresses, payment.From[:])
		toAddresses = append(toAddresses, payment.To[:])
		tokenValues = append(tokenValues, payment.TokenValue.BaseUnits())
		usdValues = append(usdValues, payment.USDValue.BaseUnits())
		statuses = append(statuses, string(payment.Status))
		timestamps = append(timestamps, payment.Timestamp)
	}

	var query string

	switch storjscanPayments.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		query = `INSERT INTO storjscan_payments(
				chain_id,
				block_hash,
				block_number,
				transaction,
				log_index,
				from_address,
				to_address,
				token_value,
				usd_value,
				status,
				block_timestamp,
				created_at
			) SELECT
				UNNEST($1::INT8[]),
				UNNEST($2::BYTEA[]),
				UNNEST($3::INT8[]),
				UNNEST($4::BYTEA[]),
				UNNEST($5::INT4[]),
				UNNEST($6::BYTEA[]),
				UNNEST($7::BYTEA[]),
				UNNEST($8::INT8[]),
				UNNEST($9::INT8[]),
				UNNEST($10::TEXT[]),
				UNNEST($11::TIMESTAMPTZ[]),
				$12
			`
		_, err = storjscanPayments.db.ExecContext(ctx, query,
			pgutil.Int8Array(chainIDs),
			pgutil.ByteaArray(blockHashes),
			pgutil.Int8Array(blockNumbers),
			pgutil.ByteaArray(transactions),
			pgutil.Int4Array(logIndexes),
			pgutil.ByteaArray(fromAddresses),
			pgutil.ByteaArray(toAddresses),
			pgutil.Int8Array(tokenValues),
			pgutil.Int8Array(usdValues),
			pgutil.TextArray(statuses),
			pgutil.TimestampTZArray(timestamps),
			createdAt)

	case dbutil.Spanner:
		type payment struct {
			ChainID        int64
			BlockHash      []byte
			BlockNumber    int64
			Transaction    []byte
			LogIndex       int64
			FromAddress    []byte
			ToAddress      []byte
			TokenValue     int64
			USDValue       int64
			Status         string
			BlockTimestamp time.Time
		}

		// TODO(spanner): optimize this by not constructing the intermediate slices.
		payments := make([]payment, len(chainIDs))
		for i := range payments {
			payments[i] = payment{
				ChainID:        chainIDs[i],
				BlockHash:      blockHashes[i],
				BlockNumber:    blockNumbers[i],
				Transaction:    transactions[i],
				LogIndex:       int64(logIndexes[i]),
				FromAddress:    fromAddresses[i],
				ToAddress:      toAddresses[i],
				TokenValue:     tokenValues[i],
				USDValue:       usdValues[i],
				Status:         statuses[i],
				BlockTimestamp: timestamps[i],
			}
		}

		_, err = storjscanPayments.db.ExecContext(ctx, `
			INSERT INTO
			storjscan_payments ( chain_id, block_hash, block_number, transaction, log_index,
				from_address, to_address, token_value, usd_value, status, block_timestamp, created_at)
			(SELECT ChainID, BlockHash, BlockNumber, Transaction, LogIndex,
				FromAddress, ToAddress, TokenValue, USDValue, Status, BlockTimestamp, ? FROM UNNEST(?))
		`, createdAt, payments)
	default:
		err = errors.New("database implementation not supported")
	}
	return err
}

// List returns list of storjscan payments order by chain ID block number and log index desc.
func (storjscanPayments *storjscanPayments) List(ctx context.Context) (_ []storjscan.CachedPayment, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxPmnts, err := storjscanPayments.db.All_StorjscanPayment_OrderBy_Asc_ChainId_Asc_BlockNumber_Asc_LogIndex(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var payments []storjscan.CachedPayment
	for _, dbxPmnt := range dbxPmnts {
		payments = append(payments, fromDBXPayment(dbxPmnt))
	}

	return payments, nil
}

// ListWallet returns list of storjscan payments order by Chain ID block number and log index desc.
func (storjscanPayments *storjscanPayments) ListWallet(ctx context.Context, wallet blockchain.Address, limit int, offset int64) ([]storjscan.CachedPayment, error) {
	dbxPmnts, err := storjscanPayments.db.Limited_StorjscanPayment_By_ToAddress_OrderBy_Desc_ChainId_Desc_BlockNumber_Desc_LogIndex(ctx,
		dbx.StorjscanPayment_ToAddress(wallet[:]),
		limit, offset)
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return []storjscan.CachedPayment{}, nil
		}
		return nil, Error.Wrap(err)
	}

	return slices2.Map(dbxPmnts, fromDBXPayment), nil
}

// LastBlocks returns the highest blocks known to DB per chain.
func (storjscanPayments *storjscanPayments) LastBlocks(ctx context.Context, status payments.PaymentStatus) (_ map[int64]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT DISTINCT chain_id FROM storjscan_payments where status = ?`
	rows, err := storjscanPayments.db.QueryContext(ctx, storjscanPayments.db.Rebind(query), string(status))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, Error.Wrap(rows.Close()))
	}()

	var latestBlocks = make(map[int64]int64)
	for rows.Next() {
		var chainID int64
		err = rows.Scan(&chainID)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		latestBlock, err := storjscanPayments.db.First_StorjscanPayment_BlockNumber_By_Status_And_ChainId_OrderBy_Desc_BlockNumber_Desc_LogIndex(
			ctx, dbx.StorjscanPayment_Status(string(status)),
			dbx.StorjscanPayment_ChainId(chainID))
		if err != nil {
			return nil, Error.Wrap(err)
		}
		if latestBlock == nil {
			return nil, Error.Wrap(sql.ErrNoRows)
		}
		latestBlocks[chainID] = latestBlock.BlockNumber
	}
	err = rows.Err()
	if err != nil {
		return nil, Error.Wrap(rows.Err())
	}
	if len(latestBlocks) == 0 {
		return nil, Error.Wrap(storjscan.ErrNoPayments)
	}
	return latestBlocks, nil
}

// DeletePending removes all pending transactions from the DB.
func (storjscanPayments storjscanPayments) DeletePending(ctx context.Context) error {
	_, err := storjscanPayments.db.Delete_StorjscanPayment_By_Status(ctx,
		dbx.StorjscanPayment_Status(payments.PaymentStatusPending))
	return err
}

func (storjscanPayments storjscanPayments) ListConfirmed(ctx context.Context, source string, chainID, blockNumber int64, logIndex int) (_ []storjscan.CachedPayment, err error) {
	defer mon.Task()(&ctx)(&err)

	var chainIDs []int64
	if chainID == 0 {
		// new source, search across all associated chainIDs
		chainIDs = billing.SourceChainIDs[source]
	} else {
		chainIDs = []int64{chainID}
	}

	var rows tagsql.Rows

	switch storjscanPayments.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		// TODO: use DBX here and optimize this query
		query := `SELECT chain_id, block_hash, block_number, transaction, log_index, from_address, to_address, token_value, usd_value, status, block_timestamp
				  FROM storjscan_payments WHERE chain_id = any($1::INT8[]) AND (storjscan_payments.block_number, storjscan_payments.log_index) > ($2, $3)
				  AND storjscan_payments.status = $4 ORDER BY storjscan_payments.block_number, storjscan_payments.log_index`
		rows, err = storjscanPayments.db.QueryContext(ctx, storjscanPayments.db.Rebind(query), pgutil.Int8Array(chainIDs), blockNumber, logIndex, payments.PaymentStatusConfirmed)
	case dbutil.Spanner:
		query := `SELECT
			chain_id,
			block_hash,
			block_number,
			transaction,
			log_index,
			from_address,
			to_address,
			token_value,
			usd_value,
			status,
			block_timestamp
		FROM
			storjscan_payments
		WHERE chain_id IN UNNEST (?)
			AND (block_number > ? OR (block_number = ? AND log_index > ?))
			AND status = ?
		ORDER BY
			block_number,
			log_index`
		rows, err = storjscanPayments.db.QueryContext(ctx, query, chainIDs, blockNumber, blockNumber, logIndex, payments.PaymentStatusConfirmed)
	default:
		return nil, Error.New("unsupported database: %v", storjscanPayments.db.impl)
	}
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var payments []storjscan.CachedPayment
	for rows.Next() {
		var payment dbx.StorjscanPayment
		err = rows.Scan(&payment.ChainId, &payment.BlockHash, &payment.BlockNumber, &payment.Transaction, &payment.LogIndex,
			&payment.FromAddress, &payment.ToAddress, &payment.TokenValue, &payment.UsdValue, &payment.Status, &payment.BlockTimestamp)
		if err != nil {
			return nil, err
		}
		payments = append(payments, fromDBXPayment(&payment))
	}
	return payments, rows.Err()
}

// fromDBXPayment converts dbx storjscan payment type to storjscan.CachedPayment.
func fromDBXPayment(dbxPmnt *dbx.StorjscanPayment) storjscan.CachedPayment {
	payment := storjscan.CachedPayment{
		ChainID:     dbxPmnt.ChainId,
		TokenValue:  currency.AmountFromBaseUnits(dbxPmnt.TokenValue, currency.StorjToken),
		USDValue:    currency.AmountFromBaseUnits(dbxPmnt.UsdValue, currency.USDollarsMicro),
		Status:      payments.PaymentStatus(dbxPmnt.Status),
		BlockNumber: dbxPmnt.BlockNumber,
		LogIndex:    dbxPmnt.LogIndex,
		Timestamp:   dbxPmnt.BlockTimestamp.UTC(),
	}
	copy(payment.From[:], dbxPmnt.FromAddress)
	copy(payment.To[:], dbxPmnt.ToAddress)
	copy(payment.BlockHash[:], dbxPmnt.BlockHash)
	copy(payment.Transaction[:], dbxPmnt.Transaction)
	return payment
}
