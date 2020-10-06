// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"storj.io/common/uuid"
)

type BucketName = string
type ObjectKey []byte

type SegmentPosition struct {
	Part    uint32
	Segment uint32
}

func EncodeSegmentPosition(partNumber, segmentPosition uint32) uint64 {
	return uint64(partNumber)<<32 | uint64(segmentPosition)
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

func SegmentPositionFromEncoded(v uint64) SegmentPosition {
	return SegmentPosition{
		Part:    uint32(v >> 32),
		Segment: uint32(v),
	}
}
func (pos SegmentPosition) Encode() uint64 { return EncodeSegmentPosition(pos.Part, pos.Segment) }

type Metabase struct {
	conn *pgx.Conn
}

func Dial(ctx context.Context, connstr string) (*Metabase, error) {
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
			project_id   BYTEA NOT NULL,
			bucket_name  BYTEA NOT NULL,
			object_key   BYTEA NOT NULL,
			version      INT4  NOT NULL default 0,
			stream_id    BYTEA NOT NULL,

			created_at TIMESTAMP NOT NULL default now(),
			expires_at TIMESTAMP, -- TODO: should we send this to storage nodes at all?
			                      -- TODO: can we use expires_at instead of zombie_deletion_deadline?

			status         INT2 NOT NULL default 0,
			segment_count  INT4 NOT NULL default 0,

			encrypted_metadata_nonce BYTEA default NULL,
			encrypted_metadata       BYTEA default NULL,

			total_encrypted_size INT4 NOT NULL default 0,
			fixed_segment_size   INT4 NOT NULL default 0,

			encryption INT8 NOT NULL default 0,

			zombie_deletion_deadline TIMESTAMPTZ default now() + '1 day', -- should this be in a separate table?

			PRIMARY KEY (project_id, bucket_name, object_key, version)
		);
		`)
	if err != nil {
		return wrapf("failed create objects table: %w", err)
	}
	_, err = mb.conn.Exec(ctx, `
		CREATE TABLE segments (
			-- TODO: how to reverse lookup stream_id -> project_id, bucket_name, object_key?
			stream_id        BYTEA NOT NULL,
			segment_position INT8  NOT NULL,

			root_piece_id       BYTEA NOT NULL,
			encrypted_key_nonce BYTEA NOT NULL,
			encrypted_key       BYTEA NOT NULL,

			plain_offset INT8 NOT NULL default -1,
			plain_size   INT4 NOT NULL default -1,

			encrypted_data_size INT4 NOT NULL,

			redundancy INT8 NOT NULL default 0, -- needs to be 9 bytes, should this be in segments?

			inline_data  BYTEA  DEFAULT NULL,
			node_aliases INT4[] NOT NULL, -- TODO: should we do the migration immediately?

			PRIMARY KEY (stream_id, segment_position)
		)
	`)
	return wrapf("failed create segments table: %w", err)
}

type CreateBucket struct {
	ProjectID  uuid.UUID
	BucketName BucketName
	BucketID   uuid.UUID
}

func (mb *Metabase) CreateBucket(ctx context.Context, opts CreateBucket) error {
	_, err := mb.conn.Exec(ctx, `
		INSERT INTO buckets (
			project_id, bucket_id, bucket_name
		) VALUES ($1, $2, $3)
	`, opts.ProjectID, opts.BucketID, []byte(opts.BucketName))
	return wrapf("failed CreateBucket: %w", err)
}

type BeginObject struct {
	ProjectID  uuid.UUID
	BucketName BucketName
	ObjectKey  ObjectKey
	Version    Version
	StreamID   uuid.UUID

	ExpiresAt *time.Time
}

func (mb *Metabase) BeginObject(ctx context.Context, opts BeginObject) (Version, error) {
	// TODO: verify existence of bucket somehow

	if opts.Version < 0 {
		return -1, errors.New("invalid version number")
	}

	if opts.Version == NextVersion {
		row := mb.conn.QueryRow(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key,
				version,
				stream_id
			) VALUES (
				$1, $2, $3, (
					SELECT coalesce(max(version), 0) + 1
					FROM objects 
					WHERE project_id = $1 AND bucket_name = $2 AND object_key = $3
				), $4)
			RETURNING version
		`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.StreamID)

		var v int64
		if err := row.Scan(&v); err != nil {
			return -1, wrapf("failed BeginObject: %w", err)
		}
		return Version(v), nil
	}

	r, err := mb.conn.Exec(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id
		) VALUES ($1, $2, $3, $4, $5)
	`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID)
	if err != nil {
		return -1, wrapf("failed BeginObject: %w", err)
	}
	if r.RowsAffected() == 0 {
		return -1, fmt.Errorf("bucket does not exist %q/%q", opts.ProjectID, opts.BucketName)
	}
	return opts.Version, nil
}

type BeginSegment struct {
	ProjectID       uuid.UUID
	BucketName      BucketName
	ObjectKey       ObjectKey
	Version         Version
	StreamID        uuid.UUID
	SegmentPosition SegmentPosition
	RootPieceID     []byte
	NodeAliases     NodeAliases
}

func (mb *Metabase) BeginSegment(ctx context.Context, opts BeginSegment) error {
	// NOTE: this isn't strictly necessary, since we can also fail this in CommitSegment.
	//       however, we should prevent creating segements for non-partial objects.

	// NOTE: these queries could be combined into one.

	tx, err := mb.conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Verify that object exists and is partial.
	var value int
	err = tx.QueryRow(ctx, `
		SELECT 1
		FROM objects WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0
	`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID).Scan(&value)
	if err != nil {
		return wrapf("object is not partial: %w", err)
	}

	// Verify that the segment does not exist.
	err = tx.QueryRow(ctx, `
		SELECT 1
		FROM segments WHERE
			stream_id        = $1 AND
			segment_position = $2
	`, opts.StreamID, opts.SegmentPosition.Encode()).Scan(&value)
	if !errors.Is(err, pgx.ErrNoRows) {
		return wrapf("segment already exists: %w", err)
	}
	err = nil // ignore explictly any other err result

	// TODO: error wrapping for concurrency errors
	err = tx.Commit(ctx)
	return wrapf("failed BeginSegment: %w", err)
}

type CommitSegment struct {
	ProjectID         uuid.UUID
	BucketName        BucketName
	ObjectKey         ObjectKey
	Version           Version
	StreamID          uuid.UUID
	SegmentPosition   SegmentPosition
	RootPieceID       []byte
	EncryptedKey      []byte
	EncryptedKeyNonce []byte

	PlainOffset   int64
	PlainSize     int32
	EncryptedSize int32

	NodeAliases NodeAliases
}

func (mb *Metabase) CommitSegment(ctx context.Context, opts CommitSegment) error {
	tx, err := mb.conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Verify that object exists and is partial, how can we do this without transactions?
	var value int
	err = tx.QueryRow(ctx, `
		SELECT 1
		FROM objects WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0
	`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID).Scan(&value)
	if err != nil {
		return wrapf("object is not partial: %w", err)
	}

	// TODO: add other segment fields
	_, err = tx.Exec(ctx, `
		INSERT INTO segments (
			stream_id, segment_position, root_piece_id,
			encrypted_key, encrypted_key_nonce,
			encrypted_data_size, 
			plain_offset, plain_size,
			node_aliases
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, 
			$7, $8,
			$9
		)
	`, opts.StreamID, opts.SegmentPosition.Encode(), opts.RootPieceID,
		opts.EncryptedKey, opts.EncryptedKeyNonce,
		opts.EncryptedSize,
		opts.PlainOffset, opts.PlainSize,
		opts.NodeAliases.Encode(),
	)
	if err != nil {
		return wrapf("failed CommitSegment: %w", err)
	}

	// TODO: error wrapping for concurrency errors
	err = tx.Commit(ctx)
	return wrapf("failed CommitSegment: %w", err)
}

type CommitObject struct {
	ProjectID  uuid.UUID
	BucketName BucketName
	ObjectKey  ObjectKey
	Version    Version
	StreamID   uuid.UUID

	Segments []SegmentOrderProof
}

type SegmentOrderProof struct { // Is there a better name for this?
	Nonce []byte // do we need this?

	Position    SegmentPosition
	Previous    SegmentPosition
	StreamID    uuid.UUID
	PlainOffset int64 // how sensitive is this information?
	PlainSize   int32 // how sensitive is this information?
	SegmentHash []byte

	// Will we later need to have ReferenceStreamID, ReferencePlainOffset, ReferencePlainSize?

	Signature []byte // can or do we need to verify this?
}

func (mb *Metabase) CommitObject(ctx context.Context, opts CommitObject) error {
	if len(opts.Segments) == 0 {
		return mb.CommitObjectV1(ctx, opts)
	}

	tx, err := mb.conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// TODO: check SegmentOrderProof validity

	fixedSegmentSize := int32(0)
	if len(opts.Segments) > 0 {
		fixedSegmentSize = opts.Segments[0].PlainSize
		for _, seg := range opts.Segments[:len(opts.Segments)-1] {
			if seg.PlainSize != fixedSegmentSize {
				fixedSegmentSize = -1
				break
			}
		}
	}

	totalEncryptedSize := int64(0)
	for _, seg := range opts.Segments[:len(opts.Segments)-1] {
		// TODO: we need to get this from the database
		totalEncryptedSize += int64(seg.PlainSize)
	}

	segmentPositions := []uint64{}
	for _, seg := range opts.Segments {
		segmentPositions = append(segmentPositions, seg.Position.Encode())
	}

	// TODO: how do we handle segments that are not in the segment positions
	_, err = tx.Exec(ctx, `
		UPDATE objects SET
			status = 1,
			segment_count = $6,
			total_encrypted_size = $7,
			fixed_segment_size = $8,
			zombie_deletion_deadline = NULL
		WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0;
	`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID,
		len(opts.Segments), totalEncryptedSize, fixedSegmentSize,
	)
	if err != nil {
		return wrapf("failed CommitObject: %w", err)
	}

	// TODO: verify segment offsets and add proofs

	_, err = tx.Exec(ctx, `
		DELETE FROM segments
		WHERE
			stream_id = $1 AND
			not segment_position = any($2::int8[])
	`, opts.StreamID, segmentPositions)
	if err != nil {
		return wrapf("failed to delete segments: %w", err)
	}

	// TODO: error wrapping for concurrency errors

	err = tx.Commit(ctx)
	return wrapf("failed CommitObject: %w", err)
}

func (mb *Metabase) CommitObjectV1(ctx context.Context, opts CommitObject) error {
	tx, err := mb.conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// TODO: figure out what information we actually have, how do we derive this from encrypted stuff

	type Segment struct {
		Position      SegmentPosition
		EncryptedSize int32
		PlainOffset   int64
		PlainSize     int32
	}

	var segments []Segment

	rows, err := tx.Query(ctx, `
		SELECT segment_position, encrypted_data_size, plain_offset, plain_size 
		FROM segments
		WHERE stream_id = $1
		ORDER BY segment_position
	`)
	if err != nil {
		return wrapf("failed CommitObjectV1, select segments: %w", err)
	}

	for rows.Next() {
		var seg Segment
		var pos uint64
		err := rows.Scan(&pos, &seg.EncryptedSize, &seg.PlainOffset, &seg.PlainSize)
		if err != nil {
			rows.Close()
			return wrapf("failed CommitObjectV1, scan: %w", err)
		}
		seg.Position = SegmentPositionFromEncoded(pos)
		segments = append(segments, seg)
	}
	rows.Close()

	fixedSegmentSize := int32(0)
	if len(segments) > 0 {
		fixedSegmentSize = segments[0].PlainSize
		for _, seg := range segments[:len(segments)-1] {
			if seg.PlainSize != fixedSegmentSize {
				fixedSegmentSize = -1
				break
			}
		}
	}

	totalEncryptedSize := int64(0)
	for _, seg := range segments {
		totalEncryptedSize += int64(seg.EncryptedSize)
	}

	segmentPositions := []uint64{}
	for _, seg := range opts.Segments {
		segmentPositions = append(segmentPositions, seg.Position.Encode())
	}

	_, err = tx.Exec(ctx, `
		UPDATE objects SET
			status = 1,
			segment_count = $6,
			total_encrypted_size = $7,
			fixed_segment_size = $8,
			zombie_deletion_deadline = NULL
		WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0;
	`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID,
		len(segments), totalEncryptedSize, fixedSegmentSize)
	if err != nil {
		return wrapf("failed CommitObjectV1: %w", err)
	}

	// TODO: verify segment offsets and add proofs

	_, err = tx.Exec(ctx, `
		DELETE FROM segments
		WHERE
			stream_id = $1 AND
			not segment_position = any($2::int8[])
	`, opts.StreamID, segmentPositions)
	if err != nil {
		return wrapf("failed to delete segments: %w", err)
	}

	err = tx.Commit(ctx)
	return wrapf("failed CommitObjectV1: %w", err)
}

func wrapf(message string, err error) error {
	if err != nil {
		return fmt.Errorf(message, err)
	}
	return nil
}
