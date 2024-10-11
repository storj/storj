// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cloud.google.com/go/spanner"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

const noLockOnUnversionedErrMsg = "Object Lock settings must not be placed on unversioned objects"

// lockInfo contains Object Lock-related information about the object
// that's being moved.
type lockInfo struct {
	objectExpiresAt *time.Time
	retention       Retention
	legalHold       bool
}

type moveObjectTransactionAdapter interface {
	objectMove(ctx context.Context, opts FinishMoveObject, newStatus ObjectStatus, nextVersion Version) (oldStatus ObjectStatus, segmentsCount int, hasMetadata bool, streamID uuid.UUID, info lockInfo, err error)
	objectMoveEncryption(ctx context.Context, opts FinishMoveObject, positions []int64, encryptedKeys [][]byte, encryptedKeyNonces [][]byte) (numAffected int64, err error)
}

// BeginMoveObjectResult holds data needed to begin move object.
type BeginMoveObjectResult BeginMoveCopyResults

// EncryptedKeyAndNonce holds single segment position, encrypted key and nonce.
type EncryptedKeyAndNonce struct {
	Position          SegmentPosition
	EncryptedKeyNonce []byte
	EncryptedKey      []byte
}

// BeginMoveObject holds all data needed begin move object method.
type BeginMoveObject struct {
	ObjectLocation
}

// BeginMoveCopyResults holds all data needed to begin move and copy object methods.
type BeginMoveCopyResults struct {
	StreamID                  uuid.UUID
	Version                   Version
	EncryptedMetadata         []byte
	EncryptedMetadataKeyNonce []byte
	EncryptedMetadataKey      []byte
	EncryptedKeysNonces       []EncryptedKeyAndNonce
	EncryptionParameters      storj.EncryptionParameters
}

// BeginMoveObject collects all data needed to begin object move procedure.
func (db *DB) BeginMoveObject(ctx context.Context, opts BeginMoveObject) (_ BeginMoveObjectResult, err error) {
	// TODO(ver) add support specifying move source object version
	result, err := db.beginMoveCopyObject(ctx, opts.ObjectLocation, 0, MoveSegmentLimit, nil)
	if err != nil {
		return BeginMoveObjectResult{}, err
	}

	return BeginMoveObjectResult(result), nil
}

// beginMoveCopyObject collects all data needed to begin object move/copy procedure.
func (db *DB) beginMoveCopyObject(ctx context.Context, location ObjectLocation, version Version, segmentLimit int64, verifyLimits func(encryptedObjectSize int64, nSegments int64) error) (result BeginMoveCopyResults, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := location.Verify(); err != nil {
		return BeginMoveCopyResults{}, err
	}

	var object Object
	if version > 0 {
		object, err = db.GetObjectExactVersion(ctx, GetObjectExactVersion{
			ObjectLocation: location,
			Version:        version,
		})
	} else {
		object, err = db.GetObjectLastCommitted(ctx, GetObjectLastCommitted{
			ObjectLocation: location,
		})
	}
	if err != nil {
		return BeginMoveCopyResults{}, err
	}

	if object.Status.IsDeleteMarker() {
		return BeginMoveCopyResults{}, ErrObjectNotFound.New("")
	}

	if int64(object.SegmentCount) > segmentLimit {
		return BeginMoveCopyResults{}, ErrInvalidRequest.New("object has too many segments (%d). Limit is %d.", object.SegmentCount, CopySegmentLimit)
	}

	if verifyLimits != nil {
		err = verifyLimits(object.TotalEncryptedSize, int64(object.SegmentCount))
		if err != nil {
			return BeginMoveCopyResults{}, err
		}
	}

	keysNonces, err := db.ChooseAdapter(location.ProjectID).GetSegmentPositionsAndKeys(ctx, object.StreamID)
	if err != nil {
		return BeginMoveCopyResults{}, err
	}

	result.EncryptedKeysNonces = keysNonces
	result.StreamID = object.StreamID
	result.Version = object.Version
	result.EncryptionParameters = object.Encryption
	result.EncryptedMetadata = object.EncryptedMetadata
	result.EncryptedMetadataKey = object.EncryptedMetadataEncryptedKey
	result.EncryptedMetadataKeyNonce = object.EncryptedMetadataNonce

	return result, nil
}

