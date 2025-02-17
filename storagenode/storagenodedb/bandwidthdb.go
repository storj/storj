// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/sqliteutil"
	"storj.io/storj/shared/tagsql"
	"storj.io/storj/storagenode/bandwidth"
)

// ErrBandwidth represents errors from the bandwidthdb database.
var ErrBandwidth = errs.Class("bandwidthdb")

// BandwidthDBName represents the database name.
const BandwidthDBName = "bandwidth"

// BandwidthDB is a database for tracking bandwidth usage.
type BandwidthDB struct {
	// Moved to top of struct to resolve alignment issue with atomic operations on ARM
	usedSpace int64
	usedMu    sync.RWMutex
	usedSince time.Time

	dbContainerImpl
}

var monAdd = mon.Task()

// Add adds bandwidth usage to the table.
func (db *BandwidthDB) Add(ctx context.Context, satelliteID storj.NodeID, action pb.PieceAction, amount int64, created time.Time) (err error) {
	defer monAdd(&ctx)(&err)

	var usage bandwidth.Usage
	usage.Include(action, amount)
	created = created.UTC()
	created = time.Date(created.Year(), created.Month(), created.Day(), 0, 0, 0, 0, created.Location())

	_, err = db.ExecContext(ctx, `
		INSERT INTO
			bandwidth_usage(interval_start, satellite_id, get_total, get_audit_total, get_repair_total, put_total, put_repair_total, delete_total)
		VALUES(datetime(?), ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (interval_start, satellite_id) DO UPDATE SET
			put_total = put_total + excluded.put_total,
			get_total = get_total + excluded.get_total,
			get_audit_total = get_audit_total + excluded.get_audit_total,
			get_repair_total = get_repair_total + excluded.get_repair_total,
			put_repair_total = put_repair_total + excluded.put_repair_total,
			delete_total = delete_total + excluded.delete_total;`,
		created, satelliteID, usage.Get, usage.GetAudit, usage.GetRepair, usage.Put, usage.PutRepair, usage.Delete,
	)
	if err == nil {
		db.usedMu.Lock()
		defer db.usedMu.Unlock()

		beginningOfMonth := getBeginningOfMonth(created.UTC())
		if beginningOfMonth.Equal(db.usedSince) {
			db.usedSpace += amount
		} else if beginningOfMonth.After(db.usedSince) {
			usage, err := db.Summary(ctx, beginningOfMonth, time.Now())
			if err != nil {
				return err
			}
			db.usedSince = beginningOfMonth
			db.usedSpace = usage.Total()
		}
	}
	return ErrBandwidth.Wrap(err)
}

// MonthSummary returns summary of the current months bandwidth usages.
func (db *BandwidthDB) MonthSummary(ctx context.Context, now time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	db.usedMu.RLock()
	beginningOfMonth := getBeginningOfMonth(now)
	if beginningOfMonth.Equal(db.usedSince) {
		defer db.usedMu.RUnlock()
		return db.usedSpace, nil
	}
	db.usedMu.RUnlock()

	usage, err := db.Summary(ctx, beginningOfMonth, now)
	if err != nil {
		return 0, err
	}
	// Just return the usage, don't update the cache. Let add handle updates
	return usage.Total(), nil
}

// Summary returns summary of bandwidth usages for all satellites.
func (db *BandwidthDB) Summary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	var usage bandwidth.Usage

	from, to = from.UTC(), to.UTC()

	rows := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(get_total), 0),
			COALESCE(SUM(get_audit_total), 0),
			COALESCE(SUM(get_repair_total), 0),
			COALESCE(SUM(put_total), 0),
			COALESCE(SUM(put_repair_total), 0),
			COALESCE(SUM(delete_total), 0)
		FROM bandwidth_usage
		WHERE datetime(?) <= interval_start AND interval_start <= datetime(?);
		`, from, to)

	err = rows.Scan(&usage.Get, &usage.GetAudit, &usage.GetRepair, &usage.Put, &usage.PutRepair, &usage.Delete)
	if err != nil {
		return nil, ErrBandwidth.Wrap(err)
	}

	return &usage, ErrBandwidth.Wrap(rows.Err())
}

// EgressSummary returns summary of egress usages for all satellites.
func (db *BandwidthDB) EgressSummary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	var usage bandwidth.Usage

	from, to = from.UTC(), to.UTC()

	row := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(get_total), 0),
			COALESCE(SUM(get_audit_total), 0),
			COALESCE(SUM(get_repair_total), 0)
		FROM bandwidth_usage
		WHERE datetime(?) <= interval_start AND interval_start <= datetime(?);
		`, from, to)

	err = row.Scan(&usage.Get, &usage.GetAudit, &usage.GetRepair)
	if err != nil {
		return nil, ErrBandwidth.Wrap(err)
	}

	return &usage, ErrBandwidth.Wrap(row.Err())
}

