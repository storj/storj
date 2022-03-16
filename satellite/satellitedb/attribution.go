// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb/dbx"
)

const (
	valueAttrQuery = `
	-- A union of both the storage tally and bandwidth rollups.
	-- Should be 1 row per project/bucket by partner within the timeframe specified
	SELECT 
		o.partner_id as partner_id, 
		o.user_agent as user_agent, 
		o.project_id as project_id, 
		o.bucket_name as bucket_name, 
		SUM(o.total)  / SUM(o.hours) as total,
		SUM(o.remote) / SUM(o.hours) as remote,
		SUM(o.inline) / SUM(o.hours) as inline,
		SUM(o.settled) as settled 
	FROM 
		(
			-- SUM the storage and hours
			-- Hours are used to calculate byte hours above
			SELECT 
				bsti.partner_id as partner_id, 
				bsti.user_agent as user_agent, 
				bsto.project_id as project_id, 
				bsto.bucket_name as bucket_name,
				SUM(bsto.total_bytes) as total,
				SUM(bsto.remote) as remote, 
				SUM(bsto.inline) as inline, 
				0 as settled, 
				count(1) as hours 
			FROM 
				(
					-- Collapse entries by the latest record in the hour
					-- If there are more than 1 records within the hour, only the latest will be considered
					SELECT 
						va.partner_id, 
						va.user_agent, 
						date_trunc('hour', bst.interval_start) as hours,
						bst.project_id, 
						bst.bucket_name, 
						MAX(bst.interval_start) as max_interval 
					FROM 
						bucket_storage_tallies bst 
						LEFT OUTER JOIN value_attributions va ON (
							bst.project_id = va.project_id 
							AND bst.bucket_name = va.bucket_name
						) 
					WHERE 
						va.partner_id = ? 
						AND va.user_agent = ? 
						AND bst.interval_start >= ? 
						AND bst.interval_start < ? 
					GROUP BY 
						va.partner_id, 
						va.user_agent, 
						bst.project_id, 
						bst.bucket_name, 
						date_trunc('hour', bst.interval_start) 
					ORDER BY 
						max_interval DESC
				) bsti 
				LEFT JOIN bucket_storage_tallies bsto ON (
					bsto.project_id = bsti.project_id 
					AND bsto.bucket_name = bsti.bucket_name 
					AND bsto.interval_start = bsti.max_interval
				) 
			GROUP BY 
				bsti.partner_id, 
				bsti.user_agent, 
				bsto.project_id, 
				bsto.bucket_name 
			UNION 
			-- SUM the bandwidth for the timeframe specified grouping by the partner_id, user_agent, project_id, and bucket_name
			SELECT 
				va.partner_id as partner_id, 
				va.user_agent as user_agent, 
				bbr.project_id as project_id, 
				bbr.bucket_name as bucket_name, 
				0 as total,
				0 as remote, 
				0 as inline, 
				SUM(settled)::integer as settled, 
				NULL as hours 
			FROM 
				bucket_bandwidth_rollups bbr 
				LEFT OUTER JOIN value_attributions va ON (
					bbr.project_id = va.project_id 
					AND bbr.bucket_name = va.bucket_name
				) 
			WHERE 
				va.partner_id = ? 
				AND va.user_agent = ? 
				AND bbr.interval_start >= ? 
				AND bbr.interval_start < ? 
				-- action 2 is GET
				AND bbr.action = 2 
			GROUP BY 
				va.partner_id, 
				va.user_agent, 
				bbr.project_id, 
				bbr.bucket_name
		) AS o 
	GROUP BY 
		o.partner_id, 
		o.user_agent, 
		o.project_id, 
		o.bucket_name;
	`
	allValueAttrQuery = `
	-- A union of both the storage tally and bandwidth rollups.
	-- Should be 1 row per project/bucket by partner within the timeframe specified
	SELECT
		o.partner_id as partner_id,
		o.user_agent as user_agent,
		o.project_id as project_id,
		o.bucket_name as bucket_name,
		SUM(o.total)  / SUM(o.hours) as total,
		SUM(o.remote) / SUM(o.hours) as remote,
		SUM(o.inline) / SUM(o.hours) as inline,
		SUM(o.settled) as settled
	FROM
		(
			-- SUM the storage and hours
			-- Hours are used to calculate byte hours above
			SELECT
				bsti.partner_id as partner_id,
				bsti.user_agent as user_agent,
				bsto.project_id as project_id,
				bsto.bucket_name as bucket_name,
				SUM(bsto.total_bytes) as total,
				SUM(bsto.remote) as remote,
				SUM(bsto.inline) as inline,
				0 as settled,
				count(1) as hours
			FROM
				(
					-- Collapse entries by the latest record in the hour
					-- If there are more than 1 records within the hour, only the latest will be considered
					SELECT
						va.partner_id,
						va.user_agent,
						date_trunc('hour', bst.interval_start) as hours,
						bst.project_id,
						bst.bucket_name,
						MAX(bst.interval_start) as max_interval
					FROM
						bucket_storage_tallies bst
						LEFT OUTER JOIN value_attributions va ON (
							bst.project_id = va.project_id
							AND bst.bucket_name = va.bucket_name
						)
					WHERE
						bst.interval_start >= $1
						AND bst.interval_start < $2
					GROUP BY
						va.partner_id,
						va.user_agent,
						bst.project_id,
						bst.bucket_name,
						date_trunc('hour', bst.interval_start)
					ORDER BY
						max_interval DESC
				) bsti
				LEFT JOIN bucket_storage_tallies bsto ON (
					bsto.project_id = bsti.project_id
					AND bsto.bucket_name = bsti.bucket_name
					AND bsto.interval_start = bsti.max_interval
				)
			GROUP BY
				bsti.partner_id,
				bsti.user_agent,
				bsto.project_id,
				bsto.bucket_name
			UNION
			-- SUM the bandwidth for the timeframe specified grouping by the partner_id, user_agent, project_id, and bucket_name
			SELECT
				va.partner_id as partner_id,
				va.user_agent as user_agent,
				bbr.project_id as project_id,
				bbr.bucket_name as bucket_name,
				0 as total,
				0 as remote,
				0 as inline,
				SUM(settled)::integer as settled,
				NULL as hours
			FROM
				bucket_bandwidth_rollups bbr
				LEFT OUTER JOIN value_attributions va ON (
					bbr.project_id = va.project_id
					AND bbr.bucket_name = va.bucket_name
				)
			WHERE
				bbr.interval_start >= $1
				AND bbr.interval_start < $2
				-- action 2 is GET
				AND bbr.action = 2
			GROUP BY
				va.partner_id,
				va.user_agent,
				bbr.project_id,
				bbr.bucket_name
		) AS o
	GROUP BY
		o.partner_id,
		o.user_agent,
		o.project_id,
		o.bucket_name;
	`
)

