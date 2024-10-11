// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
)

// I can see how keeping around the query to only get attribution for one partner might be good,
// but that means we need to maintain both queries. Maybe we should get rid of this one.
//
//go:embed attribution_value_psql.sql
var valueAttrCockroachQuery string

//go:embed attribution_value_spanner.sql
var valueAttrSpannerQuery string

//go:embed attribution_all_value_psql.sql
var allValueAttrPsqlQuery string

//go:embed attribution_all_value_spanner.sql
var allValueAttrSpannerQuery string

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

	switch keys.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		err = keys.db.QueryRowContext(ctx, `
				INSERT INTO value_attributions (project_id, bucket_name, user_agent, last_updated) 
				VALUES ($1, $2, $3, now())
				ON CONFLICT (project_id, bucket_name) DO NOTHING
				RETURNING last_updated`, info.ProjectID[:], info.BucketName, info.UserAgent).Scan(&info.CreatedAt)
		// TODO when sql.ErrNoRows is returned then CreatedAt is not set
		if errors.Is(err, sql.ErrNoRows) {
			return info, nil
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}
	case dbutil.Spanner:
		err := keys.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			return tx.QueryRowContext(ctx, `
				INSERT OR IGNORE INTO value_attributions (project_id, bucket_name, user_agent, last_updated)
				VALUES (?, ?, ?, CURRENT_TIMESTAMP())
				THEN RETURN last_updated`, info.ProjectID[:], info.BucketName, info.UserAgent).Scan(&info.CreatedAt)
			// TODO when sql.ErrNoRows is returned then CreatedAt is not set
		})
		if errors.Is(err, sql.ErrNoRows) {
			return info, nil
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}

	default:
		return nil, errs.New("unsupported database dialect: %s", keys.db.impl)
	}

	return info, nil
}

// QueryAttribution queries partner bucket attribution data.
func (keys *attributionDB) QueryAttribution(ctx context.Context, userAgent []byte, start time.Time, end time.Time) (_ []*attribution.BucketUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string
	var args []interface{}
	switch keys.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		query = valueAttrCockroachQuery
		args = append(args, userAgent, start.UTC(), end.UTC())
	case dbutil.Spanner:
		query = valueAttrSpannerQuery
		args = append(args, sql.Named("user_agent", userAgent))
		args = append(args, sql.Named("start", start.UTC()))
		args = append(args, sql.Named("end", end.UTC()))
	default:
		return nil, errs.New("unsupported database dialect: %s", keys.db.impl)
	}

	rows, err := keys.db.DB.QueryContext(ctx, keys.db.Rebind(query), args...)
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

	var query string
	var args []interface{}
	switch keys.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		query = allValueAttrPsqlQuery
		args = append(args, start.UTC(), end.UTC())
	case dbutil.Spanner:
		query = allValueAttrSpannerQuery
		args = append(args, sql.Named("start", start.UTC()))
		args = append(args, sql.Named("end", end.UTC()))
	default:
		return nil, errs.New("unsupported database dialect: %s", keys.db.impl)
	}

	rows, err := keys.db.DB.QueryContext(ctx, keys.db.Rebind(query), args...)
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
