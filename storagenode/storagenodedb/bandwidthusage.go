// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type bandwidthusage struct {
	*infodb
}

// BandwidthUsage returns table for storing bandwidth usage.
func (db *infodb) BandwidthUsage() bandwidthusage { return bandwidthusage{db} }

// Add adds bandwidth usage to the table
func (db *bandwidthusage) Add(ctx context.Context, satelliteID storj.NodeID, action pb.Action, amount int64, created time.Time) error {
	defer db.locked()()

	_, err := db.db.Exec(`
		INSERT INTO 
			bandwidth_usage(satellite_id, action, amount, created_at)
		VALUES(?, ?, ?, ?)`, satelliteID, action, amount, created)

	return ErrInfo.Wrap(err)
}

// Bandwidth usage information
// TODO: move to a better place
type BandwidthUsage struct {
	Invalid int64
	Unknown int64

	Put       int64
	Get       int64
	GetAudit  int64
	GetRepair int64
	PutRepair int64
	Delete    int64
}

// Include adds specified action to the appropriate field.
func (usage *BandwidthUsage) Include(action pb.Action, amount int64) {
	switch action {
	case pb.Action_INVALID:
		usage.Invalid += amount
	case pb.Action_PUT:
		usage.Put += amount
	case pb.Action_GET:
		usage.Get += amount
	case pb.Action_GET_AUDIT:
		usage.GetAudit += amount
	case pb.Action_GET_REPAIR:
		usage.GetRepair += amount
	case pb.Action_PUT_REPAIR:
		usage.PutRepair += amount
	case pb.Action_DELETE:
		usage.Delete += amount
	default:
		usage.Unknown += amount
	}
}

// Summary returns summary of bandwidth usages
func (db *bandwidthusage) Summary(ctx context.Context, from, to time.Time) (*BandwidthUsage, error) {
	defer db.locked()()

	usage := &BandwidthUsage{}

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

	for rows.Next() {
		var action pb.Action
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
func (db *bandwidthusage) SummaryBySatellite(ctx context.Context, from, to time.Time) (map[storj.NodeID]*BandwidthUsage, error) {
	defer db.locked()()

	entries := map[storj.NodeID]*BandwidthUsage{}

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

	for rows.Next() {
		var satelliteID storj.NodeID
		var action pb.Action
		var amount int64

		err := rows.Scan(&satelliteID, &action, &amount)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		entry, ok := entries[satelliteID]
		if !ok {
			entry = &BandwidthUsage{}
			entries[satelliteID] = entry
		}

		entry.Include(action, amount)
	}

	return entries, ErrInfo.Wrap(rows.Err())
}