// IngressSummary returns summary of ingress usages for all satellites.
func (db *BandwidthDB) IngressSummary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	var usage bandwidth.Usage

	from, to = from.UTC(), to.UTC()

	row := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(put_total), 0),
			COALESCE(SUM(put_repair_total), 0)
		FROM bandwidth_usage
		WHERE datetime(?) <= interval_start AND interval_start <= datetime(?);
		`, from, to)

	err = row.Scan(&usage.Put, &usage.PutRepair)
	if err != nil {
		return nil, ErrBandwidth.Wrap(err)
	}

	return &usage, ErrBandwidth.Wrap(row.Err())
}

// SatelliteSummary returns summary of bandwidth usages for a particular satellite.
func (db *BandwidthDB) SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	var usage bandwidth.Usage

	from, to = from.UTC(), to.UTC()

	row := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(get_total), 0),
			COALESCE(SUM(get_audit_total), 0),
			COALESCE(SUM(get_repair_total), 0),
			COALESCE(SUM(put_total), 0),
			COALESCE(SUM(put_repair_total), 0),
			COALESCE(SUM(delete_total), 0)
		FROM bandwidth_usage
		WHERE datetime(?) <= interval_start AND interval_start <= datetime(?)
		AND satellite_id = ?;
		`, from, to, satelliteID)

	err = row.Scan(&usage.Get, &usage.GetAudit, &usage.GetRepair, &usage.Put, &usage.PutRepair, &usage.Delete)
	if err != nil {
		return nil, ErrBandwidth.Wrap(err)
	}

	return &usage, ErrBandwidth.Wrap(row.Err())
}

// SatelliteEgressSummary returns summary of egress usage for a particular satellite.
func (db *BandwidthDB) SatelliteEgressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	var usage bandwidth.Usage

	from, to = from.UTC(), to.UTC()

	row := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(get_total), 0),
			COALESCE(SUM(get_audit_total), 0),
			COALESCE(SUM(get_repair_total), 0)
		FROM bandwidth_usage
		WHERE datetime(?) <= interval_start AND interval_start <= datetime(?)
		AND satellite_id = ?;
		`, from, to, satelliteID)

	err = row.Scan(&usage.Get, &usage.GetAudit, &usage.GetRepair)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &usage, nil
		}
		return nil, ErrBandwidth.Wrap(err)
	}

	return &usage, ErrBandwidth.Wrap(row.Err())
}

// SatelliteIngressSummary returns summary of ingress usage for a particular satellite.
func (db *BandwidthDB) SatelliteIngressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	var usage bandwidth.Usage

	from, to = from.UTC(), to.UTC()

	row := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(put_total), 0),
			COALESCE(SUM(put_repair_total), 0)
		FROM bandwidth_usage
		WHERE datetime(?) <= interval_start AND interval_start <= datetime(?)
		AND satellite_id = ?;
		`, from, to, satelliteID)

	err = row.Scan(&usage.Put, &usage.PutRepair)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &usage, nil
		}
		return nil, ErrBandwidth.Wrap(err)
	}

	return &usage, ErrBandwidth.Wrap(row.Err())
}

