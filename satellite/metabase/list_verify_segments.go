// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// ListVerifyLimit is the maximum number of items the client can request for listing.
const ListVerifyLimit = intLimitRange(100000)

// ListVerifySegments contains arguments necessary for listing stream segments.
type ListVerifySegments struct {
	CursorStreamID uuid.UUID
	CursorPosition SegmentPosition

	StreamIDs []uuid.UUID
	Limit     int

	CreatedAfter  *time.Time
	CreatedBefore *time.Time

	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
}

// ListVerifySegmentsResult is the result of ListVerifySegments.
type ListVerifySegmentsResult struct {
	Segments []VerifySegment
}

// VerifySegment result of listing segments for verifying remote segments.
type VerifySegment struct {
	StreamID uuid.UUID
	Position SegmentPosition

	CreatedAt  time.Time
	RepairedAt *time.Time

	RootPieceID storj.PieceID
	Redundancy  storj.RedundancyScheme

	AliasPieces AliasPieces
}

func (opts *ListVerifySegments) getQueryAndParameters(asof string) (string, []interface{}) {

	if len(opts.StreamIDs) == 0 {
		return `
		SELECT
			stream_id, position,
			created_at, repaired_at,
			root_piece_id, redundancy,
			remote_alias_pieces
		FROM segments
		` + asof + `
		WHERE
			(stream_id, position) > ($1, $2) AND
			inline_data IS NULL AND
			remote_alias_pieces IS NOT NULL AND
			(segments.expires_at IS NULL OR segments.expires_at > now()) AND
			($3::TIMESTAMPTZ IS NULL OR segments.created_at > $3) AND -- created after
			($4::TIMESTAMPTZ IS NULL OR segments.created_at < $4)     -- created before
		ORDER BY stream_id ASC, position ASC
		LIMIT $5
	`, []interface{}{
				opts.CursorStreamID,
				opts.CursorPosition,
				opts.CreatedAfter,
				opts.CreatedBefore,
				opts.Limit,
			}
	}
	return `
		SELECT
			segments.stream_id, segments.position,
			segments.created_at, segments.repaired_at,
			segments.root_piece_id, segments.redundancy,
			segments.remote_alias_pieces
		FROM segments
		` + asof + `
		WHERE
			stream_id = ANY($1) AND
			(segments.stream_id, segments.position) > ($2, $3) AND
			segments.inline_data IS NULL AND
			segments.remote_alias_pieces IS NOT NULL AND
			(segments.expires_at IS NULL OR segments.expires_at > now()) AND
			($4::TIMESTAMPTZ IS NULL OR segments.created_at > $4) AND -- created after
			($5::TIMESTAMPTZ IS NULL OR segments.created_at < $5)     -- created before
		ORDER BY segments.stream_id ASC, segments.position ASC
		LIMIT $6
	`, []interface{}{
			pgutil.UUIDArray(opts.StreamIDs),
			opts.CursorStreamID,
			opts.CursorPosition,
			opts.CreatedAfter,
			opts.CreatedBefore,
			opts.Limit,
		}
}

