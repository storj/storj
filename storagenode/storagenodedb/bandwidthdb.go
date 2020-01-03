// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/storagenode/bandwidth"
)

// ErrBandwidth represents errors from the bandwidthdb database.
var ErrBandwidth = errs.Class("bandwidthdb error")

// BandwidthDBName represents the database name.
const BandwidthDBName = "bandwidth"

type bandwidthDB struct {
	// Moved to top of struct to resolve alignment issue with atomic operations on ARM
	usedSpace int64
	usedMu    sync.RWMutex
	usedSince time.Time

	dbContainerImpl
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

// actionFilter sums bandwidth depending on piece action type.
type actionFilter func(action pb.PieceAction, amount int64, usage *bandwidth.Usage)

var (
	// ingressFilter sums put and put repair.
	ingressFilter actionFilter = func(action pb.PieceAction, amount int64, usage *bandwidth.Usage) {
		switch action {
		case pb.PieceAction_PUT, pb.PieceAction_PUT_REPAIR:
			usage.Include(action, amount)
		}
	}

	// egressFilter sums get, get audit and get repair.
	egressFilter actionFilter = func(action pb.PieceAction, amount int64, usage *bandwidth.Usage) {
		switch action {
		case pb.PieceAction_GET, pb.PieceAction_GET_AUDIT, pb.PieceAction_GET_REPAIR:
			usage.Include(action, amount)
		}
	}

	// bandwidthFilter sums all bandwidth.
	bandwidthFilter actionFilter = func(action pb.PieceAction, amount int64, usage *bandwidth.Usage) {
		usage.Include(action, amount)
	}
)

// Summary returns summary of bandwidth usages for all satellites.
func (db *bandwidthDB) Summary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	return db.getSummary(ctx, from, to, bandwidthFilter)
}

// EgressSummary returns summary of egress usages for all satellites.
func (db *bandwidthDB) EgressSummary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	return db.getSummary(ctx, from, to, egressFilter)
}

// IngressSummary returns summary of ingress usages for all satellites.
func (db *bandwidthDB) IngressSummary(ctx context.Context, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	return db.getSummary(ctx, from, to, ingressFilter)
}

// getSummary returns bandwidth data for all satellites.
func (db *bandwidthDB) getSummary(ctx context.Context, from, to time.Time, filter actionFilter) (_ *bandwidth.Usage, err error) {
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

		filter(action, amount, usage)
	}

	return usage, ErrBandwidth.Wrap(rows.Err())
}

// SatelliteSummary returns summary of bandwidth usages for a particular satellite.
func (db *bandwidthDB) SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	return db.getSatelliteSummary(ctx, satelliteID, from, to, bandwidthFilter)
}

// SatelliteEgressSummary returns summary of egress usage for a particular satellite.
func (db *bandwidthDB) SatelliteEgressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	return db.getSatelliteSummary(ctx, satelliteID, from, to, egressFilter)
}

// SatelliteIngressSummary returns summary of ingress usage for a particular satellite.
func (db *bandwidthDB) SatelliteIngressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	return db.getSatelliteSummary(ctx, satelliteID, from, to, ingressFilter)
}

