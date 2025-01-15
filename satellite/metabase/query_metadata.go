// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

const (
	MaxFindObjectsByClearMetadataQuerySize = 10
)

func (db *DB) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.ChooseAdapter(opts.ProjectID).FindObjectsByClearMetadata(ctx, opts, startAfter, batchSize)
}

func (p *PostgresAdapter) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	defer mon.Task()(&ctx)(&err)

	// Create query
	query := `
		SELECT
			project_id, bucket_name, object_key, version, stream_id, clear_metadata
		FROM objects@objects_pkey
		WHERE
	`

	// We make a subquery for each clear_metadata part. This is optimized for
	// CockroachDB whose optimizer is very unpredictable when querying with
	// multiple JSONB values, and would often scan the full table instead of
	// using the GIN index.
	args := make([]interface{}, 0)
	containsQueryParts, err := splitToJSONLeaves(opts.ContainsQuery)
	if err != nil {
		return FindObjectsByClearMetadataResult{}, Error.Wrap(err)
	}
	if len(containsQueryParts) > MaxFindObjectsByClearMetadataQuerySize {
		return FindObjectsByClearMetadataResult{}, Error.New("too many values in metadata query")
	}

	if len(containsQueryParts) > 0 {
		query += `(project_id, bucket_name, object_key, version) IN (`
		for i, part := range containsQueryParts {
			if i > 0 {
				query += "INTERSECT \n"
			}
			query += fmt.Sprintf("(SELECT project_id, bucket_name, object_key, version FROM objects@objects_clear_metadata_idx WHERE clear_metadata @> $%d)\n", len(args)+1)
			args = append(args, part)
		}
		query += `)`
	}
	if len(args) > 0 {
		query += ` AND `
	}

	query += fmt.Sprintf("project_id = $%d AND bucket_name = $%d AND status <> $%d AND (expires_at IS NULL OR expires_at > now())", len(args)+1, len(args)+2, len(args)+3)
	args = append(args, opts.ProjectID, opts.BucketName, statusPending)

	// Determine first and last object conditions
	if startAfter.ProjectID.IsZero() {
		// first page => use key prefix
		query += fmt.Sprintf("\nAND (project_id, bucket_name, object_key, version) >= ($%d, $%d, $%d, $%d)", len(args)+1, len(args)+2, len(args)+3, len(args)+4)
		args = append(args, opts.ProjectID, opts.BucketName, ObjectKey(opts.KeyPrefix), 0)
	} else {
		// subsequent pages => use startAfter
		query += fmt.Sprintf("\nAND (project_id, bucket_name, object_key, version) > ($%d, $%d, $%d, $%d)", len(args)+1, len(args)+2, len(args)+3, len(args)+4)
		args = append(args, opts.ProjectID, opts.BucketName, startAfter.ObjectKey, startAfter.Version)
	}

	if opts.KeyPrefix != "" {
		prefixLimit := PrefixLimit(ObjectKey(opts.KeyPrefix))
		query += fmt.Sprintf("\nAND (project_id, bucket_name, object_key, version) < ($%d, $%d, $%d, $%d)", len(args)+1, len(args)+2, len(args)+3, len(args)+4)
		args = append(args, opts.ProjectID, opts.BucketName, prefixLimit, 0)
	}

	query += fmt.Sprintf("\nORDER BY project_id, bucket_name, object_key, version LIMIT $%d", len(args)+1)
	// fmt.Println(query)
	args = append(args, batchSize)

	// Execute query
	p.log.Debug("Querying objects by clear metadata",
		zap.Stringer("Project", opts.ProjectID),
		zap.Stringer("Bucket", opts.BucketName),
		zap.String("KeyPrefix", string(opts.KeyPrefix)),
		zap.String("ContainsQuery", opts.ContainsQuery),
		zap.Int("BatchSize", batchSize),
		zap.String("StartAfterKey", string(startAfter.ObjectKey)),
	)

	result.Objects = make([]FindObjectsByClearMetadataResultObject, 0, batchSize)

	err = withRows(p.db.QueryContext(ctx, query, args...))(func(rows tagsql.Rows) error {
		var last FindObjectsByClearMetadataResultObject
		for rows.Next() {
			err = rows.Scan(
				&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID, &last.ClearMetadata)
			if err != nil {
				return Error.Wrap(err)
			}

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

func splitToJSONLeaves(j string) ([]string, error) {
	var obj interface{}
	if err := json.Unmarshal([]byte(j), &obj); err != nil {
		return nil, err
	}

	var leaves []string
	splitToLeafValues(obj, func(v interface{}) {
		if b, err := json.Marshal(v); err == nil {
			leaves = append(leaves, string(b))
		}
	})
	return leaves, nil
}

func splitToLeafValues(obj interface{}, add func(interface{})) []interface{} {
	switch obj := obj.(type) {
	case map[string]interface{}:
		for k, v := range obj {
			splitToLeafValues(v, func(v interface{}) {
				m := make(map[string]interface{})
				m[k] = v
				add(m)
			})
		}
	case []interface{}:
		for _, v := range obj {
			splitToLeafValues(v, func(v interface{}) {
				a := make([]interface{}, 1)
				a[0] = v
				add(a)
			})
		}
	default:
		add(obj)
	}
	return nil
}
