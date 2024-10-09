// Copyright (C) 2022 Storj Labs, Inc.
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

type copyObjectTransactionAdapter interface {
	getSegmentsForCopy(ctx context.Context, object Object) (segments transposedSegmentList, err error)
	finalizeObjectCopy(ctx context.Context, opts FinishCopyObject, nextVersion Version, newStatus ObjectStatus, sourceObject Object, copyMetadata []byte, newSegments transposedSegmentList) (newObject Object, err error)
	getObjectNonPendingExactVersion(ctx context.Context, opts FinishCopyObject) (_ Object, err error)
}

// BeginCopyObjectResult holds data needed to begin copy object.
type BeginCopyObjectResult BeginMoveCopyResults

// BeginCopyObject holds all data needed begin copy object method.
type BeginCopyObject struct {
	ObjectLocation
	Version Version

	// VerifyLimits holds a callback by which the caller can interrupt the copy
	// if it turns out the copy would exceed a limit.
	VerifyLimits func(encryptedObjectSize int64, nSegments int64) error
}

// BeginCopyObject collects all data needed to begin object copy procedure.
func (db *DB) BeginCopyObject(ctx context.Context, opts BeginCopyObject) (_ BeginCopyObjectResult, err error) {
	result, err := db.beginMoveCopyObject(ctx, opts.ObjectLocation, opts.Version, CopySegmentLimit, opts.VerifyLimits)
	if err != nil {
		return BeginCopyObjectResult{}, err
	}

	return BeginCopyObjectResult(result), nil
}

// FinishCopyObject holds all data needed to finish object copy.
type FinishCopyObject struct {
	ObjectStream
	NewBucket             BucketName
	NewEncryptedObjectKey ObjectKey
	NewStreamID           uuid.UUID

	OverrideMetadata             bool
	NewEncryptedMetadata         []byte
	NewEncryptedMetadataKeyNonce storj.Nonce
	NewEncryptedMetadataKey      []byte

	NewSegmentKeys []EncryptedKeyAndNonce

	// NewDisallowDelete indicates whether the user is allowed to delete an existing unversioned object.
	NewDisallowDelete bool

	// NewVersioned indicates that the object allows multiple versions.
	NewVersioned bool

	// Retention indicates retention settings of the object copy.
	Retention Retention
	// LegalHold indicates whether the object copy is under legal hold.
	LegalHold bool

	// VerifyLimits holds a callback by which the caller can interrupt the copy
	// if it turns out completing the copy would exceed a limit.
	// It will be called only once.
	VerifyLimits func(encryptedObjectSize int64, nSegments int64) error
}

// NewLocation returns the new object location.
func (finishCopy FinishCopyObject) NewLocation() ObjectLocation {
	return ObjectLocation{
		ProjectID:  finishCopy.ProjectID,
		BucketName: finishCopy.NewBucket,
		ObjectKey:  finishCopy.NewEncryptedObjectKey,
	}
}

// Verify verifies metabase.FinishCopyObject data.
func (finishCopy FinishCopyObject) Verify() error {
	if err := finishCopy.ObjectStream.Verify(); err != nil {
		return err
	}

	switch {
	case len(finishCopy.NewBucket) == 0:
		return ErrInvalidRequest.New("NewBucket is missing")
	case finishCopy.NewStreamID.IsZero():
		return ErrInvalidRequest.New("NewStreamID is missing")
	case finishCopy.ObjectStream.StreamID == finishCopy.NewStreamID:
		return ErrInvalidRequest.New("StreamIDs are identical")
	case len(finishCopy.NewEncryptedObjectKey) == 0:
		return ErrInvalidRequest.New("NewEncryptedObjectKey is missing")
	}

	if finishCopy.OverrideMetadata {
		if finishCopy.NewEncryptedMetadata == nil && (!finishCopy.NewEncryptedMetadataKeyNonce.IsZero() || finishCopy.NewEncryptedMetadataKey != nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set")
		} else if finishCopy.NewEncryptedMetadata != nil && (finishCopy.NewEncryptedMetadataKeyNonce.IsZero() || finishCopy.NewEncryptedMetadataKey == nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set")
		}
	} else {
		switch {
		case finishCopy.NewEncryptedMetadataKeyNonce.IsZero() && len(finishCopy.NewEncryptedMetadataKey) != 0:
			return ErrInvalidRequest.New("EncryptedMetadataKeyNonce is missing")
		case len(finishCopy.NewEncryptedMetadataKey) == 0 && !finishCopy.NewEncryptedMetadataKeyNonce.IsZero():
			return ErrInvalidRequest.New("EncryptedMetadataKey is missing")
		}
	}

	if !finishCopy.NewVersioned && (finishCopy.Retention.Enabled() || finishCopy.LegalHold) {
		return ErrObjectStatus.New(noLockOnUnversionedErrMsg)
	}

	return ErrInvalidRequest.Wrap(finishCopy.Retention.Verify())
}