func (opts *ListVerifySegments) getSpannerQueryAndParameters() spanner.Statement {
	tuple, err := spannerutil.TupleGreaterThanSQL([]string{"stream_id", "position"}, []string{"@stream_id", "@position"}, false)
	if err != nil {
		return spanner.Statement{}
	}
	if len(opts.StreamIDs) == 0 {
		return spanner.Statement{
			SQL: `
				SELECT
					stream_id, position,
					created_at, repaired_at,
					root_piece_id, redundancy,
					remote_alias_pieces
				FROM segments
				WHERE
					` + tuple + `
					AND inline_data IS NULL
					AND remote_alias_pieces IS NOT NULL
					AND (segments.expires_at IS NULL OR segments.expires_at > CURRENT_TIMESTAMP)
					AND (@created_after IS NULL OR segments.created_at > @created_after)
					AND (@created_before IS NULL OR segments.created_at < @created_before)
				ORDER BY stream_id ASC, position ASC
				LIMIT @limit
			`, Params: map[string]any{
				"stream_id":      opts.CursorStreamID,
				"position":       opts.CursorPosition,
				"created_after":  opts.CreatedAfter,
				"created_before": opts.CreatedBefore,
				"limit":          opts.Limit,
			},
		}
	}

	streamIDsBytes := make([][]byte, len(opts.StreamIDs))
	for i, streamID := range opts.StreamIDs {
		streamIDsBytes[i] = streamID.Bytes()
	}

	tuple, err = spannerutil.TupleGreaterThanSQL([]string{"segments.stream_id", "segments.position"}, []string{"@stream_id", "@position"}, false)
	if err != nil {
		return spanner.Statement{}
	}
	return spanner.Statement{
		SQL: `
			SELECT
				segments.stream_id, segments.position,
				segments.created_at, segments.repaired_at,
				segments.root_piece_id, segments.redundancy,
				segments.remote_alias_pieces
			FROM segments
			WHERE
				stream_id IN UNNEST(@stream_ids)
				AND ` + tuple + `
				AND segments.inline_data IS NULL
				AND segments.remote_alias_pieces IS NOT NULL
				AND (segments.expires_at IS NULL OR segments.expires_at > CURRENT_TIMESTAMP)
				AND (@created_after IS NULL OR segments.created_at > @created_after)
				AND (@created_before IS NULL OR segments.created_at < @created_before)
			ORDER BY segments.stream_id ASC, segments.position ASC
			LIMIT @limit`,
		Params: map[string]any{
			"stream_ids":     streamIDsBytes,
			"stream_id":      opts.CursorStreamID,
			"position":       opts.CursorPosition,
			"created_after":  opts.CreatedAfter,
			"created_before": opts.CreatedBefore,
			"limit":          opts.Limit,
		},
	}
}

// ListVerifySegments lists specified stream segments.
func (db *DB) ListVerifySegments(ctx context.Context, opts ListVerifySegments) (result ListVerifySegmentsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.Limit <= 0 {
		return ListVerifySegmentsResult{}, ErrInvalidRequest.New("invalid limit: %d", opts.Limit)
	}
	ListVerifyLimit.Ensure(&opts.Limit)
	result.Segments = make([]VerifySegment, 0, opts.Limit)

	for _, adapter := range db.adapters {
		theseSegments, err := adapter.ListVerifySegments(ctx, opts)
		if err != nil {
			return ListVerifySegmentsResult{}, Error.Wrap(err)
		}
		result.Segments = append(result.Segments, theseSegments...)
	}
	return result, Error.Wrap(err)
}

// ListVerifySegments lists the segments in a specified stream.
func (p *PostgresAdapter) ListVerifySegments(ctx context.Context, opts ListVerifySegments) (segments []VerifySegment, err error) {
	asOfString := LimitedAsOfSystemTime(p.impl, time.Now(), opts.AsOfSystemTime, opts.AsOfSystemInterval)
	query, parameters := opts.getQueryAndParameters(asOfString)

	err = withRows(p.db.QueryContext(ctx, query, parameters...))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var seg VerifySegment
			err := rows.Scan(
				&seg.StreamID,
				&seg.Position,

				&seg.CreatedAt,
				&seg.RepairedAt,

				&seg.RootPieceID,
				&seg.Redundancy,
				&seg.AliasPieces,
			)
			if err != nil {
				return Error.Wrap(err)
			}

			segments = append(segments, seg)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return segments, nil
}

// ListVerifySegments lists the segments in a specified stream.
func (s *SpannerAdapter) ListVerifySegments(ctx context.Context, opts ListVerifySegments) (segments []VerifySegment, err error) {
	queryStatement := opts.getSpannerQueryAndParameters()

	txn := s.client.Single().WithTimestampBound(spannerutil.MaxStalenessFromAOSI(opts.AsOfSystemInterval))
	return spannerutil.CollectRows(txn.QueryWithOptions(ctx, queryStatement,
		spanner.QueryOptions{
			Priority: spannerpb.RequestOptions_PRIORITY_LOW,
		},
	), func(row *spanner.Row, seg *VerifySegment) error {
		return row.Columns(
			&seg.StreamID,
			&seg.Position,

			&seg.CreatedAt,
			&seg.RepairedAt,

			&seg.RootPieceID,
			&seg.Redundancy,
			&seg.AliasPieces,
		)
	})
}

// ListVerifyBucketList represents a list of buckets.
type ListVerifyBucketList struct {
	Buckets []BucketLocation
}

