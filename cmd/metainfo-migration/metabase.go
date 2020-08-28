package main

import (
	"context"
	"errors"
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

type ObjectStatus byte

const (
	Partial   = ObjectStatus(0)
	Committed = ObjectStatus(1)
	Deleting  = ObjectStatus(2)
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
	`)
	return wrapf("failed to drop existing: %w", err)
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
	`)
	if err != nil {
		return wrapf("failed create table buckets: %w", err)
	}

	_, err = mb.conn.Exec(ctx, `
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
	`)
	if err != nil {
		return wrapf("failed create types: %w", err)
	}
	_, err = mb.conn.Exec(ctx, `
		CREATE TABLE objects (
			project_id     BYTEA NOT NULL,
			bucket_name    BYTEA NOT NULL,
			encrypted_path BYTEA NOT NULL,
			version        INT4  NOT NULL default 0,
			stream_id      BYTEA NOT NULL,

			created_at TIMESTAMP NOT NULL default now(),
			expires_at TIMESTAMP, -- TODO: should we send this to storage nodes at all?

			status         INT2 NOT NULL default 0,
			segment_count  INT4 NOT NULL default 0,

			encrypted_metadata_nonce BYTEA default NULL,
			encrypted_metadata       BYTEA default NULL,

			total_size         INT4 NOT NULL default 0,
			fixed_segment_size INT4 NOT NULL default 0,

			encryption INT8 NOT NULL default 0,
			redundancy INT8 NOT NULL default 0, -- needs to be 9 bytes, should this be in segments?

			zombie_deletion_deadline TIMESTAMPTZ default now() + '1 day', -- should this be in a separate table?

			-- FIX: we should have first segment here

			PRIMARY KEY (project_id, bucket_name, encrypted_path, version)
		);
		`)
	if err != nil {
		return wrapf("failed create objects table: %w", err)
	}
	_, err = mb.conn.Exec(ctx, `
		CREATE TABLE segments (
			-- TODO: how to reverse lookup stream_id -> project_id, bucket_name, encrypted_path?

			stream_id        BYTEA NOT NULL,
			segment_position INT8  NOT NULL,

			root_piece_id       BYTEA NOT NULL,
			encrypted_key_nonce BYTEA NOT NULL,
			encrypted_key       BYTEA NOT NULL,

			encrypted_data_size   INT4 NOT NULL,
			unencrypted_data_size INT4 NOT NULL,

			inline_data  BYTEA  DEFAULT NULL,
			node_aliases INT4[] NOT NULL, -- TODO: should we do the migration immediately?

			PRIMARY KEY (stream_id, segment_position)
		);
	`)
	return wrapf("failed create segments table: %w", err)
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
	// if version == NextVersion, use a for loop without tx max + insert query

	// TODO: verify existence of bucket somehow

	// TODO: add check for version = -1 for selecting next version
	// TODO: if <key> + version exists then should fail
	r, err := mb.conn.Exec(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, encrypted_path, version, stream_id
		) VALUES ($1, $2, $3, $4, $5)
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
	projectID UUID, bucketName BucketName, path EncryptedPath, streamID UUID,
	segmentPosition SegmentPosition, rootPieceID []byte, aliases NodeAliases) error {

	// NOTE: this isn't strictly necessary, since we can also fail this in CommitSegment.
	//       however, we should prevent creating segements for non-partial objects.

	// NOTE: these queries could be combined into one.

	// Verify that object exists and is partial.
	var value int
	err := mb.conn.QueryRow(ctx, `
		SELECT 1
		FROM objects WHERE
			project_id     = $1 AND
			bucket_name    = $2 AND
			encrypted_path = $3 AND
			stream_id      = $4 AND
			-- version     = $5 AND
			status         = 0
	`, projectID, bucketName, path, streamID).Scan(&value)
	if err != nil {
		return wrapf("object is not partial: %w", err)
	}

	// Verify that the segment does not exist.
	err = mb.conn.QueryRow(ctx, `
		SELECT 1
		FROM segments WHERE
			stream_id        = $1 AND
			segment_position = $2
	`, streamID, segmentPosition.Encode()).Scan(&value)
	if !errors.Is(err, pgx.ErrNoRows) {
		return wrapf("segment already exists: %w", err)
	}

	return nil
}

func (mb *Metabase) CommitSegment(ctx context.Context,
	projectID UUID, bucketName BucketName, path EncryptedPath, streamID UUID,
	segmentPosition SegmentPosition, rootPieceID []byte,
	encryptedKey, encryptedKeyNonce []byte,
	encryptedSize, unencryptedSize int32,
	aliases NodeAliases) error {

	// Verify that object exists and is partial, how can we do this without transactions?
	var value int
	err := mb.conn.QueryRow(ctx, `
		SELECT 1
		FROM objects WHERE
			project_id     = $1 AND
			bucket_name    = $2 AND
			encrypted_path = $3 AND
			stream_id      = $4 AND
			-- version     = $5 AND
			status         = 0
	`, projectID, bucketName, path, streamID).Scan(&value)
	if err != nil {
		return wrapf("object is not partial: %w", err)
	}

	// TODO: add other segment fields
	_, err = mb.conn.Exec(ctx, `
		INSERT INTO segments (
			stream_id, segment_position, root_piece_id,
			encrypted_key, encrypted_key_nonce,
			encrypted_data_size, unencrypted_data_size,
			node_aliases
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7,
			$8
		)
	`, streamID, segmentPosition.Encode(), rootPieceID,
		encryptedKey, encryptedKeyNonce,
		encryptedSize, unencryptedSize,
		aliases.Encode(),
	)

	// TODO: error wrapping for concurrency errors
	return wrapf("failed CommitSegment: %w", err)
}

func (mb *Metabase) CommitObject(ctx context.Context,
	projectID UUID, bucketName BucketName, path EncryptedPath, version int64, streamID UUID,
	segmentPositions []SegmentPosition) error {

	if len(segmentPositions) == 0 {
		// TODO: derive segmentPositions from database by querying the ID
	}

	// TODO: how do we handle segments that are not in the segment positions

	_, err := mb.conn.Exec(ctx, `
		UPDATE objects SET
			status = 1
			-- calculate number of segments
			-- calculate total size of segments
			-- calculate fixed segment size
		WHERE
			project_id     = $1 AND
			bucket_name      = $2 AND
			encrypted_path = $3 AND
			version        = $4 AND
			stream_id      = $5 AND
			status         = 0;
	`, projectID, bucketName, path, version, streamID)

	// TODO: previously was using `segments_pending = segments_done AND` as a protection

	// TODO: error wrapping for concurrency errors

	return wrapf("failed CommitObject: %w", err)
}