type transposedSegmentList struct {
	Positions []int64

	CreatedAts  []time.Time // non-nillable
	RepairedAts []*time.Time
	ExpiresAts  []*time.Time

	RootPieceIDs       [][]byte
	EncryptedKeyNonces [][]byte
	EncryptedKeys      [][]byte

	EncryptedSizes []int32 // sizes of the whole segments (not pieces)
	// PlainSizes holds 0 for migrated objects.
	PlainSizes []int32
	// PlainOffsets holds 0 for a migrated object.
	PlainOffsets   []int64
	EncryptedETags [][]byte

	RedundancySchemes []int64

	InlineDatas [][]byte
	PiecesLists [][]byte

	Placements []storj.PlacementConstraint
}

// FinishCopyObject accepts new encryption keys for copied object and insert the corresponding new object ObjectKey and segments EncryptedKey.
// It returns the object at the destination location.
func (db *DB) FinishCopyObject(ctx context.Context, opts FinishCopyObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	newObject := Object{}
	var copyMetadata []byte

	var precommit PrecommitConstraintResult
	err = db.ChooseAdapter(opts.ProjectID).WithTx(ctx, func(ctx context.Context, adapter TransactionAdapter) error {
		sourceObject, err := adapter.getObjectNonPendingExactVersion(ctx, opts)
		if err != nil {
			if ErrObjectNotFound.Has(err) {
				return ErrObjectNotFound.New("source object not found")
			}
			return err
		}
		if sourceObject.StreamID != opts.StreamID {
			return ErrObjectNotFound.New("object was changed during copy")
		}
		if sourceObject.Status.IsDeleteMarker() {
			return ErrMethodNotAllowed.New("copying delete marker is not allowed")
		}

		if opts.VerifyLimits != nil {
			err := opts.VerifyLimits(sourceObject.TotalEncryptedSize, int64(sourceObject.SegmentCount))
			if err != nil {
				return err
			}
		}

		if int(sourceObject.SegmentCount) != len(opts.NewSegmentKeys) {
			return ErrInvalidRequest.New("wrong number of segments keys received (received %d, need %d)", len(opts.NewSegmentKeys), sourceObject.SegmentCount)
		}

		newSegments, err := adapter.getSegmentsForCopy(ctx, sourceObject)
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}

		if err = checkExpiresAtWithObjectLock(sourceObject, newSegments, opts.Retention, opts.LegalHold); err != nil {
			return err
		}

		newSegments.EncryptedKeys = make([][]byte, len(opts.NewSegmentKeys))
		newSegments.EncryptedKeyNonces = make([][]byte, len(opts.NewSegmentKeys))
		for index, u := range opts.NewSegmentKeys {
			if int64(u.Position.Encode()) != newSegments.Positions[index] {
				return Error.New("missing new segment keys for segment %d", newSegments.Positions[index])
			}
			newSegments.EncryptedKeys[index] = u.EncryptedKey
			newSegments.EncryptedKeyNonces[index] = u.EncryptedKeyNonce
		}

		if opts.OverrideMetadata {
			copyMetadata = opts.NewEncryptedMetadata
		} else {
			copyMetadata = sourceObject.EncryptedMetadata
		}

		precommit, err = db.PrecommitConstraint(ctx, PrecommitConstraint{
			Location:       opts.NewLocation(),
			Versioned:      opts.NewVersioned,
			DisallowDelete: opts.NewDisallowDelete,
		}, adapter)
		if err != nil {
			return err
		}

		newStatus := committedWhereVersioned(opts.NewVersioned)

		newObject, err = adapter.finalizeObjectCopy(ctx, opts, precommit.HighestVersion+1, newStatus, sourceObject, copyMetadata, newSegments)
		return err
	})

	if err != nil {
		return Object{}, err
	}

	newObject.StreamID = opts.NewStreamID
	newObject.BucketName = opts.NewBucket
	newObject.ObjectKey = opts.NewEncryptedObjectKey
	newObject.EncryptedMetadata = copyMetadata
	newObject.EncryptedMetadataEncryptedKey = opts.NewEncryptedMetadataKey
	if !opts.NewEncryptedMetadataKeyNonce.IsZero() {
		newObject.EncryptedMetadataNonce = opts.NewEncryptedMetadataKeyNonce[:]
	}
	newObject.Retention = opts.Retention
	newObject.LegalHold = opts.LegalHold

	precommit.submitMetrics()
	mon.Meter("finish_copy_object").Mark(1)

	return newObject, nil
}

