// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

func (db *ordersDB) ProcessOrders(ctx context.Context, requests []*orders.ProcessOrderRequest) (responses []*pb.SettlementResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(requests) == 0 {
		return []*pb.SettlementResponse{}, err
	}

	tx, err := db.db.Begin()
	if err != nil {
		return []*pb.SettlementResponse{}, errs.Wrap(err)
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
		r := &pb.SettlementResponse{
			SerialNumber: serialNumber,
			Status:       pb.SettlementResponse_REJECTED,
		}
		rejectedRequests[serialNumber] = true
		responses = append(responses, r)
	}

	for _, request := range requests {
		_, err := tx.Exec(db.buildInsertUseSerialStatement(request.OrderLimit))
		if err != nil {
			if pgutil.IsConstraintError(err) || sqliteutil.IsConstraintError(err) {
				reject(request.OrderLimit.SerialNumber)
			} else if err != nil {
				return []*pb.SettlementResponse{}, Error.Wrap(err)
			}
		}
	}

	query := db.buildGetBucketIdsQuery(len(requests))
	statement := db.db.Rebind(query)

	args := make([]interface{}, 0, len(requests))
	for _, request := range requests {
		args = append(args, request.OrderLimit.SerialNumber.Bytes())
	}

	rows, err := tx.Query(statement, args...)
	if err != nil {
		return []*pb.SettlementResponse{}, errs.Wrap(err)
	}
	bucketMap := make(map[storj.SerialNumber][]byte)
	for rows.Next() {
		var serialNumber, bucketID []byte
		err := rows.Scan(&serialNumber, &bucketID)
		if err != nil {
			return []*pb.SettlementResponse{}, errs.Wrap(err)
		}
		sn, err := storj.SerialNumberFromBytes(serialNumber)
		if err != nil {
			return []*pb.SettlementResponse{}, errs.Wrap(err)
		}
		bucketMap[sn] = bucketID
	}

	var updateRollupStatement string
	for _, request := range requests {
		_, rejected := rejectedRequests[request.OrderLimit.SerialNumber]
		if !rejected {
			bucketID, ok := bucketMap[request.OrderLimit.SerialNumber]
			if ok {
				projectID, bucketName, err := splitBucketID(bucketID)
				if err != nil {
					return []*pb.SettlementResponse{}, errs.Wrap(err)
				}

				updateRollupStatement += db.buildUpdateBucketBandwidthRollupStatements(request.OrderLimit, request.Order, projectID[:], bucketName, intervalStart)
				updateRollupStatement += db.buildUpdateStorageNodeBandwidthRollupStatements(request.OrderLimit, request.Order, intervalStart)
			} else {
				reject(request.OrderLimit.SerialNumber)
			}
		}
	}

	_, err = tx.Exec(updateRollupStatement)
	if err != nil {
		return []*pb.SettlementResponse{}, errs.Wrap(err)
	}
	for _, request := range requests {
		_, rejected := rejectedRequests[request.OrderLimit.SerialNumber]
		if !rejected {
			r := &pb.SettlementResponse{
				SerialNumber: request.OrderLimit.SerialNumber,
				Status:       pb.SettlementResponse_ACCEPTED,
			}

			responses = append(responses, r)
		}
	}
	return responses, nil
}

var (
	errUsingSerialNumber = errs.Class("serial number")
)

func (db *ordersDB) buildInsertUseSerialStatement(orderLimit *pb.OrderLimit) string {
	return fmt.Sprintf("INSERT INTO  used_serials (serial_number_id, storage_node_id) SELECT id, %v FROM serial_numbers WHERE serial_number = %s;\n",
		db.toHex(orderLimit.StorageNodeId.Bytes()), db.toHex(orderLimit.SerialNumber.Bytes()))
}

func (db *ordersDB) buildGetBucketIdsQuery(argCount int) string {
	args := make([]string, argCount)
	for i := 0; i < argCount; i++ {
		args[i] = "?"
	}
	return fmt.Sprintf("SELECT serial_number, bucket_id FROM serial_numbers WHERE serial_number IN (%s);\n", strings.Join(args, ","))
}

func (db *ordersDB) buildUpdateBucketBandwidthRollupStatements(orderLimit *pb.OrderLimit, order *pb.Order, projectID []byte, bucketName []byte, intervalStart time.Time) string {
	return fmt.Sprintf(`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (%s, %s, '%s', %d, %d, %d, %d, %d)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET settled = bucket_bandwidth_rollups.settled + %d;
`, db.toHex(bucketName), db.toHex(projectID), intervalStart.Format("2006-01-02 15:04:05+00:00"), defaultIntervalSeconds, orderLimit.Action, 0, 0, uint64(order.Amount), uint64(order.Amount))
}

func (db *ordersDB) buildUpdateStorageNodeBandwidthRollupStatements(orderLimit *pb.OrderLimit, order *pb.Order, intervalStart time.Time) string {
	return fmt.Sprintf(`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, allocated, settled)
		VALUES (%s, '%s', %d, %d, %d, %d)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + %d;
`, db.toHex(orderLimit.StorageNodeId.Bytes()), intervalStart.Format("2006-01-02 15:04:05+00:00"), defaultIntervalSeconds, orderLimit.Action, 0, uint64(order.Amount), uint64(order.Amount))
}

func (db *ordersDB) toHex(value []byte) string {
	hexValue := hex.EncodeToString(value)
	switch db.db.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		return fmt.Sprintf("X'%v'", hexValue)
	case *pq.Driver:
		return fmt.Sprintf("decode('%v', 'hex')", hexValue)
	default:
		return ""
	}
}

func formatError(err error) error {
	if err == io.EOF {
		return nil
	}
	return status.Error(codes.Unknown, err.Error())
}

func splitBucketID(bucketID []byte) (projectID *uuid.UUID, bucketName []byte, err error) {
	pathElements := bytes.Split(bucketID, []byte("/"))
	if len(pathElements) > 1 {
		bucketName = pathElements[1]
	}
	projectID, err = uuid.Parse(string(pathElements[0]))
	if err != nil {
		return nil, nil, err
	}
	return projectID, bucketName, nil
}