// Add adds a (projectID, bucketName) to the list of buckets to be checked.
func (list *ListVerifyBucketList) Add(projectID uuid.UUID, bucketName BucketName) {
	list.Buckets = append(list.Buckets, BucketLocation{
		ProjectID:  projectID,
		BucketName: bucketName,
	})
}

// ListBucketStreamIDs contains arguments necessary for listing stream segments from bucket.
type ListBucketStreamIDs struct {
	Bucket BucketLocation
	Limit  int

	AsOfSystemInterval time.Duration
}

// ListBucketStreamIDs lists the streamIDs from a bucket.
func (db *DB) ListBucketStreamIDs(ctx context.Context, opts ListBucketStreamIDs, f func(ctx context.Context, streamIDs []uuid.UUID) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.Limit <= 0 {
		return ErrInvalidRequest.New("invalid limit: %d", opts.Limit)
	}
	ListVerifyLimit.Ensure(&opts.Limit)

	for _, adapter := range db.adapters {
		if err := adapter.ListBucketStreamIDs(ctx, opts, f); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// ListBucketStreamIDs lists the streamIDs from a bucket.
func (p *PostgresAdapter) ListBucketStreamIDs(ctx context.Context, opts ListBucketStreamIDs, process func(ctx context.Context, streamIDs []uuid.UUID) error) error {
	streamIDs := make([]uuid.UUID, 0, opts.Limit)

	// TODO this implementation is not efficient for large production buckets
	// but for now it won't be used in production
	err := withRows(p.db.QueryContext(ctx, `
		SELECT stream_id
		FROM objects
		`+p.impl.AsOfSystemInterval(opts.AsOfSystemInterval)+`
		WHERE
			 (project_id, bucket_name) = ($1::BYTEA, $2::BYTEA)
	`, opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName),
	))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var streamID uuid.UUID
			err := rows.Scan(&streamID)
			if err != nil {
				return Error.Wrap(err)
			}

			streamIDs = append(streamIDs, streamID)
			if len(streamIDs) >= opts.Limit {
				if err := process(ctx, streamIDs); err != nil {
					return Error.Wrap(err)
				}
				streamIDs = streamIDs[:0]
			}
		}
		return nil
	})
	if err != nil {
		return Error.Wrap(err)
	}

	if len(streamIDs) > 0 {
		if err := process(ctx, streamIDs); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// ListBucketStreamIDs lists the streamIDs from a bucket.
func (s *SpannerAdapter) ListBucketStreamIDs(ctx context.Context, opts ListBucketStreamIDs, process func(ctx context.Context, streamIDs []uuid.UUID) error) error {
	statement := spanner.Statement{
		SQL: `
			SELECT stream_id
			FROM objects
			WHERE project_id = @project_id AND bucket_name = @bucket_name
		`,
		Params: map[string]any{
			"project_id":  opts.Bucket.ProjectID,
			"bucket_name": opts.Bucket.BucketName,
		},
	}

	txn, err := s.client.BatchReadOnlyTransaction(ctx, spanner.StrongRead())
	if err != nil {
		return Error.Wrap(err)
	}
	defer txn.Close()

	partitions, err := txn.PartitionQueryWithOptions(ctx, statement, spanner.PartitionOptions{
		PartitionBytes: 0,
		MaxPartitions:  0,
	}, spanner.QueryOptions{
		Priority: spannerpb.RequestOptions_PRIORITY_LOW,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	streamIDs := make([]uuid.UUID, 0, opts.Limit)

	for _, partition := range partitions {
		iter := txn.Execute(ctx, partition)
		err := iter.Do(func(r *spanner.Row) error {
			var streamID uuid.UUID
			if err := r.Columns(&streamID); err != nil {
				return Error.Wrap(err)
			}

			streamIDs = append(streamIDs, streamID)
			if len(streamIDs) >= opts.Limit {
				if err := process(ctx, streamIDs); err != nil {
					return Error.Wrap(err)
				}
				streamIDs = streamIDs[:0]
			}

			return nil
		})
		if err != nil {
			return Error.Wrap(err)
		}
	}

	if len(streamIDs) > 0 {
		if err := process(ctx, streamIDs); err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}
