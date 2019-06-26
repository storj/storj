// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/attribution"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

const (
	valueAttrQuery = `
	-- A union of both the storage tally and bandwidth rollups.
	-- Should be 1 row per project/bucket by partner within the timeframe specified
	SELECT 
		o.partner_id as partner_id, 
		o.project_id as project_id, 
		o.bucket_name as bucket_name, 
		SUM(o.remote) / SUM(o.hours) as remote,
		SUM(o.inline) / SUM(o.hours) as inline,
		SUM(o.settled) as settled 
	FROM 
		(
			-- SUM the storage and hours
			-- Hours are used to calculate byte hours above
			SELECT 
				bsti.partner_id as partner_id, 
				bsto.project_id as project_id, 
				bsto.bucket_name as bucket_name, 
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
						%v as hours, 
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
						AND bst.interval_start >= ? 
						AND bst.interval_start < ? 
					GROUP BY 
						va.partner_id, 
						bst.project_id, 
						bst.bucket_name, 
						hours 
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
				bsto.project_id, 
				bsto.bucket_name 
			UNION 
			-- SUM the bandwidth for the timeframe specified grouping by the partner_id, project_id, and bucket_name
			SELECT 
				va.partner_id as partner_id, 
				bbr.project_id as project_id, 
				bbr.bucket_name as bucket_name, 
				0 as remote, 
				0 as inline, 
				SUM(settled) as settled, 
				NULL as hours 
			FROM 
				bucket_bandwidth_rollups bbr 
				LEFT OUTER JOIN value_attributions va ON (
					bbr.project_id = va.project_id 
					AND bbr.bucket_name = va.bucket_name
				) 
			WHERE 
				va.partner_id = ? 
				AND bbr.interval_start >= ? 
				AND bbr.interval_start < ? 
				AND bbr.action = 2 
			GROUP BY 
				va.partner_id, 
				bbr.project_id, 
				bbr.bucket_name
		) AS o 
	GROUP BY 
		o.partner_id, 
		o.project_id, 
		o.bucket_name;
	`
	// DB specific date/time truncations
	slHour = "datetime(strftime('%Y-%m-%dT%H:00:00', bst.interval_start))"
	pqHour = "date_trunc('hour', bst.interval_start)"
)

type attributionDB struct {
	db *dbx.DB
}

// Get reads the partner info
func (keys *attributionDB) Get(ctx context.Context, projectID uuid.UUID, bucketName []byte) (info *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Get_ValueAttribution_By_ProjectId_And_BucketName(ctx,
		dbx.ValueAttribution_ProjectId(projectID[:]),
		dbx.ValueAttribution_BucketName(bucketName),
	)
	if err == sql.ErrNoRows {
		return nil, attribution.ErrBucketNotAttributed.New(string(bucketName))
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return attributionFromDBX(dbxInfo)
}

// Insert implements create partner info
func (keys *attributionDB) Insert(ctx context.Context, info *attribution.Info) (_ *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Create_ValueAttribution(ctx,
		dbx.ValueAttribution_ProjectId(info.ProjectID[:]),
		dbx.ValueAttribution_BucketName(info.BucketName),
		dbx.ValueAttribution_PartnerId(info.PartnerID[:]),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return attributionFromDBX(dbxInfo)
}

// QueryAttribution queries partner bucket attribution data
func (keys *attributionDB) QueryAttribution(ctx context.Context, partnerID uuid.UUID, start time.Time, end time.Time) (_ []*attribution.CSVRow, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string
	switch t := keys.db.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		query = fmt.Sprintf(valueAttrQuery, slHour)
	case *pq.Driver:
		query = fmt.Sprintf(valueAttrQuery, pqHour)
	default:
		return nil, Error.New("Unsupported database %t", t)
	}

	rows, err := keys.db.DB.QueryContext(ctx, keys.db.Rebind(query), partnerID[:], start.UTC(), end.UTC(), partnerID[:], start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()
	results := make([]*attribution.CSVRow, 0, 0)
	for rows.Next() {
		r := &attribution.CSVRow{}
		err := rows.Scan(&r.PartnerID, &r.ProjectID, &r.BucketName, &r.RemoteBytesPerHour, &r.InlineBytesPerHour, &r.EgressData)
		if err != nil {
			return results, Error.Wrap(err)
		}
		results = append(results, r)
	}
	return results, nil
}

func attributionFromDBX(info *dbx.ValueAttribution) (*attribution.Info, error) {
	partnerID, err := bytesToUUID(info.PartnerId)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	projectID, err := bytesToUUID(info.ProjectId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &attribution.Info{
		ProjectID:  projectID,
		BucketName: info.BucketName,
		PartnerID:  partnerID,
		CreatedAt:  info.LastUpdated,
	}, nil
}