// SummaryBySatellite returns summary of bandwidth usage grouping by satellite.
func (db *BandwidthDB) SummaryBySatellite(ctx context.Context, from, to time.Time) (_ map[storj.NodeID]*bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	entries := map[storj.NodeID]*bandwidth.Usage{}

	from, to = from.UTC(), to.UTC()

	// get all satellites with data in the range
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT satellite_id
		FROM bandwidth_usage
		WHERE datetime(?) <= interval_start AND interval_start <= datetime(?);
		`, from, to)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entries, nil
		}
		return nil, ErrBandwidth.Wrap(err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var satelliteID storj.NodeID
		err := rows.Scan(&satelliteID)
		if err != nil {
			return nil, ErrBandwidth.Wrap(err)
		}
		entries[satelliteID] = &bandwidth.Usage{}
	}

	// get the usage for each satellite
	for id, usage := range entries {
		satelliteUsage, err := db.SatelliteSummary(ctx, id, from, to)
		if err != nil {
			return nil, ErrBandwidth.Wrap(err)
		}
		usage.Add(satelliteUsage)
	}

	return entries, ErrBandwidth.Wrap(rows.Err())
}

// AddBatch adds bandwidth usage to the table.
func (db *BandwidthDB) AddBatch(ctx context.Context, usages map[bandwidth.CacheKey]*bandwidth.Usage) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(usages) == 0 {
		return nil
	}

	query := `INSERT INTO bandwidth_usage (interval_start, satellite_id, get_total, get_audit_total, get_repair_total, put_total, put_repair_total, delete_total)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(interval_start, satellite_id)
				DO UPDATE SET
					put_total = put_total + excluded.put_total,
					get_total = get_total + excluded.get_total,
					get_audit_total = get_audit_total + excluded.get_audit_total,
					get_repair_total = get_repair_total + excluded.get_repair_total,
					put_repair_total = put_repair_total + excluded.put_repair_total,
					delete_total = delete_total + excluded.delete_total;`

	return sqliteutil.WithTx(ctx, db.GetDB(), func(ctx context.Context, tx tagsql.Tx) error {
		for key, usage := range usages {
			_, err := tx.ExecContext(ctx, query, key.CreatedAt, key.SatelliteID, usage.Get, usage.GetAudit, usage.GetRepair, usage.Put, usage.PutRepair, usage.Delete)
			if err != nil {
				return ErrBandwidth.Wrap(err)
			}
		}

		return nil
	})
}

// GetDailyRollups returns slice of daily bandwidth usage rollups for provided time range,
// sorted in ascending order.
func (db *BandwidthDB) GetDailyRollups(ctx context.Context, from, to time.Time) (_ []bandwidth.UsageRollup, err error) {
	defer mon.Task()(&ctx, from, to)(&err)

	since, _ := date.DayBoundary(from.UTC())
	_, before := date.DayBoundary(to.UTC())

	return db.getDailyUsageRollups(ctx,
		"WHERE datetime(?) <= interval_start AND interval_start <= datetime(?)",
		since, before)
}

// GetDailySatelliteRollups returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order for a particular satellite.
func (db *BandwidthDB) GetDailySatelliteRollups(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []bandwidth.UsageRollup, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	since, _ := date.DayBoundary(from.UTC())
	_, before := date.DayBoundary(to.UTC())

	return db.getDailyUsageRollups(ctx,
		"WHERE satellite_id = ? AND datetime(?) <= interval_start AND interval_start <= datetime(?)",
		satelliteID, since, before)
}

// getDailyUsageRollups returns slice of grouped by date bandwidth usage rollups
// sorted in ascending order and applied condition if any.
func (db *BandwidthDB) getDailyUsageRollups(ctx context.Context, cond string, args ...interface{}) (_ []bandwidth.UsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	var usageRollups []bandwidth.UsageRollup

	query := `
		SELECT
			interval_start,
			SUM(get_total),
			SUM(get_audit_total),
			SUM(get_repair_total),
			SUM(put_total),
			SUM(put_repair_total),
			SUM(delete_total)
		FROM bandwidth_usage
		` + cond + ` 
		GROUP BY interval_start
		ORDER BY interval_start`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return usageRollups, nil
		}
		return nil, ErrBandwidth.Wrap(err)
	}
	defer func() {
		err = ErrBandwidth.Wrap(errs.Combine(err, rows.Close()))
	}()

	for rows.Next() {
		var intervalStartN dbutil.NullTime
		var usage bandwidth.Usage

		err = rows.Scan(&intervalStartN, &usage.Get, &usage.GetAudit, &usage.GetRepair, &usage.Put, &usage.PutRepair, &usage.Delete)
		if err != nil {
			return nil, err
		}

		intervalStart := intervalStartN.Time

		rollup := usage.Rollup(intervalStart)

		usageRollups = append(usageRollups, *rollup)
	}

	return usageRollups, ErrBandwidth.Wrap(rows.Err())
}

func getBeginningOfMonth(now time.Time) time.Time {
	y, m, _ := now.UTC().Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}
