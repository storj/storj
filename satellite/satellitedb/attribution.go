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

	"storj.io/common/storj"
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
func (a *attributionDB) Get(ctx context.Context, projectID uuid.UUID, bucketName []byte) (info *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := a.db.Get_ValueAttribution_By_ProjectId_And_BucketName(ctx,
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
func (a *attributionDB) UpdateUserAgent(ctx context.Context, projectID uuid.UUID, bucketName string, userAgent []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = a.db.Update_ValueAttribution_By_ProjectId_And_BucketName(ctx,
		dbx.ValueAttribution_ProjectId(projectID[:]),
		dbx.ValueAttribution_BucketName([]byte(bucketName)),
		dbx.ValueAttribution_Update_Fields{
			UserAgent: dbx.ValueAttribution_UserAgent(userAgent),
		})

	return err
}

// UpdatePlacement updates bucket placement.
func (a *attributionDB) UpdatePlacement(ctx context.Context, projectID uuid.UUID, bucketName string, placement *storj.PlacementConstraint) (err error) {
	defer mon.Task()(&ctx)(&err)

	updateFields := dbx.ValueAttribution_Update_Fields{}
	if placement == nil {
		updateFields.Placement = dbx.ValueAttribution_Placement_Null()
	} else {
		updateFields.Placement = dbx.ValueAttribution_Placement(int(*placement))
	}

	_, err = a.db.Update_ValueAttribution_By_ProjectId_And_BucketName(ctx,
		dbx.ValueAttribution_ProjectId(projectID[:]),
		dbx.ValueAttribution_BucketName([]byte(bucketName)),
		updateFields)

	return err
}

// TestDelete is used for testing purposes to delete all attribution data for a given project and bucket.
func (a *attributionDB) TestDelete(ctx context.Context, projectID uuid.UUID, bucketName []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = a.db.Delete_ValueAttribution_By_ProjectId_And_BucketName(ctx,
		dbx.ValueAttribution_ProjectId(projectID[:]),
		dbx.ValueAttribution_BucketName(bucketName))

	return err
}

// Insert implements create partner info.
func (a *attributionDB) Insert(ctx context.Context, info *attribution.Info) (_ *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	switch a.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		err = a.db.QueryRowContext(ctx, `
				INSERT INTO value_attributions (project_id, bucket_name, user_agent, placement, last_updated) 
				VALUES ($1, $2, $3, $4, now())
				ON CONFLICT (project_id, bucket_name) DO NOTHING
				RETURNING last_updated`, info.ProjectID[:], info.BucketName, info.UserAgent, info.Placement).Scan(&info.CreatedAt)
		// TODO when sql.ErrNoRows is returned then CreatedAt is not set
		if errors.Is(err, sql.ErrNoRows) {
			return info, nil
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}
	case dbutil.Spanner:
		err := a.db.QueryRowContext(ctx, `
			INSERT OR IGNORE INTO value_attributions (project_id, bucket_name, user_agent, placement, last_updated)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP())
			THEN RETURN last_updated`, info.ProjectID[:], info.BucketName, info.UserAgent, info.Placement).Scan(&info.CreatedAt)
		// TODO when sql.ErrNoRows is returned then CreatedAt is not set
		if errors.Is(err, sql.ErrNoRows) {
			return info, nil
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}

	default:
		return nil, errs.New("unsupported database dialect: %s", a.db.impl)
	}

	return info, nil
}

// QueryAttribution queries partner bucket attribution data.
func (a *attributionDB) QueryAttribution(ctx context.Context, userAgent []byte, start time.Time, end time.Time) (_ []*attribution.BucketUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string
	var args []interface{}
	switch a.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		query = valueAttrCockroachQuery
		args = append(args, userAgent, start.UTC(), end.UTC())
	case dbutil.Spanner:
		query = valueAttrSpannerQuery
		args = append(args, sql.Named("user_agent", userAgent))
		args = append(args, sql.Named("start", start.UTC()))
		args = append(args, sql.Named("end", end.UTC()))
	default:
		return nil, errs.New("unsupported database dialect: %s", a.db.impl)
	}

	rows, err := a.db.DB.QueryContext(ctx, a.db.Rebind(query), args...)
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
func (a *attributionDB) QueryAllAttribution(ctx context.Context, start time.Time, end time.Time) (_ []*attribution.BucketUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string
	var args []interface{}
	switch a.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		query = allValueAttrPsqlQuery
		args = append(args, start.UTC(), end.UTC())
	case dbutil.Spanner:
		query = allValueAttrSpannerQuery
		args = append(args, sql.Named("start", start.UTC()))
		args = append(args, sql.Named("end", end.UTC()))
	default:
		return nil, errs.New("unsupported database dialect: %s", a.db.impl)
	}

	rows, err := a.db.DB.QueryContext(ctx, a.db.Rebind(query), args...)
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

// BackfillPlacementBatch updates up to batchSize rows of value_attributions.placement from bucket_metainfos.
// It returns:
//
//	rowsProcessed = number of rows updated in this batch
//	hasNext       = true if there may be more batches to run
func (a *attributionDB) BackfillPlacementBatch(
	ctx context.Context,
	batchSize int,
) (rowsProcessed int64, hasNext bool, err error) {
	defer mon.Task()(&ctx)(&err)

	switch a.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		res, err := a.db.ExecContext(ctx, `
		WITH to_update AS (
			SELECT va.project_id, va.bucket_name
			FROM value_attributions AS va
			JOIN bucket_metainfos   AS bm
				ON va.project_id = bm.project_id
				AND va.bucket_name = bm.name
			WHERE va.placement IS NULL
				AND bm.placement IS NOT NULL
			ORDER BY va.project_id, va.bucket_name
			LIMIT $1
		)
		UPDATE value_attributions AS va
		SET placement = bm.placement
		FROM to_update AS u
		JOIN bucket_metainfos AS bm
			ON bm.project_id = u.project_id
			AND bm.name = u.bucket_name
		WHERE va.project_id = u.project_id
			AND va.bucket_name = u.bucket_name
			AND bm.placement IS NOT NULL;
		`, batchSize)
		if err != nil {
			return 0, false, Error.Wrap(err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return 0, false, Error.New("could not get rows affected: %w", err)
		}

		return n, n == int64(batchSize), nil
	case dbutil.Spanner:
		res, err := a.db.ExecContext(ctx, `
		UPDATE value_attributions AS va
		SET placement = (
		SELECT bm.placement
		FROM bucket_metainfos AS bm
		WHERE bm.project_id = va.project_id
			AND bm.name = va.bucket_name
			AND bm.placement IS NOT NULL
		)
		WHERE STRUCT<project_id BYTES, bucket_name BYTES>(va.project_id, va.bucket_name)
			IN UNNEST(
				ARRAY(
					SELECT AS STRUCT va2.project_id, va2.bucket_name
					FROM value_attributions AS va2
					JOIN bucket_metainfos AS bm2
						ON va2.project_id = bm2.project_id
						AND va2.bucket_name = bm2.name
					WHERE va2.placement IS NULL
						AND bm2.placement IS NOT NULL
			    	ORDER BY va2.project_id, va2.bucket_name
			    	LIMIT ?
				)
			);
		`, batchSize)
		if err != nil {
			return 0, false, Error.Wrap(err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return 0, false, Error.New("could not get rows affected: %w", err)
		}

		return n, n == int64(batchSize), nil
	default:
		return 0, false, errs.New("unsupported database dialect: %s", a.db.impl)
	}
}

func attributionFromDBX(info *dbx.ValueAttribution) (*attribution.Info, error) {
	userAgent := info.UserAgent
	projectID, err := uuid.FromBytes(info.ProjectId)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	var placementPtr *storj.PlacementConstraint
	if info.Placement != nil {
		placementVal := storj.PlacementConstraint(*info.Placement)
		placementPtr = &placementVal
	}
	return &attribution.Info{
		ProjectID:  projectID,
		BucketName: info.BucketName,
		UserAgent:  userAgent,
		Placement:  placementPtr,
		CreatedAt:  info.LastUpdated,
	}, nil
}
