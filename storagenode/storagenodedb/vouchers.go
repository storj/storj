// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// ErrVouchers represents errors from the vouchers database.
var ErrVouchers = errs.Class("vouchersdb error")

type vouchersDB struct {
	location string
	SQLDB
}

// newVouchersDB returns a new instance of vouchersdb initialized with the specified database.
func newVouchersDB(db SQLDB, location string) *vouchersDB {
	return &vouchersDB{
		location: location,
		SQLDB:    db,
	}
}

// Put inserts or updates a voucher from a satellite
func (db *vouchersDB) Put(ctx context.Context, voucher *pb.Voucher) (err error) {
	defer mon.Task()(&ctx)(&err)

	voucherSerialized, err := proto.Marshal(voucher)
	if err != nil {
		return ErrVouchers.Wrap(err)
	}

	_, err = db.Exec(`
		INSERT INTO vouchers(
			satellite_id,
			voucher_serialized,
			expiration
		) VALUES (?, ?, ?)
			ON CONFLICT(satellite_id) DO UPDATE SET
				voucher_serialized = ?,
				expiration = ?
	`, voucher.SatelliteId, voucherSerialized, voucher.Expiration.UTC(), voucherSerialized, voucher.Expiration.UTC())

	return err
}

// NeedVoucher returns true if a voucher from a particular satellite is expired, about to expire, or does not exist
func (db *vouchersDB) NeedVoucher(ctx context.Context, satelliteID storj.NodeID, expirationBuffer time.Duration) (need bool, err error) {
	defer mon.Task()(&ctx)(&err)

	expiresBefore := time.Now().Add(expirationBuffer)

	// query returns row if voucher is good. If not, it is either expiring or does not exist
	row := db.QueryRow(`
		SELECT satellite_id
		FROM vouchers
		WHERE satellite_id = ? AND expiration >= ?
	`, satelliteID, expiresBefore.UTC())

	var bytes []byte
	err = row.Scan(&bytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, ErrVouchers.Wrap(err)
	}
	return false, nil
}

// GetAll returns all vouchers in the table
func (db *vouchersDB) GetAll(ctx context.Context) (vouchers []*pb.Voucher, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.Query(`
		SELECT voucher_serialized
		FROM vouchers
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrVouchers.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var voucherSerialized []byte
		err := rows.Scan(&voucherSerialized)
		if err != nil {
			return nil, ErrVouchers.Wrap(err)
		}
		voucher := &pb.Voucher{}
		err = proto.Unmarshal(voucherSerialized, voucher)
		if err != nil {
			return nil, ErrVouchers.Wrap(err)
		}
		vouchers = append(vouchers, voucher)
	}

	return vouchers, nil
}
