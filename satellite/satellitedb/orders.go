// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/dbutil/sqliteutil"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/orders"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

const defaultIntervalSeconds = int(time.Hour / time.Second)

type ordersDB struct {
	db *dbx.DB
}

// CreateSerialInfo creates serial number entry in database
func (db *ordersDB) CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Create_SerialNumber(
		ctx,
		dbx.SerialNumber_SerialNumber(serialNumber.Bytes()),
		dbx.SerialNumber_BucketId(bucketID),
		dbx.SerialNumber_ExpiresAt(limitExpiration),
	)
	return err
}

// DeleteExpiredSerials deletes all expired serials in serial_number and used_serials table.
func (db *ordersDB) DeleteExpiredSerials(ctx context.Context, now time.Time) (_ int, err error) {
	defer mon.Task()(&ctx)(&err)
	count, err := db.db.Delete_SerialNumber_By_ExpiresAt_LessOrEqual(ctx, dbx.SerialNumber_ExpiresAt(now))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// UseSerialNumber creates serial number entry in database
func (db *ordersDB) UseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO used_serials (serial_number_id, storage_node_id)
		SELECT id, ? FROM serial_numbers WHERE serial_number = ?`,
	)
	_, err = db.db.ExecContext(ctx, statement, storageNodeID.Bytes(), serialNumber.Bytes())
	if err != nil {
		if pgutil.IsConstraintError(err) || sqliteutil.IsConstraintError(err) {
			return nil, orders.ErrUsingSerialNumber.New("serial number already used")
		}
		return nil, err
	}

	dbxSerialNumber, err := db.db.Find_SerialNumber_By_SerialNumber(
		ctx,
		dbx.SerialNumber_SerialNumber(serialNumber.Bytes()),
	)
	if err != nil {
		return nil, err
	}
	if dbxSerialNumber == nil {
		return nil, orders.ErrUsingSerialNumber.New("serial number not found")
	}
	return dbxSerialNumber.BucketId, nil
}

// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket
func (db *ordersDB) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET allocated = bucket_bandwidth_rollups.allocated + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart, defaultIntervalSeconds, action, 0, uint64(amount), 0, uint64(amount),
	)
	if err != nil {
		return err
	}

	return nil
}

// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
func (db *ordersDB) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET settled = bucket_bandwidth_rollups.settled + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart, defaultIntervalSeconds, action, 0, 0, uint64(amount), uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
func (db *ordersDB) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET inline = bucket_bandwidth_rollups.inline + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart, defaultIntervalSeconds, action, uint64(amount), 0, 0, uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// UpdateStoragenodeBandwidthAllocation updates 'allocated' bandwidth for given storage node
func (db *ordersDB) UpdateStoragenodeBandwidthAllocation(ctx context.Context, storageNodes []storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch t := db.db.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		statement := db.db.Rebind(
			`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, allocated, settled)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(storagenode_id, interval_start, action)
			DO UPDATE SET allocated = storagenode_bandwidth_rollups.allocated + excluded.allocated`,
		)
		for _, storageNode := range storageNodes {
			_, err = db.db.ExecContext(ctx, statement,
				storageNode.Bytes(), intervalStart, defaultIntervalSeconds, action, uint64(amount), 0,
			)
			if err != nil {
				return Error.Wrap(err)
			}
		}

	case *pq.Driver:
		// sort nodes to avoid update deadlock
		sort.Sort(storj.NodeIDList(storageNodes))

		_, err := db.db.ExecContext(ctx, `
			INSERT INTO storagenode_bandwidth_rollups
				(storagenode_id, interval_start, interval_seconds, action, allocated, settled)
			SELECT unnest($1::bytea[]), $2, $3, $4, $5, $6
			ON CONFLICT(storagenode_id, interval_start, action)
			DO UPDATE SET allocated = storagenode_bandwidth_rollups.allocated + excluded.allocated
		`, postgresNodeIDList(storageNodes), intervalStart, defaultIntervalSeconds, action, uint64(amount), 0)
		if err != nil {
			return Error.Wrap(err)
		}
	default:
		return Error.New("Unsupported database %t", t)
	}

	return nil
}

// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node for the given intervalStart time
func (db *ordersDB) UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		storageNode.Bytes(), intervalStart, defaultIntervalSeconds, action, 0, uint64(amount), uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// GetBucketBandwidth gets total bucket bandwidth from period of time
func (db *ordersDB) GetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE bucket_name = ? AND project_id = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(db.db.Rebind(query), bucketName, projectID[:], from, to).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
func (db *ordersDB) GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	query := `SELECT SUM(settled) FROM storagenode_bandwidth_rollups WHERE storagenode_id = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(db.db.Rebind(query), nodeID.Bytes(), from, to).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// UnuseSerialNumber removes pair serial number -> storage node id from database
func (db *ordersDB) UnuseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := `DELETE FROM used_serials WHERE storage_node_id = ? AND
				  serial_number_id IN (SELECT id FROM serial_numbers WHERE serial_number = ?)`
	_, err = db.db.ExecContext(ctx, db.db.Rebind(statement), storageNodeID.Bytes(), serialNumber.Bytes())
	return err
}