type attributionDB struct {
	db *satelliteDB
}

// Get reads the partner info.
func (keys *attributionDB) Get(ctx context.Context, projectID uuid.UUID, bucketName []byte) (info *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Get_ValueAttribution_By_ProjectId_And_BucketName(ctx,
		dbx.ValueAttribution_ProjectId(projectID[:]),
		dbx.ValueAttribution_BucketName(bucketName),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, attribution.ErrBucketNotAttributed.New("%q", bucketName)
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return attributionFromDBX(dbxInfo)
}

// Insert implements create partner info.
func (keys *attributionDB) Insert(ctx context.Context, info *attribution.Info) (_ *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	err = keys.db.QueryRowContext(ctx, `
		INSERT INTO value_attributions (project_id, bucket_name, partner_id, user_agent, last_updated) 
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (project_id, bucket_name) DO NOTHING
		RETURNING last_updated
	`, info.ProjectID[:], info.BucketName, info.PartnerID[:], info.UserAgent).Scan(&info.CreatedAt)
	// TODO when sql.ErrNoRows is returned then CreatedAt is not set
	if errors.Is(err, sql.ErrNoRows) {
		return info, nil
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return info, nil
}

// QueryAttribution queries partner bucket attribution data.
func (keys *attributionDB) QueryAttribution(ctx context.Context, partnerID uuid.UUID, userAgent []byte, start time.Time, end time.Time) (_ []*attribution.CSVRow, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := keys.db.DB.QueryContext(ctx, keys.db.Rebind(valueAttrQuery), partnerID[:], userAgent, start.UTC(), end.UTC(), partnerID[:], userAgent, start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	results := []*attribution.CSVRow{}
	for rows.Next() {
		r := &attribution.CSVRow{}
		var inline, remote float64
		err := rows.Scan(&r.PartnerID, &r.UserAgent, &r.ProjectID, &r.BucketName, &r.TotalBytesPerHour, &inline, &remote, &r.EgressData)
		if err != nil {
			return results, Error.Wrap(err)
		}

		if r.TotalBytesPerHour == 0 {
			r.TotalBytesPerHour = inline + remote
		}

		results = append(results, r)
	}
	return results, Error.Wrap(rows.Err())
}

// QueryAllAttribution queries all partner bucket attribution data.
func (keys *attributionDB) QueryAllAttribution(ctx context.Context, start time.Time, end time.Time) (_ []*attribution.CSVRow, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := keys.db.DB.QueryContext(ctx, keys.db.Rebind(allValueAttrQuery), start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	results := []*attribution.CSVRow{}
	for rows.Next() {
		r := &attribution.CSVRow{}
		var inline, remote float64
		err := rows.Scan(&r.PartnerID, &r.UserAgent, &r.ProjectID, &r.BucketName, &r.TotalBytesPerHour, &inline, &remote, &r.EgressData)
		if err != nil {
			return results, Error.Wrap(err)
		}

		if r.TotalBytesPerHour == 0 {
			r.TotalBytesPerHour = inline + remote
		}

		results = append(results, r)
	}
	return results, Error.Wrap(rows.Err())
}

func attributionFromDBX(info *dbx.ValueAttribution) (*attribution.Info, error) {
	partnerID, err := uuid.FromBytes(info.PartnerId)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	userAgent := info.UserAgent
	if err != nil {
		return nil, Error.Wrap(err)
	}
	projectID, err := uuid.FromBytes(info.ProjectId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &attribution.Info{
		ProjectID:  projectID,
		BucketName: info.BucketName,
		PartnerID:  partnerID,
		UserAgent:  userAgent,
		CreatedAt:  info.LastUpdated,
	}, nil
}
