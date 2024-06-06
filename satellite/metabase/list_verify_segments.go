// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
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
				redundancyScheme{&seg.Redundancy},
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
	// TODO: implement me
	panic("implement me")
}

// ListVerifyBucketList represents a list of buckets.
type ListVerifyBucketList struct {
	Buckets []BucketLocation
}

// Add adds a (projectID, bucketName) to the list of buckets to be checked.
func (list *ListVerifyBucketList) Add(projectID uuid.UUID, bucketName string) {
	list.Buckets = append(list.Buckets, BucketLocation{
		ProjectID:  projectID,
		BucketName: bucketName,
	})
}

// ListBucketsStreamIDsResult is the result of listing segments of a list of buckets.
type ListBucketsStreamIDsResult struct {
	StreamIDs  []uuid.UUID
	Counts     []int
	LastBucket BucketLocation
}

func (list *ListBucketsStreamIDsResult) addStreamID(streamID uuid.UUID, count int) {
	list.StreamIDs = append(list.StreamIDs, streamID)
	list.Counts = append(list.Counts, count)
}

// ListBucketsStreamIDs contains arguments necessary for listing stream segments from buckets.
type ListBucketsStreamIDs struct {
	BucketList     ListVerifyBucketList
	CursorBucket   BucketLocation
	CursorStreamID uuid.UUID
	Limit          int

	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
}

// ListBucketsStreamIDs lists the streamIDs of a list of buckets.
func (db *DB) ListBucketsStreamIDs(ctx context.Context, opts ListBucketsStreamIDs) (ListBucketsStreamIDsResult, error) {

	if opts.Limit <= 0 {
		return ListBucketsStreamIDsResult{}, ErrInvalidRequest.New("invalid limit: %d", opts.Limit)
	}
	ListVerifyLimit.Ensure(&opts.Limit)

	result := ListBucketsStreamIDsResult{}

	bucketNamesBytes := [][]byte{}
	projectIDs := []uuid.UUID{}
	for _, bucket := range opts.BucketList.Buckets {
		bucketNamesBytes = append(bucketNamesBytes, []byte(bucket.BucketName))
		projectIDs = append(projectIDs, bucket.ProjectID)
	}
	// get the list of stream_ids and segment counts from the objects table
	err := withRows(db.db.QueryContext(ctx, `
		SELECT DISTINCT project_id, bucket_name, stream_id, segment_count
		FROM objects
		`+db.asOfTime(opts.AsOfSystemTime, opts.AsOfSystemInterval)+`
		WHERE
			 (project_id, bucket_name, stream_id) > ($4::BYTEA, $5::BYTEA, $6::BYTEA) AND
		(project_id, bucket_name) IN (SELECT UNNEST($1::BYTEA[]),UNNEST($2::BYTEA[]))
		ORDER BY project_id, bucket_name, stream_id ASC
		LIMIT $3
	`, pgutil.UUIDArray(projectIDs), pgutil.ByteaArray(bucketNamesBytes),
		opts.Limit,
		opts.CursorBucket.ProjectID, []byte(opts.CursorBucket.BucketName), opts.CursorStreamID,
	))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var streamID uuid.UUID
			var count int
			err := rows.Scan(
				&result.LastBucket.ProjectID,
				&result.LastBucket.BucketName,
				&streamID,
				&count,
			)
			if err != nil {
				return Error.Wrap(err)
			}
			result.addStreamID(streamID, count)
		}
		return nil
	})
	if err != nil {
		return ListBucketsStreamIDsResult{}, err
	}
	return result, nil
}