func (ptx *postgresTransactionAdapter) getSegmentsForCopy(ctx context.Context, sourceObject Object) (segments transposedSegmentList, err error) {
	segments.Positions = make([]int64, sourceObject.SegmentCount)

	segments.RootPieceIDs = make([][]byte, sourceObject.SegmentCount)

	segments.ExpiresAts = make([]*time.Time, sourceObject.SegmentCount)
	segments.EncryptedSizes = make([]int32, sourceObject.SegmentCount)
	segments.PlainSizes = make([]int32, sourceObject.SegmentCount)
	segments.PlainOffsets = make([]int64, sourceObject.SegmentCount)
	segments.InlineDatas = make([][]byte, sourceObject.SegmentCount)
	segments.Placements = make([]storj.PlacementConstraint, sourceObject.SegmentCount)
	segments.PiecesLists = make([][]byte, sourceObject.SegmentCount)

	segments.RedundancySchemes = make([]int64, sourceObject.SegmentCount)

	err = withRows(ptx.tx.QueryContext(ctx, `
				SELECT
					position,
					expires_at,
					root_piece_id,
					encrypted_size, plain_offset, plain_size,
					redundancy,
					remote_alias_pieces,
					placement,
					inline_data
				FROM segments
				WHERE stream_id = $1
				ORDER BY position ASC
				LIMIT  $2
			`, sourceObject.StreamID, sourceObject.SegmentCount))(func(rows tagsql.Rows) error {
		index := 0
		for rows.Next() {
			err := rows.Scan(
				&segments.Positions[index],
				&segments.ExpiresAts[index],
				&segments.RootPieceIDs[index],
				&segments.EncryptedSizes[index], &segments.PlainOffsets[index], &segments.PlainSizes[index],
				&segments.RedundancySchemes[index],
				&segments.PiecesLists[index],
				&segments.Placements[index],
				&segments.InlineDatas[index],
			)
			if err != nil {
				return err
			}
			index++
		}

		if err := rows.Err(); err != nil {
			return err
		}

		if index != int(sourceObject.SegmentCount) {
			return Error.New("could not load all of the segment information")
		}

		return nil
	})
	return segments, err
}