// ProcessOrders take a list of order requests and "settles" them in one transaction
func (db *ordersDB) ProcessOrders(ctx context.Context, requests []*orders.ProcessOrderRequest) (responses []*orders.ProcessOrderResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(requests) == 0 {
		return nil, err
	}

	tx, err := db.db.Begin()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()

	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	rejectedRequests := make(map[storj.SerialNumber]bool)
	reject := func(serialNumber storj.SerialNumber) {
		r := &orders.ProcessOrderResponse{
			SerialNumber: serialNumber,
			Status:       pb.SettlementResponse_REJECTED,
		}
		rejectedRequests[serialNumber] = true
		responses = append(responses, r)
	}

	// processes the insert to used serials table individually so we can handle
	// the case where the order has already been processed.  Duplicates and previously
	// processed orders are rejected
	for _, request := range requests {
		// avoid the PG error "current transaction is aborted, commands ignored until end of transaction block" if the below insert fails due any constraint.
		// see https://www.postgresql.org/message-id/13131805-BCBB-42DF-953B-27EE36AAF213%40yahoo.com
		_, err = tx.Exec("savepoint sp")
		if err != nil {
			return nil, err
		}

		insert := "INSERT INTO used_serials (serial_number_id, storage_node_id) SELECT id, ? FROM serial_numbers WHERE serial_number = ?"

		_, err = tx.Exec(db.db.Rebind(insert), request.OrderLimit.StorageNodeId.Bytes(), request.OrderLimit.SerialNumber.Bytes())
		if err != nil {
			if pgutil.IsConstraintError(err) || sqliteutil.IsConstraintError(err) {
				reject(request.OrderLimit.SerialNumber)
				// rollback to the savepoint before the insert failed
				_, err = tx.Exec("rollback to savepoint sp")
				if err != nil {
					return nil, Error.Wrap(err)
				}
			} else {
				return nil, Error.Wrap(err)
			}
		}
		_, err = tx.Exec("release savepoint sp")
		if err != nil {
			return nil, err
		}
	}

	// call to get all the bucket IDs
	query := db.buildGetBucketIdsQuery(len(requests))
	statement := db.db.Rebind(query)

	args := make([]interface{}, len(requests))
	for i, request := range requests {
		args[i] = request.OrderLimit.SerialNumber.Bytes()
	}

	rows, err := tx.Query(statement, args...)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	bucketMap := make(map[storj.SerialNumber][]byte)
	for rows.Next() {
		var serialNumber, bucketID []byte
		err := rows.Scan(&serialNumber, &bucketID)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		sn, err := storj.SerialNumberFromBytes(serialNumber)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		bucketMap[sn] = bucketID
	}

	// build all the bandwidth updates into one sql statement
	var updateRollupStatement string
	for _, request := range requests {
		_, rejected := rejectedRequests[request.OrderLimit.SerialNumber]
		if rejected {
			continue
		}
		bucketID, ok := bucketMap[request.OrderLimit.SerialNumber]
		if !ok {
			reject(request.OrderLimit.SerialNumber)
			continue
		}
		projectID, bucketName, err := orders.SplitBucketID(bucketID)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		stmt, err := db.buildUpdateBucketBandwidthRollupStatements(request.OrderLimit, request.Order, projectID[:], bucketName, intervalStart)
		if err != nil {
			return nil, err
		}
		updateRollupStatement += stmt

		stmt, err = db.buildUpdateStorageNodeBandwidthRollupStatements(request.OrderLimit, request.Order, intervalStart)
		if err != nil {
			return nil, err
		}
		updateRollupStatement += stmt
	}

	_, err = tx.Exec(updateRollupStatement)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	for _, request := range requests {
		_, rejected := rejectedRequests[request.OrderLimit.SerialNumber]
		if !rejected {
			r := &orders.ProcessOrderResponse{
				SerialNumber: request.OrderLimit.SerialNumber,
				Status:       pb.SettlementResponse_ACCEPTED,
			}

			responses = append(responses, r)
		}
	}
	return responses, nil
}

func (db *ordersDB) buildGetBucketIdsQuery(argCount int) string {
	args := make([]string, argCount)
	for i := 0; i < argCount; i++ {
		args[i] = "?"
	}
	return fmt.Sprintf("SELECT serial_number, bucket_id FROM serial_numbers WHERE serial_number IN (%s);\n", strings.Join(args, ","))
}

func (db *ordersDB) buildUpdateBucketBandwidthRollupStatements(orderLimit *pb.OrderLimit, order *pb.Order, projectID []byte, bucketName []byte, intervalStart time.Time) (string, error) {
	hexName, err := db.toHex(bucketName)
	if err != nil {
		return "", err
	}
	hexProjectID, err := db.toHex(projectID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (%s, %s, '%s', %d, %d, %d, %d, %d)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET settled = bucket_bandwidth_rollups.settled + %d;
`, hexName, hexProjectID, intervalStart.Format("2006-01-02 15:04:05+00:00"), defaultIntervalSeconds, orderLimit.Action, 0, 0, order.Amount, order.Amount), nil
}

func (db *ordersDB) buildUpdateStorageNodeBandwidthRollupStatements(orderLimit *pb.OrderLimit, order *pb.Order, intervalStart time.Time) (string, error) {
	hexNodeID, err := db.toHex(orderLimit.StorageNodeId.Bytes())
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, allocated, settled)
		VALUES (%s, '%s', %d, %d, %d, %d)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + %d;
`, hexNodeID, intervalStart.Format("2006-01-02 15:04:05+00:00"), defaultIntervalSeconds, orderLimit.Action, 0, order.Amount, order.Amount), nil
}

func (db *ordersDB) toHex(value []byte) (string, error) {
	hexValue := hex.EncodeToString(value)
	switch t := db.db.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		return fmt.Sprintf("X'%v'", hexValue), nil
	case *pq.Driver:
		return fmt.Sprintf("decode('%v', 'hex')", hexValue), nil
	default:
		return "", errs.New("Unsupported DB type %q", t)
	}
}
