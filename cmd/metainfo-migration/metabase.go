package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
)

type BucketName = string
type EncryptedPath []byte

type SegmentPosition struct {
	Part    uint32
	Segment uint32
}

type NodeAlias int32
type NodeAliases []NodeAlias

type Version int64

const (
	NextVersion = Version(0)
)

func (aliases NodeAliases) Encode() []int32 {
	xs := make([]int32, len(aliases))
	for i, v := range aliases {
		xs[i] = int32(v)
	}
	return xs
}

func (pos SegmentPosition) Encode() uint64 { return uint64(pos.Part)<<32 | uint64(pos.Segment) }

type Metabase struct {
	conn *pgx.Conn
}

func DialMetainfo(ctx context.Context, connstr string) (*Metabase, error) {
	conn, err := pgx.Connect(ctx, connstr)
	if err != nil {
		return nil, fmt.Errorf("unable to connect %q: %w", connstr, err)
	}
	return &Metabase{conn}, nil
}

func (mb *Metabase) Exec(ctx context.Context, v string, args ...interface{}) error {
	_, err := mb.conn.Exec(ctx, v, args...)
	return wrapf("failed exec: %w", err)
}

func (mb *Metabase) Close(ctx context.Context) error {
	return mb.conn.Close(ctx)
}

func (mb *Metabase) Drop(ctx context.Context) error {
	_, err := mb.conn.Exec(ctx, `
		DROP TABLE IF EXISTS buckets;
		DROP TABLE IF EXISTS objects;
		DROP TABLE IF EXISTS segments;
		DROP TYPE IF EXISTS  object_status;
	`)
	return wrapf("failed to drop: %w", err)
}

func (mb *Metabase) Migrate(ctx context.Context) error {
	_, err := mb.conn.Exec(ctx, `
		CREATE TABLE buckets (
			project_id     BYTEA NOT NULL,
			bucket_id      BYTEA NOT NULL,

			bucket_name    BYTEA NOT NULL,

			attribution_useragent BYTEA default ''::BYTEA,
			-- see other fields in current dbx

			zombie_deletion_grace_duration INTERVAL default '1 day',

			PRIMARY KEY (bucket_id)
		);
		CREATE UNIQUE INDEX buckets_project_index ON buckets (project_id, bucket_name);

		-- CREATE TYPE encryption_parameters AS (
		-- 	-- total 5 bytes
		-- 	ciphersuite BYTE NOT NULL;
		-- 	block_size  INT4 NOT NULL;
		-- );
		-- 	
		-- CREATE TYPE redundancy_scheme AS (
		-- 	-- total 9 bytes
		-- 	algorithm   BYTE   NOT NULL;
		-- 	share_size  INT4   NOT NULL;
		-- 	required    INT2   NOT NULL;
		-- 	repair      INT2   NOT NULL;
		-- 	optimal     INT2   NOT NULL;
		-- 	total       INT2   NOT NULL;
		-- );

		CREATE TYPE object_status AS ENUM (
			'partial', 'committing', 'committed', 'deleting'
		);

		CREATE TABLE objects (
			project_id     BYTEA NOT NULL,
			bucket_id      BYTEA NOT NULL,
			encrypted_path BYTEA NOT NULL,
			version        INT4  NOT NULL default 0,
			stream_id      BYTEA NOT NULL,

			created_at TIMESTAMP NOT NULL default now(),
			expires_at TIMESTAMP, -- TODO: should we send this to storage nodes at all?

			status         object_status NOT NULL default 'partial',
			segment_count  INT4 NOT NULL default 0,

			encrypted_metadata_nonce BYTEA default NULL,
			encrypted_metadata       BYTEA default NULL,

			total_size         INT4 NOT NULL default 0,
			fixed_segment_size INT4 NOT NULL default 0,

			encryption INT8 NOT NULL default 0,
			redundancy INT8 NOT NULL default 0, -- needs to be 9 bytes, should this be in segments?

			zombie_deletion_deadline TIME default now() + '1 day', -- should this be in a separate table?

			-- TODO: should we have first segment here?
			-- TODO: should we use []segment instead?

			PRIMARY KEY (project_id, bucket_id, encrypted_path, version)
		);

		CREATE TABLE segments (
			-- TODO: how to reverse lookup stream_id -> project_id, bucket_id, encrypted_path?

			stream_id        BYTEA NOT NULL,
			segment_position INT8  NOT NULL,

			root_piece_id       BYTEA NOT NULL,
			encrypted_key_nonce BYTEA NOT NULL,
			encrypted_key       BYTEA NOT NULL,

			data_size    INT4    NOT NULL DEFAULT -1,
			inline_data  BYTEA   DEFAULT NULL,
			node_aliases INT4[]  NOT NULL, -- TODO: should we do the migration immediately?

			PRIMARY KEY (stream_id, segment_position)
		);
	`)
	return wrapf("failed to migrate: %w", err)
}

