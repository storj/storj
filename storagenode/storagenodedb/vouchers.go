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
	"storj.io/storj/storagenode/vouchers"
)

type vouchersdb struct{ *InfoDB }

// Vouchers returns database for storing vouchers
func (db *DB) Vouchers() vouchers.DB { return db.info.Vouchers() }

// Vouchers returns database for storing vouchers
func (db *InfoDB) Vouchers() vouchers.DB { return &vouchersdb{db} }

// Put inserts or updates a voucher from a satellite
func (db *vouchersdb) Put(ctx context.Context, voucher *pb.Voucher) (err error) {
	defer mon.Task()(&ctx)(&err)

	voucherSerialized, err := proto.Marshal(voucher)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	_, err = db.db.Exec(`
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
func (db *vouchersdb) NeedVoucher(ctx context.Context, satelliteID storj.NodeID, expirationBuffer time.Duration) (need bool, err error) {
	defer mon.Task()(&ctx)(&err)

	expiresBefore := time.Now().Add(expirationBuffer)

	// query returns row if voucher is good. If not, it is either expiring or does not exist
	row := db.db.QueryRow(`
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
		return false, ErrInfo.Wrap(err)
	}
	return false, nil
}

// GetAll returns all vouchers in the table
func (db *vouchersdb) GetAll(ctx context.Context) (vouchers []*pb.Voucher, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.Query(`
		SELECT voucher_serialized
		FROM vouchers
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var voucherSerialized []byte
		err := rows.Scan(&voucherSerialized)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}
		voucher := &pb.Voucher{}
		err = proto.Unmarshal(voucherSerialized, voucher)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}
		vouchers = append(vouchers, voucher)
	}

	return vouchers, nil
}