func (stx *spannerTransactionAdapter) getSegmentsForCopy(ctx context.Context, sourceObject Object) (segments transposedSegmentList, err error) {
	segments.Positions = make([]int64, sourceObject.SegmentCount)

	segments.RootPieceIDs = make([][]byte, sourceObject.SegmentCount)

	segments.ExpiresAts = make([]*time.Time, sourceObject.SegmentCount)
	segments.EncryptedSizes = make([]int32, sourceObject.SegmentCount)
	segments.PlainSizes = make([]int32, sourceObject.SegmentCount)
	segments.PlainOffsets = make([]int64, sourceObject.SegmentCount)
	segments.InlineDatas = make([][]byte, sourceObject.SegmentCount)
	segments.Placements = make([]storj.PlacementConstraint, sourceObject.SegmentCount)
	segments.PiecesLists = make([][]byte, sourceObject.SegmentCount)

	segments.RedundancySchemes = make([]int64, sourceObject.SegmentCount)

	index := 0
	err = stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				position,
				expires_at,
				root_piece_id,
				encrypted_size, plain_offset, plain_size,
				redundancy,
				remote_alias_pieces,
				placement,
				COALESCE(inline_data, B'') AS inline_data
			FROM segments
			WHERE stream_id = @stream_id
			ORDER BY position ASC
			LIMIT @segment_count
		`,
		Params: map[string]interface{}{
			"stream_id":     sourceObject.StreamID,
			"segment_count": int64(sourceObject.SegmentCount),
		},
	}).Do(func(row *spanner.Row) error {
		err := row.Columns(
			&segments.Positions[index],
			&segments.ExpiresAts[index],
			&segments.RootPieceIDs[index],
			spannerutil.Int(&segments.EncryptedSizes[index]), &segments.PlainOffsets[index], spannerutil.Int(&segments.PlainSizes[index]),
			&segments.RedundancySchemes[index],
			&segments.PiecesLists[index],
			&segments.Placements[index],
			&segments.InlineDatas[index],
		)
		if err != nil {
			return Error.New("could not read segments for copy: %w", err)
		}
		index++
		return nil
	})

	if err != nil {
		return transposedSegmentList{}, Error.New("could not load segments for copy: %w", err)
	}

	if index != int(sourceObject.SegmentCount) {
		return transposedSegmentList{}, Error.New("could not load all of the segment information (%d != %d)", index, sourceObject.SegmentCount)
	}

	return segments, err
}

func (ptx *postgresTransactionAdapter) finalizeObjectCopy(ctx context.Context, opts FinishCopyObject, nextVersion Version, newStatus ObjectStatus, sourceObject Object, copyMetadata []byte, newSegments transposedSegmentList) (newObject Object, err error) {
	// TODO we need to handle metadata correctly (copy from original object or replace)
	row := ptx.tx.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				status, expires_at, segment_count,
				encryption,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				zombie_deletion_deadline,
				retention_mode, retain_until
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8,
				$9,
				$10, $11, $12,
				$13, $14, $15,
				null,
				$16, $17
			)
			RETURNING
				created_at`,
		opts.ProjectID, opts.NewBucket, opts.NewEncryptedObjectKey, nextVersion, opts.NewStreamID,
		newStatus, sourceObject.ExpiresAt, sourceObject.SegmentCount,
		encryptionParameters{&sourceObject.Encryption},
		copyMetadata, opts.NewEncryptedMetadataKeyNonce, opts.NewEncryptedMetadataKey,
		sourceObject.TotalPlainSize, sourceObject.TotalEncryptedSize, sourceObject.FixedSegmentSize,
		lockModeWrapper{retentionMode: &opts.Retention.Mode, legalHold: &opts.LegalHold},
		timeWrapper{&opts.Retention.RetainUntil},
	)

	newObject = sourceObject
	newObject.Version = nextVersion
	newObject.Status = newStatus

	err = row.Scan(&newObject.CreatedAt)
	if err != nil {
		return Object{}, Error.New("unable to copy object: %w", err)
	}

	_, err = ptx.tx.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				encrypted_key_nonce, encrypted_key,
				root_piece_id,
				redundancy,
				encrypted_size, plain_offset, plain_size,
				remote_alias_pieces, placement,
				inline_data
			) SELECT
				$1, UNNEST($2::INT8[]), UNNEST($3::timestamptz[]),
				UNNEST($4::BYTEA[]), UNNEST($5::BYTEA[]),
				UNNEST($6::BYTEA[]),
				UNNEST($7::INT8[]),
				UNNEST($8::INT4[]), UNNEST($9::INT8[]),	UNNEST($10::INT4[]),
				UNNEST($11::BYTEA[]), UNNEST($12::INT2[]),
				UNNEST($13::BYTEA[])
		`, opts.NewStreamID, pgutil.Int8Array(newSegments.Positions), pgutil.NullTimestampTZArray(newSegments.ExpiresAts),
		pgutil.ByteaArray(newSegments.EncryptedKeyNonces), pgutil.ByteaArray(newSegments.EncryptedKeys),
		pgutil.ByteaArray(newSegments.RootPieceIDs),
		pgutil.Int8Array(newSegments.RedundancySchemes),
		pgutil.Int4Array(newSegments.EncryptedSizes), pgutil.Int8Array(newSegments.PlainOffsets), pgutil.Int4Array(newSegments.PlainSizes),
		pgutil.ByteaArray(newSegments.PiecesLists), pgutil.PlacementConstraintArray(newSegments.Placements),
		pgutil.ByteaArray(newSegments.InlineDatas),
	)
	if err != nil {
		return Object{}, Error.New("unable to copy segments: %w", err)
	}

	return newObject, nil
}

func (stx *spannerTransactionAdapter) finalizeObjectCopy(ctx context.Context, opts FinishCopyObject, nextVersion Version, newStatus ObjectStatus, sourceObject Object, copyMetadata []byte, newSegments transposedSegmentList) (newObject Object, err error) {
	// TODO we need to handle metadata correctly (copy from original object or replace)

	newObject = sourceObject

	err = stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				status, expires_at, segment_count,
				encryption,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				zombie_deletion_deadline,
				retention_mode, retain_until
			) VALUES (
				@project_id, @bucket_name, @object_key, @version, @stream_id,
				@status, @expires_at, @segment_count,
				@encryption,
				@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key,
				@total_plain_size, @total_encrypted_size, @fixed_segment_size,
				NULL,
				@retention_mode, @retain_until
			)
			THEN RETURN
				created_at
		`,
		Params: map[string]interface{}{
			"project_id":                       opts.ProjectID,
			"bucket_name":                      opts.NewBucket,
			"object_key":                       opts.NewEncryptedObjectKey,
			"version":                          nextVersion,
			"stream_id":                        opts.NewStreamID,
			"status":                           newStatus,
			"expires_at":                       sourceObject.ExpiresAt,
			"segment_count":                    int64(sourceObject.SegmentCount),
			"encryption":                       encryptionParameters{&sourceObject.Encryption},
			"encrypted_metadata":               copyMetadata,
			"encrypted_metadata_nonce":         opts.NewEncryptedMetadataKeyNonce,
			"encrypted_metadata_encrypted_key": opts.NewEncryptedMetadataKey,
			"total_plain_size":                 sourceObject.TotalPlainSize,
			"total_encrypted_size":             sourceObject.TotalEncryptedSize,
			"fixed_segment_size":               int64(sourceObject.FixedSegmentSize),
			"retention_mode":                   lockModeWrapper{retentionMode: &opts.Retention.Mode, legalHold: &opts.LegalHold},
			"retain_until":                     timeWrapper{&opts.Retention.RetainUntil},
		},
	}).Do(func(row *spanner.Row) error {
		err := row.Columns(&newObject.CreatedAt)
		if err != nil {
			return Error.New("unable to scan created_at: %w", err)
		}
		return nil
	})
	if err != nil {
		return Object{}, Error.New("unable to copy object: %w", err)
	}

	newObject.Version = nextVersion
	newObject.Status = newStatus

	// Warning: these mutations will not be visible inside the transaction! Mutations only take
	// effect when the transaction is closed. As the code is now, this is not a problem, but in
	// case things are rearranged this may become an issue.
	inserts := make([]*spanner.Mutation, len(newSegments.Positions))
	for i := range newSegments.Positions {
		inserts[i] = spanner.Insert("segments",
			[]string{
				"stream_id", "position", "expires_at",
				"encrypted_key_nonce", "encrypted_key",
				"root_piece_id",
				"redundancy",
				"encrypted_size", "plain_offset", "plain_size",
				"remote_alias_pieces", "placement",
				"inline_data",
			}, []any{
				opts.NewStreamID, newSegments.Positions[i], newSegments.ExpiresAts[i],
				newSegments.EncryptedKeyNonces[i], newSegments.EncryptedKeys[i],
				newSegments.RootPieceIDs[i],
				newSegments.RedundancySchemes[i],
				int64(newSegments.EncryptedSizes[i]), newSegments.PlainOffsets[i], int64(newSegments.PlainSizes[i]),
				newSegments.PiecesLists[i], int64(newSegments.Placements[i]),
				newSegments.InlineDatas[i],
			},
		)
	}
	err = stx.tx.BufferWrite(inserts)
	if err != nil {
		return Object{}, Error.New("unable to copy segments: %w", err)
	}

	return newObject, nil
}