func (mb *Metabase) CreateBucket(ctx context.Context, projectID UUID, bucketName BucketName, bucketID UUID) error {
	_, err := mb.conn.Exec(ctx, `
		INSERT INTO buckets (
			project_id, bucket_id, bucket_name
		) VALUES ($1, $2, $3)
	`, projectID, bucketID, []byte(bucketName))
	return wrapf("failed to BeginObject: %w", err)
}

func (mb *Metabase) BeginObject(ctx context.Context, projectID UUID, bucketName BucketName, path EncryptedPath, version Version, streamID UUID) error {
	// NOTE: One of the problems in using SELECT is that it implies that
	//       objects/buckets are in the same database, it's not clear whether this is a good constraint to have.
	//       It definitely is more convienient.

	// if version == NextVersion, use a for loop without tx max + insert query

	// TODO: add check for version = -1 for selecting next version
	// TODO: if <key> + version exists then should fail
	r, err := mb.conn.Exec(ctx, `
		INSERT INTO objects (
			project_id, bucket_id, encrypted_path, version, stream_id
		) SELECT $1, bucket_id, $3, $4, $5 -- this verifies existence of a bucket
			FROM buckets WHERE project_id = $1 AND bucket_name = $2
	`, projectID, bucketName, string(path), version, streamID)
	if err != nil {
		return wrapf("failed to BeginObject: %w", err)
	}
	if r.RowsAffected() == 0 {
		return fmt.Errorf("bucket does not exist %q/%q", projectID, bucketName)
	}

	return nil
}

func (mb *Metabase) BeginSegment(ctx context.Context,
	projectID, bucketID UUID, path EncryptedPath, streamID UUID,
	segmentPosition SegmentPosition, rootPieceID []byte, aliases NodeAliases) error {
	// TODO: verify somehow that object is still partial

	/*
		err := mb.Exec(ctx, `
			UPDATE objects SET
				segments_pending = segments_pending + 1
			WHERE
				project_id     = $1 AND
				bucket_id      = $2 AND
				encrypted_path = $3 AND
				stream_id      = $4 AND
				version        = 0  AND
				status         = 'partial';
		`, projectID, bucketID, encryptedPath, streamID)
		check(err)
	*/

	// FIX: just verify that the object is still partial

	// TODO: error wrapping for concurrency errors
	return wrapf("failed BeginSegment: %w", nil)
}

func (mb *Metabase) CommitSegment(ctx context.Context,
	projectID, bucketID UUID, path EncryptedPath, streamID UUID,
	segmentPosition SegmentPosition, rootPieceID []byte,
	encryptedKey, encryptedKeyNonce []byte, aliases NodeAliases) error {

	// TODO: add other segment fields

	// TODO: verify somehow that object is still partial
	/*
		err := mb.Exec(ctx, `
			UPDATE objects SET
				segments_pending = segments_pending + 1
			WHERE
				project_id     = $1 AND
				bucket_id      = $2 AND
				encrypted_path = $3 AND
				stream_id      = $4 AND
				version        = 0  AND
				status         = 'partial';
		`, projectID, bucketID, encryptedPath, streamID)
		check(err)
	*/

	// FIX: use select for verifying object partialness and then insert.

	_, err := mb.conn.Exec(ctx, `
		INSERT INTO segments (
			stream_id, segment_position, root_piece_id,
			encrypted_key, encrypted_key_nonce,
			node_aliases
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6
		)
	`, streamID, segmentPosition.Encode(), rootPieceID,
		encryptedKey, encryptedKeyNonce,
		aliases.Encode(),
	)

	// TODO: should we track segment_status = 'object_committed', helps to clarify the bill

	// TODO: error wrapping for concurrency errors

	return wrapf("failed CommitSegment: %w", err)
}

func (mb *Metabase) CommitObject(ctx context.Context,
	projectID, bucketID UUID, path EncryptedPath, version int64, streamID UUID,
	segmentPositions []SegmentPosition) error {

	if len(segmentPositions) == 0 {
		// TODO: derive segmentPositions from databas by querying the ID
	}

	// TODO: should we rewrite segmentPositions
	// TODO: how do we handle segments that are not in the segment positions

	_, err := mb.conn.Exec(ctx, `
		UPDATE objects SET
			status = 'committed'
			-- calculate number of segments
			-- calculate total size of segments
			-- calculate fixed segment size
		WHERE
			project_id     = $1 AND
			bucket_id      = $2 AND
			encrypted_path = $3 AND
			version        = $4 AND
			stream_id      = $5 AND
			status         = 'partial';
	`, projectID, bucketID, path, version, streamID)

	// TODO: previously was using `segments_pending = segments_done AND` as a protection

	// TODO: error wrapping for concurrency errors

	return wrapf("failed CommitObject: %w", err)
}
