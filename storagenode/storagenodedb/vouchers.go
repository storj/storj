// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

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

	id := voucher.SatelliteId
	expiration, err := ptypes.Timestamp(voucher.GetExpiration())
	if err != nil {
		return ErrInfo.Wrap(err)
	}

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
	`, id, voucherSerialized, expiration, voucherSerialized, expiration)

	return err
}

// NeedVoucher returns true if a voucher from a particular satellite is expired, about to expire, or does not exist
func (db *vouchersdb) NeedVoucher(ctx context.Context, satelliteID storj.NodeID, expirationBuffer time.Duration) (need bool, err error) {
	defer mon.Task()(&ctx)(&err)

	expiresBefore := time.Now().UTC().Add(expirationBuffer)

	// query returns row if voucher is good. If not, it is either expiring or does not exist
	row := db.db.QueryRow(`
		SELECT satellite_id
		FROM vouchers
		WHERE satellite_id = ? AND expiration >= ?
	`, satelliteID, expiresBefore)

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

// GetValid returns one valid voucher from the list of approved satellites
func (db *vouchersdb) GetValid(ctx context.Context, satellites []storj.NodeID) (*pb.Voucher, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	var args []interface{}

	idCondition := `satellite_id IN (?` + strings.Repeat(", ?", len(satellites)-1) + `)`

	for _, id := range satellites {
		args = append(args, id)
	}

	args = append(args, time.Now().UTC())

	row := db.db.QueryRow(db.InfoDB.Rebind(`
		SELECT voucher_serialized
		FROM vouchers
		WHERE `+idCondition+`
			AND expiration > ?
		LIMIT 1
	`), args...)

	var bytes []byte
	err = row.Scan(&bytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrInfo.Wrap(err)
	}

	voucher := &pb.Voucher{}
	err = proto.Unmarshal(bytes, voucher)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	return voucher, nil
}
