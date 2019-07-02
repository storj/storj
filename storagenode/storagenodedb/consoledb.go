// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/console"
)

type consoledb struct{ *InfoDB }

// Console returns console.DB
func (db *InfoDB) Console() console.DB { return &consoledb{db} }

// Console returns console.DB
func (db *DB) Console() console.DB { return db.info.Console() }

// GetSatelliteIDs returns list of satelliteIDs that storagenode has interacted with
// at least once
func (db *consoledb) GetSatelliteIDs(ctx context.Context, from, to time.Time) (_ storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	var satellites storj.NodeIDList

	rows, err := db.db.QueryContext(ctx, db.Rebind(`
		SELECT DISTINCT satellite_id
		FROM bandwidth_usage
		WHERE ? <= created_at AND created_at <= ?`), from, to)

	if err != nil {
		if err == sql.ErrNoRows {
			return satellites, nil
		}
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var satelliteID storj.NodeID
		if err = rows.Scan(&satelliteID); err != nil {
			return nil, err
		}

		satellites = append(satellites, satelliteID)
	}

	return satellites, nil
}

// GetDailyBandwidthUsed returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order
func (db *consoledb) GetDailyTotalBandwidthUsed(ctx context.Context, from, to time.Time) (_ []console.BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	since, _ := getDateEdges(from.UTC())
	_, before := getDateEdges(to.UTC())

	return db.getDailyBandwidthUsed(ctx,
		"WHERE ? <= created_at AND created_at <= ?",
		since, before)
}

// GetDailyBandwidthUsed returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order for particular satellite
func (db *consoledb) GetDailyBandwidthUsed(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []console.BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	since, _ := getDateEdges(from.UTC())
	_, before := getDateEdges(to.UTC())

	return db.getDailyBandwidthUsed(ctx,
		"WHERE satellite_id = ? AND ? <= created_at AND created_at <= ?",
		satelliteID, since, before)
}

// getDailyBandwidthUsed returns slice of grouped by date bandwidth usage
// sorted in ascending order and applied condition if any
func (db *consoledb) getDailyBandwidthUsed(ctx context.Context, cond string, args ...interface{}) (_ []console.BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	qb := strings.Builder{}
	qb.WriteString("SELECT action, SUM(amount), created_at ")
	qb.WriteString("FROM bandwidth_usage ")
	if cond != "" {
		qb.WriteString(cond + " ")
	}
	qb.WriteString("GROUP BY DATE(created_at), action ")
	qb.WriteString("ORDER BY created_at ASC")

	rows, err := db.db.QueryContext(ctx, db.Rebind(qb.String()), args)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var dates []time.Time
	dailyBandwidth := make(map[time.Time]*console.BandwidthUsed, 0)

	for rows.Next() {
		var action int32
		var amount int64
		var createdAt time.Time

		err = rows.Scan(&action, &amount, &createdAt)
		if err != nil {
			return nil, err
		}

		from, to := getDateEdges(createdAt)

		bandwidthUsed, ok := dailyBandwidth[from]
		if !ok {
			bandwidthUsed = &console.BandwidthUsed{
				From: from,
				To:   to,
			}

			dates = append(dates, from)
			dailyBandwidth[from] = bandwidthUsed
		}

		switch pb.PieceAction(action) {
		case pb.PieceAction_GET:
			bandwidthUsed.Egress.Usage = amount
		case pb.PieceAction_GET_AUDIT:
			bandwidthUsed.Egress.Audit = amount
		case pb.PieceAction_GET_REPAIR:
			bandwidthUsed.Egress.Repair = amount
		case pb.PieceAction_PUT:
			bandwidthUsed.Ingress.Usage = amount
		case pb.PieceAction_PUT_REPAIR:
			bandwidthUsed.Ingress.Repair = amount
		}
	}

	var bandwidthUsedList []console.BandwidthUsed
	for _, date := range dates {
		bandwidthUsedList = append(bandwidthUsedList, *dailyBandwidth[date])
	}

	return bandwidthUsedList, nil
}

// getDateEdges returns start and end of the provided day
func getDateEdges(t time.Time) (time.Time, time.Time) {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, -1, time.UTC)
}
