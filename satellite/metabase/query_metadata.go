// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/shared/tagsql"
)

type FindObjectsByClearMetadata struct {
	ProjectID     uuid.UUID
	BucketName    BucketName
	KeyPrefix     string
	ContainsQuery string
}

type FindObjectsByClearMetadataResult struct {
	Objects []FindObjectsByClearMetadataResultObject
}

type FindObjectsByClearMetadataResultObject struct {
	ObjectStream
	ClearMetadata string
}

func (db *DB) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.ChooseAdapter(opts.ProjectID).FindObjectsByClearMetadata(ctx, opts, startAfter, batchSize)
}

func (p *PostgresAdapter) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	defer mon.Task()(&ctx)(&err)

	// Determine first and last object conditions
	//
	// It looks like the query is optimized most consistently if we have both a
	// start and end condition for object keys. So we come up with one, even if
	// we do not have a start condition or prefix.
	var startCondition string
	var startKey ObjectKey
	var startVersion Version
	if startAfter.ProjectID.IsZero() {
		// first page => use key prefix
		startCondition = `(project_id, bucket_name, object_key, version) >= ($1, $2, $3, $4)`
		startKey = ObjectKey(opts.KeyPrefix)
		startVersion = 0
	} else {
		// subsequent pages => use startAfter
		startCondition = `(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)`
		startKey = startAfter.ObjectKey
		startVersion = startAfter.Version
	}

	var endCondition string
	var endKey = []byte(opts.KeyPrefix)
	if len(endKey) == 0 || endKey[len(endKey)-1] == 0xff {
		// TODO: this is not 100% accurate, but it is the best we can do without a prefix.
		endKey = append(endKey, 0xff)
		endCondition = `(project_id, bucket_name, object_key, version) <= ($5, $6, $7, $8)`
	} else {
		endKey[len(endKey)-1]++
		endCondition = `(project_id, bucket_name, object_key, version) < ($5, $6, $7, $8)`
	}

	// Create query
	query := `
		SELECT
			project_id, bucket_name, object_key, version, stream_id, clear_metadata
		FROM objects
		WHERE
			` + startCondition + ` AND
			` + endCondition + ` AND
			clear_metadata @> $9 AND
			status <> ` + statusPending + ` AND
			(expires_at IS NULL OR expires_at > now())
		ORDER BY project_id, bucket_name, object_key, version
		LIMIT $10;
	`

	result.Objects = make([]FindObjectsByClearMetadataResultObject, 0, batchSize)

	err = withRows(p.db.QueryContext(ctx, query,
		opts.ProjectID, opts.BucketName, startKey, startVersion,
		opts.ProjectID, opts.BucketName, endKey, 0,
		opts.ContainsQuery,
		batchSize),
	)(func(rows tagsql.Rows) error {
		var last FindObjectsByClearMetadataResultObject
		for rows.Next() {
			err = rows.Scan(
				&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID, &last.ClearMetadata)
			if err != nil {
				return Error.Wrap(err)
			}

			p.log.Debug("Querying objects by clear metadata",
				zap.Stringer("Project", last.ProjectID),
				zap.Stringer("Bucket", last.BucketName),
				zap.String("Object Key", string(last.ObjectKey)),
				zap.Int64("Version", int64(last.Version)),
				zap.Stringer("StreamID", last.StreamID),
			)
			result.Objects = append(result.Objects, last)
		}

		return nil
	})
	if err != nil {
		return FindObjectsByClearMetadataResult{}, Error.Wrap(err)
	}
	return result, nil
}

func (p *SpannerAdapter) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	return FindObjectsByClearMetadataResult{}, errors.New("not implemented")
}