// getObjectNonPendingExactVersion returns object information for exact version.
//
// Note: this returns both committed objects and delete markers.
func (ptx *postgresTransactionAdapter) getObjectNonPendingExactVersion(ctx context.Context, opts FinishCopyObject) (_ Object, err error) {
	defer mon.Task()(&ctx)(&err)

	object := Object{}
	err = ptx.tx.QueryRowContext(ctx, `
		SELECT
			stream_id, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE
			(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4) AND
			status <> `+statusPending+` AND
			(expires_at IS NULL OR expires_at > now())`,
		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version).
		Scan(
			&object.StreamID, &object.Status,
			&object.CreatedAt, &object.ExpiresAt,
			&object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Object{}, ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return Object{}, Error.New("unable to query object status: %w", err)
	}

	object.ProjectID = opts.ProjectID
	object.BucketName = opts.BucketName
	object.ObjectKey = opts.ObjectKey
	object.Version = opts.Version

	return object, nil
}

func (stx *spannerTransactionAdapter) getObjectNonPendingExactVersion(ctx context.Context, opts FinishCopyObject) (_ Object, err error) {
	defer mon.Task()(&ctx)(&err)

	found := false
	object := Object{}
	err = stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				stream_id, status,
				created_at, expires_at,
				segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption
			FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version) AND
				status <> ` + statusPending + ` AND
				(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
			"version":     opts.Version,
		},
	}).Do(func(row *spanner.Row) error {
		found = true
		err := row.Columns(
			&object.StreamID, &object.Status,
			&object.CreatedAt, &object.ExpiresAt,
			spannerutil.Int(&object.SegmentCount),
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, spannerutil.Int(&object.FixedSegmentSize),
			encryptionParameters{&object.Encryption},
		)
		if err != nil {
			return Error.New("unable to scan object: %w", err)
		}
		return nil
	})
	if err != nil {
		return Object{}, Error.New("unable to query object status: %w", err)
	}
	if !found {
		return Object{}, ErrObjectNotFound.Wrap(Error.New("object does not exist"))
	}

	object.ProjectID = opts.ProjectID
	object.BucketName = opts.BucketName
	object.ObjectKey = opts.ObjectKey
	object.Version = opts.Version

	return object, nil
}

func checkExpiresAtWithObjectLock(object Object, segments transposedSegmentList, retention Retention, legalHold bool) error {
	if !retention.Enabled() && !legalHold {
		return nil
	}
	for _, e := range segments.ExpiresAts {
		if e != nil {
			return ErrObjectExpiration.New(noLockWithExpirationSegmentsErrMsg)
		}
	}
	if object.ExpiresAt != nil && !object.ExpiresAt.IsZero() {
		return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
	}
	return nil
}