// GetSegmentPositionsAndKeys fetches the Position, EncryptedKeyNonce, and EncryptedKey for all
// segments in the db for the given stream ID, ordered by position.
func (p *PostgresAdapter) GetSegmentPositionsAndKeys(ctx context.Context, streamID uuid.UUID) (keysNonces []EncryptedKeyAndNonce, err error) {
	err = withRows(p.db.QueryContext(ctx, `
		SELECT
			position, encrypted_key_nonce, encrypted_key
		FROM segments
		WHERE stream_id = $1
		ORDER BY stream_id, position ASC
	`, streamID))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var keys EncryptedKeyAndNonce

			err = rows.Scan(&keys.Position, &keys.EncryptedKeyNonce, &keys.EncryptedKey)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			keysNonces = append(keysNonces, keys)
		}
		return nil
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, Error.New("unable to fetch object segments: %w", err)
	}
	return keysNonces, nil
}

// GetSegmentPositionsAndKeys fetches the Position, EncryptedKeyNonce, and EncryptedKey for all
// segments in the db for the given stream ID, ordered by position.
func (s *SpannerAdapter) GetSegmentPositionsAndKeys(ctx context.Context, streamID uuid.UUID) (keysNonces []EncryptedKeyAndNonce, err error) {
	keysNonces, err = spannerutil.CollectRows(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				position, encrypted_key_nonce, encrypted_key
			FROM segments
			WHERE stream_id = @stream_id
			ORDER BY stream_id, position ASC
		`,
		Params: map[string]interface{}{
			"stream_id": streamID,
		},
	}), func(row *spanner.Row, keys *EncryptedKeyAndNonce) error {
		err := row.Columns(&keys.Position, &keys.EncryptedKeyNonce, &keys.EncryptedKey)
		if err != nil {
			return Error.New("failed to scan segments: %w", err)
		}
		return nil
	})
	return keysNonces, Error.Wrap(err)
}

// FinishMoveObject holds all data needed to finish object move.
type FinishMoveObject struct {
	ObjectStream

	NewBucket             BucketName
	NewSegmentKeys        []EncryptedKeyAndNonce
	NewEncryptedObjectKey ObjectKey
	// Optional. Required if object has metadata.
	NewEncryptedMetadataKeyNonce storj.Nonce
	NewEncryptedMetadataKey      []byte

	// NewDisallowDelete indicates whether the user is allowed to delete an existing unversioned object.
	NewDisallowDelete bool

	// NewVersioned indicates that the object allows multiple versions.
	NewVersioned bool

	// Retention indicates retention settings of the moved object
	// version.
	Retention Retention
	// LegalHold indicates legal hold settings of the moved object
	// version.
	LegalHold bool
}

// NewLocation returns the new object location.
func (finishMove FinishMoveObject) NewLocation() ObjectLocation {
	return ObjectLocation{
		ProjectID:  finishMove.ProjectID,
		BucketName: finishMove.NewBucket,
		ObjectKey:  finishMove.NewEncryptedObjectKey,
	}
}

// Verify verifies metabase.FinishMoveObject data.
func (finishMove FinishMoveObject) Verify() error {
	if err := finishMove.ObjectStream.Verify(); err != nil {
		return err
	}

	switch {
	case len(finishMove.NewBucket) == 0:
		return ErrInvalidRequest.New("NewBucket is missing")
	case len(finishMove.NewEncryptedObjectKey) == 0:
		return ErrInvalidRequest.New("NewEncryptedObjectKey is missing")
	}

	if !finishMove.NewVersioned && (finishMove.Retention.Enabled() || finishMove.LegalHold) {
		return ErrObjectStatus.New(noLockOnUnversionedErrMsg)
	}

	return ErrInvalidRequest.Wrap(finishMove.Retention.Verify())
}

// FinishMoveObject accepts new encryption keys for moved object and updates the corresponding object ObjectKey and segments EncryptedKey.
func (db *DB) FinishMoveObject(ctx context.Context, opts FinishMoveObject) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	var precommit PrecommitConstraintResult
	err = db.ChooseAdapter(opts.ProjectID).WithTx(ctx, func(ctx context.Context, adapter TransactionAdapter) error {
		precommit, err = db.PrecommitConstraint(ctx, PrecommitConstraint{
			Location:       opts.NewLocation(),
			Versioned:      opts.NewVersioned,
			DisallowDelete: opts.NewDisallowDelete,
		}, adapter)
		if err != nil {
			return err
		}

		newStatus := committedWhereVersioned(opts.NewVersioned)
		nextVersion := precommit.HighestVersion + 1

		oldStatus, segmentsCount, hasMetadata, streamID, lockInfo, err := adapter.objectMove(ctx, opts, newStatus, nextVersion)
		if err != nil {
			// purposefully not wrapping the error here, so as not to break expected error text in tests
			return err
		}
		if streamID != opts.StreamID {
			return ErrObjectNotFound.New("object was changed during move")
		}
		if segmentsCount != len(opts.NewSegmentKeys) {
			return ErrInvalidRequest.New("wrong number of segments keys received")
		}
		if oldStatus.IsDeleteMarker() {
			return ErrMethodNotAllowed.New("moving delete marker is not allowed")
		}
		if hasMetadata {
			switch {
			case opts.NewEncryptedMetadataKeyNonce.IsZero() && len(opts.NewEncryptedMetadataKey) != 0:
				return ErrInvalidRequest.New("EncryptedMetadataKeyNonce is missing")
			case len(opts.NewEncryptedMetadataKey) == 0 && !opts.NewEncryptedMetadataKeyNonce.IsZero():
				return ErrInvalidRequest.New("EncryptedMetadataKey is missing")
			}
		}
		if lockInfo.retention.ActiveNow() {
			return ErrObjectLock.New(retentionErrMsg)
		}
		if lockInfo.legalHold {
			return ErrObjectLock.New(legalHoldErrMsg)
		}
		if lockInfo.objectExpiresAt != nil && (opts.Retention.Enabled() || opts.LegalHold) {
			return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
		}

		var (
			positions          []int64
			encryptedKeys      [][]byte
			encryptedKeyNonces [][]byte
		)

		for _, u := range opts.NewSegmentKeys {
			encryptedKeys = append(encryptedKeys, u.EncryptedKey)
			encryptedKeyNonces = append(encryptedKeyNonces, u.EncryptedKeyNonce)
			positions = append(positions, int64(u.Position.Encode()))
		}

		affected, err := adapter.objectMoveEncryption(ctx, opts, positions, encryptedKeys, encryptedKeyNonces)
		if err != nil {
			return Error.New("failed to get rows affected: %w", err)
		}

		if affected != int64(len(positions)) {
			return Error.New("segment is missing")
		}
		return nil
	})
	if err != nil {
		return err
	}

	precommit.submitMetrics()
	mon.Meter("finish_move_object").Mark(1)

	return nil
}

func (ptx *postgresTransactionAdapter) objectMove(ctx context.Context, opts FinishMoveObject, newStatus ObjectStatus, nextVersion Version) (oldStatus ObjectStatus, segmentsCount int, hasMetadata bool, streamID uuid.UUID, info lockInfo, err error) {
	err = ptx.tx.QueryRowContext(ctx, `
			WITH
			new AS (
				UPDATE objects SET
					bucket_name = $1,
					object_key = $2,
					version = $10,
					status = $9,
					encrypted_metadata_encrypted_key =
						CASE WHEN encrypted_metadata IS NOT NULL
							THEN $3
							ELSE encrypted_metadata_encrypted_key
						END,
					encrypted_metadata_nonce =
						CASE WHEN encrypted_metadata IS NOT NULL
							THEN $4
							ELSE encrypted_metadata_nonce
						END,
					retention_mode = $11,
					retain_until = $12
				WHERE
					(project_id, bucket_name, object_key, version) = ($5, $6, $7, $8)
				RETURNING
					segment_count,
					encrypted_metadata IS NOT NULL AND LENGTH(encrypted_metadata) > 0 AS has_metadata,
					stream_id
			),
			old AS (
    			SELECT status, expires_at, retention_mode, retain_until
    			FROM objects
    			WHERE (project_id, bucket_name, object_key, version) = ($5, $6, $7, $8)
			)
				SELECT
					old.status,
					new.segment_count,
					new.has_metadata,
					new.stream_id,
					old.expires_at,
					old.retention_mode,
					old.retain_until
				FROM old, new;
		`,
		opts.NewBucket,
		opts.NewEncryptedObjectKey,
		opts.NewEncryptedMetadataKey,
		opts.NewEncryptedMetadataKeyNonce,
		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
		newStatus,
		nextVersion,
		lockModeWrapper{retentionMode: &opts.Retention.Mode, legalHold: &opts.LegalHold},
		timeWrapper{&opts.Retention.RetainUntil},
	).Scan(
		&oldStatus,
		&segmentsCount,
		&hasMetadata,
		&streamID,
		&info.objectExpiresAt,
		lockModeWrapper{retentionMode: &info.retention.Mode, legalHold: &info.legalHold},
		timeWrapper{&info.retention.RetainUntil},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, false, uuid.UUID{}, lockInfo{}, ErrObjectNotFound.New("object not found")
		}
		return 0, 0, false, uuid.UUID{}, lockInfo{}, Error.New("unable to update object: %w", err)
	}
	return oldStatus, segmentsCount, hasMetadata, streamID, info, nil
}

func (stx *spannerTransactionAdapter) objectMove(ctx context.Context, opts FinishMoveObject, newStatus ObjectStatus, nextVersion Version) (oldStatus ObjectStatus, segmentsCount int, hasMetadata bool, streamID uuid.UUID, info lockInfo, err error) {
	// We cannot UPDATE the object record in place, because some of the columns we need to update are
	// part of the primary key. We must DELETE and INSERT instead.

	// TODO(spanner): check whether INSERT FROM and then DELETE would be more performant, because
	// it will use a single round trip, instead of two.

	var (
		found                         bool
		createdAt                     time.Time
		expiresAt                     *time.Time
		segmentCount                  int64
		encryptedMetadataNonce        []byte
		encryptedMetadata             []byte
		encryptedMetadataEncryptedKey []byte
		totalPlainSize                int64
		totalEncryptedSize            int64
		fixedSegmentSize              int64
		encryption                    storj.EncryptionParameters
		zombieDeletionDeadline        *time.Time
	)

	err = stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			DELETE FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
			THEN RETURN
				stream_id, created_at, expires_at, status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption,
				zombie_deletion_deadline,
				retention_mode, retain_until
		`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
			"version":     opts.Version,
		},
	}).Do(func(row *spanner.Row) error {
		found = true
		err := row.Columns(
			&streamID, &createdAt, &expiresAt, &oldStatus, &segmentCount,
			&encryptedMetadataNonce, &encryptedMetadata, &encryptedMetadataEncryptedKey,
			&totalPlainSize, &totalEncryptedSize, &fixedSegmentSize,
			encryptionParameters{&encryption},
			&zombieDeletionDeadline,
			lockModeWrapper{retentionMode: &info.retention.Mode, legalHold: &info.legalHold},
			timeWrapper{&info.retention.RetainUntil},
		)
		if err != nil {
			return Error.New("unable to read old object record: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, 0, false, uuid.UUID{}, lockInfo{}, Error.New("unable to remove old object record: %w", err)
	}
	if !found {
		return 0, 0, false, uuid.UUID{}, lockInfo{}, ErrObjectNotFound.New("object not found")
	}

	info.objectExpiresAt = expiresAt

	segmentsCount = int(segmentCount)

	if encryptedMetadata != nil {
		encryptedMetadataEncryptedKey = opts.NewEncryptedMetadataKey
		encryptedMetadataNonce = opts.NewEncryptedMetadataKeyNonce[:]
	}

	_, err = stx.tx.Update(ctx, spanner.Statement{
		SQL: `
			INSERT INTO objects (
			    project_id, bucket_name, object_key, version,
				stream_id, created_at, expires_at, status, segment_count,
			    encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption,
				zombie_deletion_deadline,
				retention_mode, retain_until
			) VALUES (
			    @project_id, @bucket_name, @object_key, @version,
				@stream_id, @created_at, @expires_at, @status, @segment_count,
			    @encrypted_metadata_nonce, @encrypted_metadata, @encrypted_metadata_encrypted_key,
				@total_plain_size, @total_encrypted_size, @fixed_segment_size,
				@encryption,
				@zombie_deletion_deadline,
				@retention_mode, @retain_until
			)
		`,
		Params: map[string]interface{}{
			"project_id":                       opts.ProjectID,
			"bucket_name":                      opts.NewBucket,
			"object_key":                       opts.NewEncryptedObjectKey,
			"version":                          nextVersion,
			"stream_id":                        streamID,
			"created_at":                       createdAt,
			"expires_at":                       expiresAt,
			"status":                           newStatus,
			"segment_count":                    segmentsCount,
			"encrypted_metadata_nonce":         encryptedMetadataNonce,
			"encrypted_metadata":               encryptedMetadata,
			"encrypted_metadata_encrypted_key": encryptedMetadataEncryptedKey,
			"total_plain_size":                 totalPlainSize,
			"total_encrypted_size":             totalEncryptedSize,
			"fixed_segment_size":               fixedSegmentSize,
			"encryption":                       encryptionParameters{&encryption},
			"zombie_deletion_deadline":         zombieDeletionDeadline,
			"retention_mode":                   lockModeWrapper{retentionMode: &opts.Retention.Mode, legalHold: &opts.LegalHold},
			"retain_until":                     timeWrapper{&opts.Retention.RetainUntil},
		},
	})
	if err != nil {
		return 0, 0, false, uuid.UUID{}, lockInfo{}, Error.New("unable to create new object record: %w", err)
	}

	return oldStatus, segmentsCount, len(encryptedMetadata) > 0, streamID, info, nil
}

func (ptx *postgresTransactionAdapter) objectMoveEncryption(ctx context.Context, opts FinishMoveObject, positions []int64, encryptedKeys [][]byte, encryptedKeyNonces [][]byte) (numAffected int64, err error) {
	updateResult, err := ptx.tx.ExecContext(ctx, `
			UPDATE segments SET
				encrypted_key_nonce = P.encrypted_key_nonce,
				encrypted_key = P.encrypted_key
			FROM (SELECT unnest($2::INT8[]), unnest($3::BYTEA[]), unnest($4::BYTEA[])) as P(position, encrypted_key_nonce, encrypted_key)
			WHERE
				stream_id = $1 AND
				segments.position = P.position
		`, opts.StreamID, pgutil.Int8Array(positions), pgutil.ByteaArray(encryptedKeyNonces), pgutil.ByteaArray(encryptedKeys))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrObjectNotFound.New("object not found")
		}
		return 0, Error.Wrap(err)
	}

	return updateResult.RowsAffected()
}

func (stx *spannerTransactionAdapter) objectMoveEncryption(ctx context.Context, opts FinishMoveObject, positions []int64, encryptedKeys [][]byte, encryptedKeyNonces [][]byte) (numAffected int64, err error) {
	if len(positions) == 0 {
		return 0, nil
	}

	stmts := make([]spanner.Statement, 0, len(positions))
	for i := range positions {
		stmts = append(stmts, spanner.Statement{
			SQL: `
				UPDATE segments SET
					encrypted_key_nonce = COALESCE(@encrypted_key_nonce, B''),
					encrypted_key = COALESCE(@encrypted_key, B'')
				WHERE
					stream_id = @stream_id
					AND position = @position
			`,
			Params: map[string]interface{}{
				"stream_id":           opts.StreamID,
				"position":            positions[i],
				"encrypted_key_nonce": encryptedKeyNonces[i],
				"encrypted_key":       encryptedKeys[i],
			},
		})
	}
	affecteds, err := stx.tx.BatchUpdate(ctx, stmts)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	var totalFound int64
	for _, affected := range affecteds {
		totalFound += affected
	}
	return totalFound, nil
}
