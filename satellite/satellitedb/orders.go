// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

const defaultIntervalSeconds = int(time.Hour / time.Second)

type ordersDB struct {
	db *dbx.DB
}

func (db *ordersDB) CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) error {
	_, err := db.db.Create_SerialNumber(
		ctx,
		dbx.SerialNumber_SerialNumber(serialNumber.Bytes()),
		dbx.SerialNumber_BucketId(bucketID),
		dbx.SerialNumber_ExpiresAt(limitExpiration),
	)
	return err
}

func (db *ordersDB) UseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) ([]byte, error) {
	statement := db.db.Rebind(
		`INSERT INTO used_serials (serial_number_id, storage_node_id)
		SELECT id, ? FROM serial_numbers WHERE serial_number = ?`,
	)
	_, err := db.db.ExecContext(ctx, statement, storageNodeID.Bytes(), serialNumber.Bytes())
	if err != nil {
		return nil, err
	}

	dbxSerialNumber, err := db.db.Find_SerialNumber_By_SerialNumber(
		ctx,
		dbx.SerialNumber_SerialNumber(serialNumber.Bytes()),
	)
	if err != nil {
		return nil, err
	}
	return dbxSerialNumber.BucketId, nil
}

// UpdateBucketBandwidthAllocation
func (db *ordersDB) UpdateBucketBandwidthAllocation(ctx context.Context, bucketID []byte, action pb.PieceAction, amount int64) error {
	now := time.Now()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_id, interval_start, action)
		DO UPDATE SET allocated = bucket_bandwidth_rollups.allocated + ?`,
	)
	_, err := db.db.ExecContext(ctx, statement, bucketID, intervalStart, defaultIntervalSeconds, action, 0, uint64(amount), 0, uint64(amount))
	if err != nil {
		return err
	}

	return nil
}

// UpdateBucketBandwidthSettle
func (db *ordersDB) UpdateBucketBandwidthSettle(ctx context.Context, bucketID []byte, action pb.PieceAction, amount int64) error {
	now := time.Now()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_id, interval_start, action)
		DO UPDATE SET settled = bucket_bandwidth_rollups.settled + ?`,
	)
	_, err := db.db.ExecContext(ctx, statement, bucketID, intervalStart, defaultIntervalSeconds, action, 0, 0, uint64(amount), uint64(amount))
	if err != nil {
		return err
	}
	return nil
}

// UpdateBucketBandwidthInline
func (db *ordersDB) UpdateBucketBandwidthInline(ctx context.Context, bucketID []byte, action pb.PieceAction, amount int64) error {
	now := time.Now()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_id, interval_start, action)
		DO UPDATE SET inline = bucket_bandwidth_rollups.inline + ?`,
	)
	_, err := db.db.ExecContext(ctx, statement, bucketID, intervalStart, defaultIntervalSeconds, action, uint64(amount), 0, 0, uint64(amount))
	if err != nil {
		return err
	}
	return nil
}

// UpdateStoragenodeBandwidthAllocation
func (db *ordersDB) UpdateStoragenodeBandwidthAllocation(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64) error {
	now := time.Now()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	statement := db.db.Rebind(
		`INSERT INTO storagenode_bandwidth_rollups VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET allocated = storagenode_bandwidth_rollups.allocated + ?`,
	)
	_, err := db.db.ExecContext(ctx, statement, storageNode.Bytes(), intervalStart, defaultIntervalSeconds, action, uint64(amount), 0, uint64(amount))
	if err != nil {
		return err
	}
	return nil
}

// UpdateStoragenodeBandwidthSettle
func (db *ordersDB) UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64) error {
	now := time.Now()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	statement := db.db.Rebind(
		`INSERT INTO storagenode_bandwidth_rollups VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + ?`,
	)
	_, err := db.db.ExecContext(ctx, statement, storageNode.Bytes(), intervalStart, defaultIntervalSeconds, action, 0, uint64(amount), uint64(amount))
	if err != nil {
		return err
	}
	return nil
}

// GetBucketBandwidth
func (db *ordersDB) GetBucketBandwidth(ctx context.Context, bucketID []byte, from, to time.Time) (int64, error) {
	var sum *int64
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE bucket_id = ? AND interval_start > ? AND interval_start <= ?`
	err := db.db.QueryRow(db.db.Rebind(query), bucketID, from, to).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// GetStorageNodeBandwidth
func (db *ordersDB) GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error) {
	var sum *int64
	query := `SELECT SUM(settled) FROM storagenode_bandwidth_rollups WHERE storagenode_id = ? AND interval_start > ? AND interval_start <= ?`
	err := db.db.QueryRow(db.db.Rebind(query), nodeID.Bytes(), from, to).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}
