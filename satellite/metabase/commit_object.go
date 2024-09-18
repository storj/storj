// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

type commitObjectWithSegmentsTransactionAdapter interface {
	fetchSegmentsForCommit(ctx context.Context, streamID uuid.UUID) (segments []segmentInfoForCommit, err error)
	finalizeObjectCommitWithSegments(ctx context.Context, opts CommitObjectWithSegments, nextStatus ObjectStatus, finalSegments []segmentToCommit, totalPlainSize int64, totalEncryptedSize int64, fixedSegmentSize int32, nextVersion Version, object *Object) error
	deleteSegmentsNotInCommit(ctx context.Context, streamID uuid.UUID, segments []SegmentPosition) (deletedSegmentCount int64, err error)

	precommitTransactionAdapter
}

// CommitObjectWithSegments contains arguments necessary for committing an object.
//
// TODO: not ready for production.
type CommitObjectWithSegments struct {
	ObjectStream

	EncryptedMetadata             []byte
	EncryptedMetadataNonce        []byte
	EncryptedMetadataEncryptedKey []byte

	// TODO: this probably should use segment ranges rather than individual items
	Segments []SegmentPosition

	// DisallowDelete indicates whether the user is allowed to overwrite
	// the previous unversioned object.
	DisallowDelete bool

	// Versioned indicates whether an object is allowed to have multiple versions.
	Versioned bool
}

// CommitObjectWithSegments commits pending object to the database.
//
// TODO: not ready for production.
func (db *DB) CommitObjectWithSegments(ctx context.Context, opts CommitObjectWithSegments) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return Object{}, err
	}
	if err := verifySegmentOrder(opts.Segments); err != nil {
		return Object{}, err
	}

	var deletedSegmentCount int64
	var precommit PrecommitConstraintResult
	err = db.ChooseAdapter(opts.ProjectID).WithTx(ctx, func(ctx context.Context, adapter TransactionAdapter) error {
		// TODO: should we prevent this from executing when the object has been committed
		// currently this requires quite a lot of database communication, so invalid handling can be expensive.

		precommit, err = db.PrecommitConstraint(ctx, PrecommitConstraint{
			Location:                   opts.Location(),
			Versioned:                  opts.Versioned,
			DisallowDelete:             opts.DisallowDelete,
			TestingPrecommitDeleteMode: db.config.TestingPrecommitDeleteMode,
		}, adapter)
		if err != nil {
			return err
		}

		segmentsInDatabase, err := adapter.fetchSegmentsForCommit(ctx, opts.StreamID)
		if err != nil {
			return err
		}

		finalSegments, segmentsToDelete, err := determineCommitActions(opts.Segments, segmentsInDatabase)
		if err != nil {
			return err
		}

		err = adapter.updateSegmentOffsets(ctx, opts.StreamID, finalSegments)
		if err != nil {
			return err
		}

		deletedSegmentCount, err = adapter.deleteSegmentsNotInCommit(ctx, opts.StreamID, segmentsToDelete)
		if err != nil {
			return err
		}

		// TODO: would we even need this when we make main index plain_offset?
		fixedSegmentSize := int32(0)
		if len(finalSegments) > 0 {
			fixedSegmentSize = finalSegments[0].PlainSize
			for i, seg := range finalSegments {
				if seg.Position.Part != 0 || seg.Position.Index != uint32(i) {
					fixedSegmentSize = -1
					break
				}
				if i < len(finalSegments)-1 && seg.PlainSize != fixedSegmentSize {
					fixedSegmentSize = -1
					break
				}
			}
		}

		var totalPlainSize, totalEncryptedSize int64
		for _, seg := range finalSegments {
			totalPlainSize += int64(seg.PlainSize)
			totalEncryptedSize += int64(seg.EncryptedSize)
		}

		nextStatus := committedWhereVersioned(opts.Versioned)
		nextVersion := opts.Version
		if nextVersion < precommit.HighestVersion {
			nextVersion = precommit.HighestVersion + 1
		}

		err = adapter.finalizeObjectCommitWithSegments(ctx, opts, nextStatus, finalSegments, totalPlainSize, totalEncryptedSize, fixedSegmentSize, nextVersion, &object)
		if err != nil {
			return err
		}

		object.StreamID = opts.StreamID
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey
		object.Version = nextVersion
		object.Status = nextStatus
		object.SegmentCount = int32(len(finalSegments))
		object.EncryptedMetadataNonce = opts.EncryptedMetadataNonce
		object.EncryptedMetadata = opts.EncryptedMetadata
		object.EncryptedMetadataEncryptedKey = opts.EncryptedMetadataEncryptedKey
		object.TotalPlainSize = totalPlainSize
		object.TotalEncryptedSize = totalEncryptedSize
		object.FixedSegmentSize = fixedSegmentSize
		return nil
	})
	if err != nil {
		return Object{}, err
	}

	precommit.submitMetrics()

	mon.Meter("object_commit").Mark(1)
	mon.IntVal("object_commit_segments").Observe(int64(object.SegmentCount))
	mon.IntVal("object_commit_encrypted_size").Observe(object.TotalEncryptedSize)
	mon.Meter("segment_delete").Mark64(deletedSegmentCount)

	return object, nil
}

