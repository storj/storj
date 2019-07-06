// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
)

type bandwidthdb struct {
	*InfoDB
}

// Bandwidth returns table for storing bandwidth usage.
func (db *DB) Bandwidth() bandwidth.DB { return db.info.Bandwidth() }

// Bandwidth returns table for storing bandwidth usage.
func (db *InfoDB) Bandwidth() bandwidth.DB { return &bandwidthdb{db} }

// Add adds bandwidth usage to the table
func (db *bandwidthdb) Add(ctx context.Context, satelliteID storj.NodeID, action pb.PieceAction, amount int64, created time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Exec(`
		INSERT INTO
			bandwidth_usage(satellite_id, action, amount, created_at)
		VALUES(?, ?, ?, ?)`, satelliteID, action, amount, created)
	if err == nil {
		db.bandwidth.mu.Lock()
		defer db.bandwidth.mu.Unlock()

		beginningOfMonth := getBeginningOfMonth(created.UTC())
		if beginningOfMonth.Equal(db.bandwidth.usedSince) {
			db.bandwidth.usedBandwidth += amount
		} else {
			usage, err := db.Summary(ctx, beginningOfMonth, time.Now().UTC())
			if err == nil {
				return err
			}
			db.bandwidth.usedSince = beginningOfMonth
			db.bandwidth.usedBandwidth = usage.Total()
		}
	}
	return ErrInfo.Wrap(err)
}

// CachedBandwidthUsed returns summary of bandwidth usages
func (db *bandwidthdb) CachedBandwidthUsed(ctx context.Context) (_ int64, err error) {
	db.bandwidth.mu.RLock()
	beginningOfMonth := getBeginningOfMonth(time.Now().UTC())
	if beginningOfMonth.Equal(db.bandwidth.usedSince) {
		defer db.bandwidth.mu.RUnlock()
		return db.bandwidth.usedBandwidth, nil
	}

	db.bandwidth.mu.RUnlock()
	db.bandwidth.mu.Lock()
	defer db.bandwidth.mu.Unlock()
	// double check no one else changed this
	if !beginningOfMonth.Equal(db.bandwidth.usedSince) {
		usage, err := db.Summary(ctx, beginningOfMonth, time.Now())
		if err != nil {
			return 0, err
		}
		db.bandwidth.usedSince = beginningOfMonth
		db.bandwidth.usedBandwidth = usage.Total()
	}
	return db.bandwidth.usedBandwidth, nil
}

// Summary returns summary of bandwidth usages
func (db *bandwidthdb) Summary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	usage := &bandwidth.Usage{}

	rows, err := db.db.Query(`
		SELECT action, sum(amount)
		FROM bandwidth_usage
		WHERE ? <= created_at AND created_at <= ?
		GROUP BY action`, from, to)
	if err != nil {
		if err == sql.ErrNoRows {
			return usage, nil
		}
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var action pb.PieceAction
		var amount int64
		err := rows.Scan(&action, &amount)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}
		usage.Include(action, amount)
	}

	return usage, ErrInfo.Wrap(rows.Err())
}

// SummaryBySatellite returns summary of bandwidth usage grouping by satellite.
func (db *bandwidthdb) SummaryBySatellite(ctx context.Context, from, to time.Time) (_ map[storj.NodeID]*bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	entries := map[storj.NodeID]*bandwidth.Usage{}

	rows, err := db.db.Query(`
		SELECT satellite_id, action, sum(amount)
		FROM bandwidth_usage
		WHERE ? <= created_at AND created_at <= ?
		GROUP BY satellite_id, action`, from, to)
	if err != nil {
		if err == sql.ErrNoRows {
			return entries, nil
		}
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var satelliteID storj.NodeID
		var action pb.PieceAction
		var amount int64

		err := rows.Scan(&satelliteID, &action, &amount)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		entry, ok := entries[satelliteID]
		if !ok {
			entry = &bandwidth.Usage{}
			entries[satelliteID] = entry
		}

		entry.Include(action, amount)
	}

	return entries, ErrInfo.Wrap(rows.Err())
}

func getBeginningOfMonth(now time.Time) time.Time {
	y, m, _ := now.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.Now().UTC().Location())
}
