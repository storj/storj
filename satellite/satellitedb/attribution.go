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
	// I can see how keeping around the query to only get attribution for one partner might be good,
	// but that means we need to maintain both queries. Maybe we should get rid of this one.
	valueAttrQuery = `
	-- A union of both the storage tally and bandwidth rollups.
	-- Should be 1 row per project/bucket by partner within the timeframe specified
	SELECT 
		o.user_agent as user_agent, 
		o.project_id as project_id, 
		o.bucket_name as bucket_name, 
		SUM(o.total) as total_byte_hours,
		SUM(o.remote) as remote_byte_hours,
		SUM(o.inline) as inline_byte_hours,
		SUM(o.segments) as segment_hours,
		SUM(o.objects) as object_hours,
		SUM(o.settled) as settled, 
		COALESCE(SUM(o.hours),0) as hours
	FROM 
		(
			-- SUM the storage and hours
			-- Hours are used to calculate byte hours above
			SELECT 
				bsti.user_agent as user_agent, 
				bsto.project_id as project_id, 
				bsto.bucket_name as bucket_name,
				SUM(bsto.total_bytes) as total,
				SUM(bsto.remote) as remote, 
				SUM(bsto.inline) as inline, 
				SUM(bsto.total_segments_count) as segments,
				SUM(bsto.object_count) as objects,
				0 as settled, 
				count(1) as hours 
			FROM 
				(
					-- Collapse entries by the latest record in the hour
					-- If there are more than 1 records within the hour, only the latest will be considered
					SELECT 
						va.user_agent, 
						date_trunc('hour', bst.interval_start) as hours,
						bst.project_id, 
						bst.bucket_name, 
						MAX(bst.interval_start) as max_interval 
					FROM 
						bucket_storage_tallies bst 
						RIGHT JOIN value_attributions va ON (
							bst.project_id = va.project_id 
							AND bst.bucket_name = va.bucket_name
						) 
					WHERE 
						va.user_agent = ? 
						AND bst.interval_start >= ? 
						AND bst.interval_start < ? 
					GROUP BY 
						va.user_agent, 
						bst.project_id, 
						bst.bucket_name, 
						date_trunc('hour', bst.interval_start) 
					ORDER BY 
						max_interval DESC
				) bsti 
				INNER JOIN bucket_storage_tallies bsto ON (
					bsto.project_id = bsti.project_id 
					AND bsto.bucket_name = bsti.bucket_name 
					AND bsto.interval_start = bsti.max_interval
				) 
			GROUP BY 
				bsti.user_agent, 
				bsto.project_id, 
				bsto.bucket_name 
			UNION 
			-- SUM the bandwidth for the timeframe specified grouping by the user_agent, project_id, and bucket_name
			SELECT 
				va.user_agent as user_agent, 
				bbr.project_id as project_id, 
				bbr.bucket_name as bucket_name, 
				0 as total,
				0 as remote, 
				0 as inline, 
				0 as segments,
				0 as objects,
				SUM(settled)::integer as settled, 
				NULL as hours 
			FROM 
				bucket_bandwidth_rollups bbr 
				INNER JOIN value_attributions va ON (
					bbr.project_id = va.project_id 
					AND bbr.bucket_name = va.bucket_name
				) 
			WHERE 
				va.user_agent = ? 
				AND bbr.interval_start >= ? 
				AND bbr.interval_start < ? 
				-- action 2 is GET
				AND bbr.action = 2 
			GROUP BY 
				va.user_agent, 
				bbr.project_id, 
				bbr.bucket_name
		) AS o 
	GROUP BY 
		o.user_agent, 
		o.project_id, 
		o.bucket_name;
	`
	allValueAttrQuery = `
	-- A union of both the storage tally and bandwidth rollups.
	-- Should be 1 row per project/bucket by partner within the timeframe specified
	SELECT
		o.user_agent as user_agent,
		o.project_id as project_id,
		o.bucket_name as bucket_name,
		SUM(o.total) as total_byte_hours,
		SUM(o.remote) as remote_byte_hours,
		SUM(o.inline) as inline_byte_hours,
		SUM(o.segments) as segment_hours,
		SUM(o.objects) as object_hours,
		SUM(o.settled) as settled,
		COALESCE(SUM(o.hours),0) as hours
	FROM
		(
			-- SUM the storage and hours
			-- Hours are used to calculate byte hours above
			SELECT
				bsti.user_agent as user_agent,
				bsto.project_id as project_id,
				bsto.bucket_name as bucket_name,
				SUM(bsto.total_bytes) as total,
				SUM(bsto.remote) as remote,
				SUM(bsto.inline) as inline,
				SUM(bsto.total_segments_count) as segments,
				SUM(bsto.object_count) as objects,
				0 as settled,
				count(1) as hours
			FROM
				(
					-- Collapse entries by the latest record in the hour
					-- If there are more than 1 records within the hour, only the latest will be considered
					SELECT
						va.user_agent,
						date_trunc('hour', bst.interval_start) as hours,
						bst.project_id,
						bst.bucket_name,
						MAX(bst.interval_start) as max_interval
					FROM
						bucket_storage_tallies bst
						INNER JOIN value_attributions va ON (
							bst.project_id = va.project_id
							AND bst.bucket_name = va.bucket_name
						)
					WHERE
						bst.interval_start >= $1
						AND bst.interval_start < $2
					GROUP BY
						va.user_agent,
						bst.project_id,
						bst.bucket_name,
						date_trunc('hour', bst.interval_start)
					ORDER BY
						max_interval DESC
				) bsti
				INNER JOIN bucket_storage_tallies bsto ON (
					bsto.project_id = bsti.project_id
					AND bsto.bucket_name = bsti.bucket_name
					AND bsto.interval_start = bsti.max_interval
				)
			GROUP BY
				bsti.user_agent,
				bsto.project_id,
				bsto.bucket_name
			UNION
			-- SUM the bandwidth for the timeframe specified grouping by the user_agent, project_id, and bucket_name
			SELECT
				va.user_agent as user_agent,
				bbr.project_id as project_id,
				bbr.bucket_name as bucket_name,
				0 as total,
				0 as remote,
				0 as inline,
				0 as segments,
				0 as objects,
				SUM(settled)::integer as settled,
				null as hours
			FROM
				bucket_bandwidth_rollups bbr
				INNER JOIN value_attributions va ON (
					bbr.project_id = va.project_id
					AND bbr.bucket_name = va.bucket_name
				)
			WHERE
				bbr.interval_start >= $1
				AND bbr.interval_start < $2
				-- action 2 is GET
				AND bbr.action = 2
			GROUP BY
				va.user_agent,
				bbr.project_id,
				bbr.bucket_name
		) AS o
	GROUP BY
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

// UpdateUserAgent updates bucket attribution data.
func (keys *attributionDB) UpdateUserAgent(ctx context.Context, projectID uuid.UUID, bucketName string, userAgent []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = keys.db.Update_ValueAttribution_By_ProjectId_And_BucketName(ctx,
		dbx.ValueAttribution_ProjectId(projectID[:]),
		dbx.ValueAttribution_BucketName([]byte(bucketName)),
		dbx.ValueAttribution_Update_Fields{
			UserAgent: dbx.ValueAttribution_UserAgent(userAgent),
		})

	return err
}

// Insert implements create partner info.
func (keys *attributionDB) Insert(ctx context.Context, info *attribution.Info) (_ *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	err = keys.db.QueryRowContext(ctx, `
		INSERT INTO value_attributions (project_id, bucket_name, user_agent, last_updated) 
		VALUES ($1, $2, $3, now())
		ON CONFLICT (project_id, bucket_name) DO NOTHING
		RETURNING last_updated
	`, info.ProjectID[:], info.BucketName, info.UserAgent).Scan(&info.CreatedAt)
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
func (keys *attributionDB) QueryAttribution(ctx context.Context, userAgent []byte, start time.Time, end time.Time) (_ []*attribution.BucketUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := keys.db.DB.QueryContext(ctx, keys.db.Rebind(valueAttrQuery), userAgent, start.UTC(), end.UTC(), userAgent, start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	results := []*attribution.BucketUsage{}
	for rows.Next() {
		r := &attribution.BucketUsage{}
		var inline, remote float64
		err := rows.Scan(&r.UserAgent, &r.ProjectID, &r.BucketName, &r.ByteHours, &inline, &remote, &r.SegmentHours, &r.ObjectHours, &r.EgressData, &r.Hours)
		if err != nil {
			return results, Error.Wrap(err)
		}

		if r.ByteHours == 0 {
			r.ByteHours = inline + remote
		}

		results = append(results, r)
	}
	return results, Error.Wrap(rows.Err())
}

// QueryAllAttribution queries all partner bucket attribution data.
func (keys *attributionDB) QueryAllAttribution(ctx context.Context, start time.Time, end time.Time) (_ []*attribution.BucketUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := keys.db.DB.QueryContext(ctx, keys.db.Rebind(allValueAttrQuery), start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	results := []*attribution.BucketUsage{}
	for rows.Next() {
		r := &attribution.BucketUsage{}
		var inline, remote float64
		err := rows.Scan(&r.UserAgent, &r.ProjectID, &r.BucketName, &r.ByteHours, &inline, &remote, &r.SegmentHours, &r.ObjectHours, &r.EgressData, &r.Hours)
		if err != nil {
			return results, Error.Wrap(err)
		}

		if r.ByteHours == 0 {
			r.ByteHours = inline + remote
		}
		results = append(results, r)
	}
	return results, Error.Wrap(rows.Err())
}

func attributionFromDBX(info *dbx.ValueAttribution) (*attribution.Info, error) {
	userAgent := info.UserAgent
	projectID, err := uuid.FromBytes(info.ProjectId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &attribution.Info{
		ProjectID:  projectID,
		BucketName: info.BucketName,
		UserAgent:  userAgent,
		CreatedAt:  info.LastUpdated,
	}, nil
}
