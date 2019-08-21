// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
)

// ErrBandwidth represents errors from the bandwidthdb database.
var ErrBandwidth = errs.Class("bandwidthdb error")

type bandwidthDB struct {
	// Moved to top of struct to resolve alignment issue with atomic operations on ARM
	usedSpace int64
	usedMu    sync.RWMutex
	usedSince time.Time

	location string
	SQLDB
}

// newBandwidthDB returns a new instance of usedSerials initialized with the specified database.
func newBandwidthDB(db SQLDB, location string) *bandwidthDB {
	return &bandwidthDB{
		location: location,
		SQLDB:    db,
	}
}

// Add adds bandwidth usage to the table
func (db *bandwidthDB) Add(ctx context.Context, satelliteID storj.NodeID, action pb.PieceAction, amount int64, created time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.Exec(`
		INSERT INTO
			bandwidth_usage(satellite_id, action, amount, created_at)
		VALUES(?, ?, ?, ?)`, satelliteID, action, amount, created.UTC())
	if err == nil {
		db.usedMu.Lock()
		defer db.usedMu.Unlock()

		beginningOfMonth := getBeginningOfMonth(created.UTC())
		if beginningOfMonth.Equal(db.usedSince) {
			db.usedSpace += amount
		} else if beginningOfMonth.After(db.usedSince) {
			usage, err := db.Summary(ctx, beginningOfMonth, time.Now().UTC())
			if err != nil {
				return err
			}
			db.usedSince = beginningOfMonth
			db.usedSpace = usage.Total()
		}
	}
	return ErrBandwidth.Wrap(err)
}

// MonthSummary returns summary of the current months bandwidth usages
func (db *bandwidthDB) MonthSummary(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	db.usedMu.RLock()
	beginningOfMonth := getBeginningOfMonth(time.Now().UTC())
	if beginningOfMonth.Equal(db.usedSince) {
		defer db.usedMu.RUnlock()
		return db.usedSpace, nil
	}
	db.usedMu.RUnlock()

	usage, err := db.Summary(ctx, beginningOfMonth, time.Now())
	if err != nil {
		return 0, err
	}
	// Just return the usage, don't update the cache. Let add handle updates
	return usage.Total(), nil
}

// Summary returns summary of bandwidth usages
func (db *bandwidthDB) Summary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	usage := &bandwidth.Usage{}

	from = from.UTC()
	to = to.UTC()
	rows, err := db.Query(`
		SELECT action, sum(a) amount from(
				SELECT action, sum(amount) a
				FROM bandwidth_usage
				WHERE datetime(?) <= datetime(created_at) AND datetime(created_at) <= datetime(?)
				GROUP BY action
				UNION ALL
				SELECT action, sum(amount) a
				FROM bandwidth_usage_rollups
				WHERE datetime(?) <= datetime(interval_start) AND datetime(interval_start) <= datetime(?)
				GROUP BY action
		) GROUP BY action;
		`, from, to, from, to)
	if err != nil {
		if err == sql.ErrNoRows {
			return usage, nil
		}
		return nil, ErrBandwidth.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var action pb.PieceAction
		var amount int64
		err := rows.Scan(&action, &amount)
		if err != nil {
			return nil, ErrBandwidth.Wrap(err)
		}
		usage.Include(action, amount)
	}

	return usage, ErrBandwidth.Wrap(rows.Err())
}

// SummaryBySatellite returns summary of bandwidth usage grouping by satellite.
func (db *bandwidthDB) SummaryBySatellite(ctx context.Context, from, to time.Time) (_ map[storj.NodeID]*bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	entries := map[storj.NodeID]*bandwidth.Usage{}

	from = from.UTC()
	to = to.UTC()
	rows, err := db.Query(`
	SELECT satellite_id, action, sum(a) amount from(
			SELECT satellite_id, action, sum(amount) a
			FROM bandwidth_usage
			WHERE datetime(?) <= datetime(created_at) AND datetime(created_at) <= datetime(?)
			GROUP BY satellite_id, action
			UNION ALL
			SELECT satellite_id, action, sum(amount) a
			FROM bandwidth_usage_rollups
			WHERE datetime(?) <= datetime(interval_start) AND datetime(interval_start) <= datetime(?)
			GROUP BY satellite_id, action
		) GROUP BY satellite_id, action;
		`, from, to, from, to)
	if err != nil {
		if err == sql.ErrNoRows {
			return entries, nil
		}
		return nil, ErrBandwidth.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var satelliteID storj.NodeID
		var action pb.PieceAction
		var amount int64

		err := rows.Scan(&satelliteID, &action, &amount)
		if err != nil {
			return nil, ErrBandwidth.Wrap(err)
		}

		entry, ok := entries[satelliteID]
		if !ok {
			entry = &bandwidth.Usage{}
			entries[satelliteID] = entry
		}

		entry.Include(action, amount)
	}

	return entries, ErrBandwidth.Wrap(rows.Err())
}

// Rollup bandwidth_usage data earlier than the current hour, then delete the rolled up records
func (db *bandwidthDB) Rollup(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().UTC()

	// Go back an hour to give us room for late persists
	hour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location()).Add(-time.Hour)

	tx, err := db.Begin()
	if err != nil {
		return ErrBandwidth.Wrap(err)
	}

	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()

	result, err := tx.Exec(`
		INSERT INTO bandwidth_usage_rollups (interval_start, satellite_id,  action, amount)
		SELECT datetime(strftime('%Y-%m-%dT%H:00:00', created_at)) created_hr, satellite_id, action, SUM(amount)
			FROM bandwidth_usage
		WHERE datetime(created_at) < datetime(?)
		GROUP BY created_hr, satellite_id, action
		ON CONFLICT(interval_start, satellite_id,  action)
		DO UPDATE SET amount = bandwidth_usage_rollups.amount + excluded.amount;

		DELETE FROM bandwidth_usage WHERE datetime(created_at) < datetime(?);
	`, hour, hour)
	if err != nil {
		return ErrBandwidth.Wrap(err)
	}

	_, err = result.RowsAffected()
	if err != nil {
		return ErrBandwidth.Wrap(err)
	}

	return nil
}

func getBeginningOfMonth(now time.Time) time.Time {
	y, m, _ := now.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.Now().UTC().Location())
}