// getSummary returns bandwidth data for a particular satellite.
func (db *bandwidthDB) getSatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time, filter actionFilter) (_ *bandwidth.Usage, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	from, to = from.UTC(), to.UTC()

	query := `SELECT action, sum(a) amount from(
			SELECT action, sum(amount) a
				FROM bandwidth_usage
				WHERE datetime(?) <= datetime(created_at) AND datetime(created_at) <= datetime(?)
				AND satellite_id = ?
				GROUP BY action
			UNION ALL
			SELECT action, sum(amount) a
				FROM bandwidth_usage_rollups
				WHERE datetime(?) <= datetime(interval_start) AND datetime(interval_start) <= datetime(?)
				AND satellite_id = ?
				GROUP BY action
		) GROUP BY action;`

	rows, err := db.QueryContext(ctx, query, from, to, satelliteID, from, to, satelliteID)
	if err != nil {
		return nil, ErrBandwidth.Wrap(err)
	}

	defer func() {
		err = ErrBandwidth.Wrap(errs.Combine(err, rows.Close()))
	}()

	usage := new(bandwidth.Usage)
	for rows.Next() {
		var action pb.PieceAction
		var amount int64

		err := rows.Scan(&action, &amount)
		if err != nil {
			return nil, err
		}

		filter(action, amount, usage)
	}

	return usage, nil
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

// Rollup bandwidth_usage data earlier than the current hour, then delete the rolled up records.
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

// GetDailyRollups returns slice of daily bandwidth usage rollups for provided time range,
// sorted in ascending order.
func (db *bandwidthDB) GetDailyRollups(ctx context.Context, from, to time.Time) (_ []bandwidth.UsageRollup, err error) {
	defer mon.Task()(&ctx, from, to)(&err)

	since, _ := date.DayBoundary(from.UTC())
	_, before := date.DayBoundary(to.UTC())

	return db.getDailyUsageRollups(ctx,
		"WHERE DATETIME(?) <= DATETIME(interval_start) AND DATETIME(interval_start) <= DATETIME(?)",
		since, before)
}

// GetDailySatelliteRollups returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order for a particular satellite.
func (db *bandwidthDB) GetDailySatelliteRollups(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []bandwidth.UsageRollup, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)

	since, _ := date.DayBoundary(from.UTC())
	_, before := date.DayBoundary(to.UTC())

	return db.getDailyUsageRollups(ctx,
		"WHERE satellite_id = ? AND DATETIME(?) <= DATETIME(interval_start) AND DATETIME(interval_start) <= DATETIME(?)",
		satelliteID, since, before)
}

// getDailyUsageRollups returns slice of grouped by date bandwidth usage rollups
// sorted in ascending order and applied condition if any.
func (db *bandwidthDB) getDailyUsageRollups(ctx context.Context, cond string, args ...interface{}) (_ []bandwidth.UsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT action, sum(a) as amount, DATETIME(DATE(interval_start)) as date FROM (
			SELECT action, sum(amount) as a, created_at AS interval_start
				FROM bandwidth_usage
				` + cond + `
				GROUP BY interval_start, action
			UNION ALL
			SELECT action, sum(amount) as a, interval_start
				FROM bandwidth_usage_rollups
				` + cond + `
				GROUP BY interval_start, action
		) GROUP BY date, action
		ORDER BY interval_start`

	// duplicate args as they are used twice
	args = append(args, args...)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, ErrBandwidth.Wrap(err)
	}

	defer func() {
		err = ErrBandwidth.Wrap(errs.Combine(err, rows.Close()))
	}()

	var dates []time.Time
	usageRollupsByDate := make(map[time.Time]*bandwidth.UsageRollup)

	for rows.Next() {
		var action int32
		var amount int64
		var intervalStartN dbutil.NullTime

		err = rows.Scan(&action, &amount, &intervalStartN)
		if err != nil {
			return nil, err
		}

		intervalStart := intervalStartN.Time

		rollup, ok := usageRollupsByDate[intervalStart]
		if !ok {
			rollup = &bandwidth.UsageRollup{
				IntervalStart: intervalStart,
			}

			dates = append(dates, intervalStart)
			usageRollupsByDate[intervalStart] = rollup
		}

		switch pb.PieceAction(action) {
		case pb.PieceAction_GET:
			rollup.Egress.Usage = amount
		case pb.PieceAction_GET_AUDIT:
			rollup.Egress.Audit = amount
		case pb.PieceAction_GET_REPAIR:
			rollup.Egress.Repair = amount
		case pb.PieceAction_PUT:
			rollup.Ingress.Usage = amount
		case pb.PieceAction_PUT_REPAIR:
			rollup.Ingress.Repair = amount
		case pb.PieceAction_DELETE:
			rollup.Delete = amount
		}
	}

	var usageRollups []bandwidth.UsageRollup
	for _, d := range dates {
		usageRollups = append(usageRollups, *usageRollupsByDate[d])
	}

	return usageRollups, nil
}

func getBeginningOfMonth(now time.Time) time.Time {
	y, m, _ := now.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.Now().UTC().Location())
}