func (ptx *postgresTransactionAdapter) finalizeObjectCommitWithSegments(ctx context.Context, opts CommitObjectWithSegments, nextStatus ObjectStatus, finalSegments []segmentToCommit, totalPlainSize int64, totalEncryptedSize int64, fixedSegmentSize int32, nextVersion Version, object *Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = ptx.tx.QueryRowContext(ctx, `
			UPDATE objects SET
				version = $14,
				status = $6,
				segment_count = $7,

				encrypted_metadata_nonce         = $8,
				encrypted_metadata               = $9,
				encrypted_metadata_encrypted_key = $10,

				total_plain_size     = $11,
				total_encrypted_size = $12,
				fixed_segment_size   = $13,
				zombie_deletion_deadline = NULL
			WHERE (project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
				status = `+statusPending+`
			RETURNING
				created_at, expires_at,
				encryption;
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID, nextStatus,
		len(finalSegments),
		opts.EncryptedMetadataNonce, opts.EncryptedMetadata, opts.EncryptedMetadataEncryptedKey,
		totalPlainSize,
		totalEncryptedSize,
		fixedSegmentSize,
		nextVersion,
	).
		Scan(
			&object.CreatedAt, &object.ExpiresAt,
			encryptionParameters{&object.Encryption},
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
		}
		return Error.New("failed to update object: %w", err)
	}
	return nil
}

func (stx *spannerTransactionAdapter) finalizeObjectCommitWithSegments(ctx context.Context, opts CommitObjectWithSegments, nextStatus ObjectStatus, finalSegments []segmentToCommit, totalPlainSize int64, totalEncryptedSize int64, fixedSegmentSize int32, nextVersion Version, object *Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	// We cannot do an UPDATE here because we want to change the version column,
	// and that column is part of the primary key. We must delete the row and
	// insert a new one.

	deleted := false
	err = stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			DELETE FROM objects
			WHERE project_id    = @project_id
				AND bucket_name = @bucket_name
				AND object_key  = @object_key
				AND version     = @previous_version
				AND stream_id   = @stream_id
				AND status      = ` + statusPending + `
			THEN RETURN
				created_at, expires_at, encryption
		`,
		Params: map[string]interface{}{
			"project_id":       opts.ProjectID,
			"bucket_name":      opts.BucketName,
			"object_key":       opts.ObjectKey,
			"previous_version": opts.Version,
			"stream_id":        opts.StreamID,
		},
	}).Do(func(row *spanner.Row) error {
		deleted = true
		err := row.Columns(&object.CreatedAt, &object.ExpiresAt, encryptionParameters{&object.Encryption})
		if err != nil {
			return Error.New("failed to read old object details: %w", err)
		}
		return nil
	})
	if err != nil {
		return Error.New("failed to update object: %w", err)
	}
	if !deleted {
		return ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
	}

	_, err = stx.tx.Update(ctx, spanner.Statement{
		SQL: `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version,
				stream_id,
				created_at, expires_at, status,
			    segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			    total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption, zombie_deletion_deadline
			) VALUES (
				@project_id, @bucket_name, @object_key, @version,
				@stream_id,
				@created_at, @expires_at, @status,
			    @segment_count,
				@encrypted_metadata_nonce, @encrypted_metadata, @encrypted_metadata_encrypted_key,
			    @total_plain_size, @total_encrypted_size, @fixed_segment_size,
				@encryption, NULL
			)
		`,
		Params: map[string]interface{}{
			"project_id":                       opts.ProjectID,
			"bucket_name":                      opts.BucketName,
			"object_key":                       opts.ObjectKey,
			"version":                          nextVersion,
			"stream_id":                        opts.StreamID,
			"created_at":                       object.CreatedAt,
			"expires_at":                       object.ExpiresAt,
			"status":                           int64(nextStatus),
			"segment_count":                    len(finalSegments),
			"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
			"encrypted_metadata":               opts.EncryptedMetadata,
			"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
			"total_plain_size":                 totalPlainSize,
			"total_encrypted_size":             totalEncryptedSize,
			"fixed_segment_size":               int64(fixedSegmentSize),
			"encryption":                       encryptionParameters{&object.Encryption},
		},
	})

	return Error.Wrap(err)
}

func verifySegmentOrder(positions []SegmentPosition) error {
	if len(positions) == 0 {
		return nil
	}

	last := positions[0]
	for _, next := range positions[1:] {
		if !last.Less(next) {
			return Error.New("segments not in ascending order, got %v before %v", last, next)
		}
		last = next
	}

	return nil
}

// segmentInfoForCommit is database state prior to deleting objects.
type segmentInfoForCommit struct {
	Position      SegmentPosition
	EncryptedSize int32
	PlainOffset   int64
	PlainSize     int32
}

// fetchSegmentsForCommit loads information necessary for validating segment existence and offsets.
func (ptx *postgresTransactionAdapter) fetchSegmentsForCommit(ctx context.Context, streamID uuid.UUID) (segments []segmentInfoForCommit, err error) {
	defer mon.Task()(&ctx)(&err)

	err = withRows(ptx.tx.QueryContext(ctx, `
		SELECT position, encrypted_size, plain_offset, plain_size
		FROM segments
		WHERE stream_id = $1
		ORDER BY position
	`, streamID))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var segment segmentInfoForCommit
			err := rows.Scan(&segment.Position, &segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}
			segments = append(segments, segment)
		}
		return nil
	})
	if err != nil {
		return nil, Error.New("failed to fetch segments: %w", err)
	}
	return segments, nil
}

func (stx *spannerTransactionAdapter) fetchSegmentsForCommit(ctx context.Context, streamID uuid.UUID) (segments []segmentInfoForCommit, err error) {
	defer mon.Task()(&ctx)(&err)

	segments, err = spannerutil.CollectRows(stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			SELECT position, encrypted_size, plain_offset, plain_size
			FROM segments
			WHERE stream_id = @stream_id
			ORDER BY position
		`,
		Params: map[string]interface{}{
			"stream_id": streamID,
		},
	}), func(row *spanner.Row, segment *segmentInfoForCommit) error {
		return Error.Wrap(row.Columns(
			&segment.Position, spannerutil.Int(&segment.EncryptedSize), &segment.PlainOffset, spannerutil.Int(&segment.PlainSize),
		))
	})

	return segments, Error.Wrap(err)
}

type segmentToCommit struct {
	Position       SegmentPosition
	OldPlainOffset int64
	PlainSize      int32
	EncryptedSize  int32
}

// determineCommitActions detects how should the database be updated and which segments should be deleted.
func determineCommitActions(segments []SegmentPosition, segmentsInDatabase []segmentInfoForCommit) (commit []segmentToCommit, toDelete []SegmentPosition, err error) {
	var invalidSegments errs.Group

	commit = make([]segmentToCommit, 0, len(segments))
	diffSegmentsWithDatabase(segments, segmentsInDatabase, func(a *SegmentPosition, b *segmentInfoForCommit) {
		// If we do not have an appropriate segment in the database it means
		// either the segment was deleted before commit finished or the
		// segment was not uploaded. Either way we need to fail the commit.
		if b == nil {
			invalidSegments.Add(fmt.Errorf("%v: segment not committed", *a))
			return
		}

		// If we do not commit a segment that's in a database we should delete them.
		// This could happen when the user tries to upload a segment,
		// fails, reuploads and then during commit decides to not commit into the object.
		if a == nil {
			toDelete = append(toDelete, b.Position)
			return
		}

		commit = append(commit, segmentToCommit{
			Position:       *a,
			OldPlainOffset: b.PlainOffset,
			PlainSize:      b.PlainSize,
			EncryptedSize:  b.EncryptedSize,
		})
	})

	if err := invalidSegments.Err(); err != nil {
		return nil, nil, Error.New("segments and database does not match: %v", err)
	}
	return commit, toDelete, nil
}

// convertToFinalSegments converts segmentInfoForCommit to segmentToCommit.
func convertToFinalSegments(segmentsInDatabase []segmentInfoForCommit) (commit []segmentToCommit) {
	commit = make([]segmentToCommit, 0, len(segmentsInDatabase))
	for _, seg := range segmentsInDatabase {
		commit = append(commit, segmentToCommit{
			Position:       seg.Position,
			OldPlainOffset: seg.PlainOffset,
			PlainSize:      seg.PlainSize,
			EncryptedSize:  seg.EncryptedSize,
		})
	}
	return commit
}

// updateSegmentOffsets updates segment offsets that didn't match the database state.
func (ptx *postgresTransactionAdapter) updateSegmentOffsets(ctx context.Context, streamID uuid.UUID, updates []segmentToCommit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(updates) == 0 {
		return nil
	}

	// When none of the segments have changed, then the update will be skipped.

	// Update plain offsets of the segments.
	var batch struct {
		Positions    []int64
		PlainOffsets []int64
	}
	expectedOffset := int64(0)
	for _, u := range updates {
		if u.OldPlainOffset != expectedOffset {
			batch.Positions = append(batch.Positions, int64(u.Position.Encode()))
			batch.PlainOffsets = append(batch.PlainOffsets, expectedOffset)
		}
		expectedOffset += int64(u.PlainSize)
	}
	if len(batch.Positions) == 0 {
		return nil
	}

	updateResult, err := ptx.tx.ExecContext(ctx, `
		UPDATE segments
		SET plain_offset = P.plain_offset
		FROM (SELECT unnest($2::INT8[]), unnest($3::INT8[])) as P(position, plain_offset)
		WHERE segments.stream_id = $1 AND segments.position = P.position
	`, streamID, pgutil.Int8Array(batch.Positions), pgutil.Int8Array(batch.PlainOffsets))
	if err != nil {
		return Error.New("unable to update segments offsets: %w", err)
	}

	affected, err := updateResult.RowsAffected()
	if err != nil {
		return Error.New("unable to get number of affected segments: %w", err)
	}
	if affected != int64(len(batch.Positions)) {
		return Error.New("not all segments were updated, expected %d got %d", len(batch.Positions), affected)
	}

	return nil
}

func (stx *spannerTransactionAdapter) updateSegmentOffsets(ctx context.Context, streamID uuid.UUID, updates []segmentToCommit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(updates) == 0 {
		return nil
	}

	// When none of the segments have changed, then the update will be skipped.

	// Update plain offsets of the segments.
	var batch []spanner.Statement
	expectedOffset := int64(0)
	for _, u := range updates {
		if u.OldPlainOffset != expectedOffset {
			batch = append(batch, spanner.Statement{
				SQL: `
					UPDATE segments SET plain_offset = @plain_offset
					WHERE stream_id = @stream_id and position = @position
				`,
				Params: map[string]interface{}{
					"position":     u.Position,
					"plain_offset": expectedOffset,
					"stream_id":    streamID,
				},
			})
		}
		expectedOffset += int64(u.PlainSize)
	}
	if len(batch) == 0 {
		return nil
	}

	affecteds, err := stx.tx.BatchUpdate(ctx, batch)
	if err != nil {
		return Error.New("unable to update segments offsets: %w", err)
	}
	sumAffected := int64(0)
	for _, affected := range affecteds {
		sumAffected += affected
	}
	if sumAffected != int64(len(batch)) {
		return Error.New("not all segments were updated, expected %d got %d", len(batch), sumAffected)
	}

	return nil
}

// deleteSegmentsNotInCommit deletes the listed segments inside the tx.
func (ptx *postgresTransactionAdapter) deleteSegmentsNotInCommit(ctx context.Context, streamID uuid.UUID, segments []SegmentPosition) (deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(segments) == 0 {
		return 0, nil
	}

	positions := []int64{}
	for _, p := range segments {
		positions = append(positions, int64(p.Encode()))
	}

	// This potentially could be done together with the previous database call.
	result, err := ptx.tx.ExecContext(ctx, `
		DELETE FROM segments
		WHERE stream_id = $1 AND position = ANY($2)
	`, streamID, pgutil.Int8Array(positions))
	if err != nil {
		return 0, Error.New("unable to delete segments: %w", err)
	}

	deletedCount, err := result.RowsAffected()
	if err != nil {
		return 0, Error.New("unable to count deleted segments: %w", err)
	}

	return deletedCount, nil
}

func (stx *spannerTransactionAdapter) deleteSegmentsNotInCommit(ctx context.Context, streamID uuid.UUID, segments []SegmentPosition) (deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(segments) == 0 {
		return 0, nil
	}

	stmts := make([]spanner.Statement, len(segments))
	for ix, segment := range segments {
		stmts[ix] = spanner.Statement{
			SQL: `DELETE FROM segments WHERE stream_id = @stream_id AND position = @position`,
			Params: map[string]interface{}{
				"stream_id": streamID,
				"position":  int64(segment.Encode()),
			},
		}
	}

	if len(stmts) > 0 {
		deleted, err := stx.tx.BatchUpdate(ctx, stmts)
		if err != nil {
			return 0, Error.New("unable to delete segments: %w", err)
		}
		for _, v := range deleted {
			deletedSegmentCount += v
		}
	}
	return deletedSegmentCount, nil
}

// diffSegmentsWithDatabase matches up segment positions with their database information.
func diffSegmentsWithDatabase(as []SegmentPosition, bs []segmentInfoForCommit, cb func(a *SegmentPosition, b *segmentInfoForCommit)) {
	for len(as) > 0 && len(bs) > 0 {
		if as[0] == bs[0].Position {
			cb(&as[0], &bs[0])
			as, bs = as[1:], bs[1:]
		} else if as[0].Less(bs[0].Position) {
			cb(&as[0], nil)
			as = as[1:]
		} else {
			cb(nil, &bs[0])
			bs = bs[1:]
		}
	}
	for i := range as {
		cb(&as[i], nil)
	}
	for i := range bs {
		cb(nil, &bs[i])
	}
}
