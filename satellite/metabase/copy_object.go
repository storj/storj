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

type copyObjectAdapter interface {
	getSegmentsForCopy(ctx context.Context, object Object) (segments transposedSegmentList, err error)
	getObjectNonPendingExactVersion(ctx context.Context, opts FinishCopyObject) (_ Object, err error)
	finalizeSegmentsCopy(ctx context.Context, opts FinishCopyObject, newSegments transposedSegmentList) (err error)
	insertPendingCopyObject(ctx context.Context, opts FinishCopyObject, sourceObject Object, encryptedUserData EncryptedUserData) (newObject Object, err error)
	deleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error)
}

type copyObjectTransactionAdapter interface {
	commitPendingCopyObject(ctx context.Context, object *Object, highestVersion Version) (err error)
	commitPendingCopyObject2(ctx context.Context, opts commitPendingCopyObject) (err error)
}

// BeginCopyObjectResult holds data needed to begin copy object.
type BeginCopyObjectResult BeginMoveCopyResults

// BeginCopyObject holds all data needed begin copy object method.
type BeginCopyObject struct {
	ObjectLocation
	Version Version

	SegmentLimit int64

	// VerifyLimits holds a callback by which the caller can interrupt the copy
	// if it turns out the copy would exceed a limit.
	VerifyLimits func(encryptedObjectSize int64, nSegments int64) error
}

// BeginCopyObject collects all data needed to begin object copy procedure.
func (db *DB) BeginCopyObject(ctx context.Context, opts BeginCopyObject) (_ BeginCopyObjectResult, err error) {
	result, err := db.beginMoveCopyObject(ctx, opts.ObjectLocation, opts.Version, opts.SegmentLimit, opts.VerifyLimits)
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

	// OverrideMetadata specifies that EncryptedETag and EncryptedMetadata should be changed on the copied object.
	// Otherwise, only EncryptedMetadataNonce and EncryptedMetadataEncryptedKey are changed.
	OverrideMetadata     bool
	NewEncryptedUserData EncryptedUserData

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

	// IfNoneMatch is an optional field for conditional writes.
	IfNoneMatch IfNoneMatch

	// supported only by Spanner.
	TransmitEvent bool
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
		// check whether new metadata is valid
		err := finishCopy.NewEncryptedUserData.Verify()
		if err != nil {
			return err
		}
	} else {
		// otherwise check that we are setting reencrypted keys
		// TODO: this should validate against the database that it matches it
		switch {
		case len(finishCopy.NewEncryptedUserData.EncryptedMetadataNonce) == 0 && len(finishCopy.NewEncryptedUserData.EncryptedMetadataEncryptedKey) != 0:
			return ErrInvalidRequest.New("EncryptedMetadataNonce is missing")
		case len(finishCopy.NewEncryptedUserData.EncryptedMetadataEncryptedKey) == 0 && len(finishCopy.NewEncryptedUserData.EncryptedMetadataNonce) != 0:
			return ErrInvalidRequest.New("EncryptedMetadataEncryptedKey is missing")
		}
	}

	if !finishCopy.NewVersioned && (finishCopy.Retention.Enabled() || finishCopy.LegalHold) {
		return ErrObjectStatus.New(noLockOnUnversionedErrMsg)
	}

	if err := finishCopy.IfNoneMatch.Verify(); err != nil {
		return err
	}

	return ErrInvalidRequest.Wrap(finishCopy.Retention.Verify())
}

type transposedSegmentList struct {
	StreamID uuid.UUID

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

func transposeSegments(segments []*Segment, convertPieces func(Pieces) ([]byte, error)) (transposedSegmentList, error) {
	if len(segments) == 0 {
		return transposedSegmentList{}, nil
	}
	var t transposedSegmentList
	t.StreamID = segments[0].StreamID

	t.Positions = make([]int64, len(segments))
	t.CreatedAts = make([]time.Time, len(segments))
	t.RepairedAts = make([]*time.Time, len(segments))
	t.ExpiresAts = make([]*time.Time, len(segments))
	t.RootPieceIDs = make([][]byte, len(segments))
	t.EncryptedKeyNonces = make([][]byte, len(segments))
	t.EncryptedKeys = make([][]byte, len(segments))
	t.EncryptedSizes = make([]int32, len(segments))
	t.PlainSizes = make([]int32, len(segments))
	t.PlainOffsets = make([]int64, len(segments))
	t.EncryptedETags = make([][]byte, len(segments))
	t.RedundancySchemes = make([]int64, len(segments))
	t.InlineDatas = make([][]byte, len(segments))
	t.PiecesLists = make([][]byte, len(segments))
	t.Placements = make([]storj.PlacementConstraint, len(segments))

	for i, segment := range segments {
		if t.StreamID != segment.StreamID {
			return t, Error.New("inconsistent segments")
		}

		t.Positions[i] = int64(segment.Position.Encode())
		t.CreatedAts[i] = segment.CreatedAt
		t.RepairedAts[i] = segment.RepairedAt
		t.ExpiresAts[i] = segment.ExpiresAt
		t.RootPieceIDs[i] = segment.RootPieceID.Bytes()
		t.EncryptedKeyNonces[i] = segment.EncryptedKeyNonce
		t.EncryptedKeys[i] = segment.EncryptedKey
		t.EncryptedSizes[i] = segment.EncryptedSize
		t.PlainSizes[i] = segment.PlainSize
		t.PlainOffsets[i] = segment.PlainOffset
		t.EncryptedETags[i] = segment.EncryptedETag
		redundancy, err := segment.Redundancy.EncodeInt64()
		if err != nil {
			return t, Error.New("unable to encode redundancy: %w", err)
		}
		t.RedundancySchemes[i] = redundancy
		t.InlineDatas[i] = segment.InlineData

		piecesData, err := convertPieces(segment.Pieces)
		if err != nil {
			return t, Error.New("unable to convert pieces")
		}

		t.PiecesLists[i] = piecesData
		t.Placements[i] = segment.Placement
	}

	return t, nil
}

// FinishCopyObject accepts new encryption keys for copied object and insert the corresponding new object ObjectKey and segments EncryptedKey.
// It returns the object at the destination location.
func (db *DB) FinishCopyObject(ctx context.Context, opts FinishCopyObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	adapter := db.ChooseAdapter(opts.ProjectID)

	sourceObject, err := adapter.getObjectNonPendingExactVersion(ctx, opts)
	if err != nil {
		if ErrObjectNotFound.Has(err) {
			return Object{}, ErrObjectNotFound.New("source object not found")
		}
		return Object{}, err
	}
	if sourceObject.StreamID != opts.StreamID {
		return Object{}, ErrObjectNotFound.New("object was changed during copy")
	}
	if sourceObject.Status.IsDeleteMarker() {
		return Object{}, ErrMethodNotAllowed.New("copying delete marker is not allowed")
	}

	if opts.VerifyLimits != nil {
		err := opts.VerifyLimits(sourceObject.TotalEncryptedSize, int64(sourceObject.SegmentCount))
		if err != nil {
			return Object{}, err
		}
	}

	if int(sourceObject.SegmentCount) != len(opts.NewSegmentKeys) {
		return Object{}, ErrInvalidRequest.New("wrong number of segments keys received (received %d, need %d)", len(opts.NewSegmentKeys), sourceObject.SegmentCount)
	}

	newSegments, err := adapter.getSegmentsForCopy(ctx, sourceObject)
	if err != nil {
		return Object{}, Error.New("unable to copy object: %w", err)
	}

	if err = checkExpiresAtWithObjectLock(sourceObject, newSegments, opts.Retention, opts.LegalHold); err != nil {
		return Object{}, err
	}

	newSegments.EncryptedKeys = make([][]byte, len(opts.NewSegmentKeys))
	newSegments.EncryptedKeyNonces = make([][]byte, len(opts.NewSegmentKeys))
	for index, u := range opts.NewSegmentKeys {
		if int64(u.Position.Encode()) != newSegments.Positions[index] {
			return Object{}, Error.New("missing new segment keys for segment %d", newSegments.Positions[index])
		}
		newSegments.EncryptedKeys[index] = u.EncryptedKey
		newSegments.EncryptedKeyNonces[index] = u.EncryptedKeyNonce
	}

	var finalEncryptedUserData EncryptedUserData
	if opts.OverrideMetadata {
		finalEncryptedUserData = opts.NewEncryptedUserData
	} else {
		finalEncryptedUserData = opts.NewEncryptedUserData
		finalEncryptedUserData.EncryptedETag = sourceObject.EncryptedETag
		finalEncryptedUserData.EncryptedMetadata = sourceObject.EncryptedMetadata
	}

	// TODO(optimize): inserting pending copy object and segments can be done as a single
	// batch write.
	//
	// TODO(optimize): move inserting encrypted user data into commit. This in some scenarios
	// can avoid some extra data moving (e.g. when the version needs to change) in Spanner.
	// this should also allow use to reuse `finalizeCommitObject` rather than having a separate commitPendingCopyObject.
	newObject, err := adapter.insertPendingCopyObject(ctx, opts, sourceObject, finalEncryptedUserData)
	if err != nil {
		return Object{}, err
	}
	newObject.StreamID = opts.NewStreamID
	newObject.BucketName = opts.NewBucket
	newObject.ObjectKey = opts.NewEncryptedObjectKey
	newObject.EncryptedUserData = finalEncryptedUserData
	newObject.Retention = opts.Retention
	newObject.LegalHold = opts.LegalHold
	newObject.Status = committedWhereVersioned(opts.NewVersioned)

	if err := adapter.finalizeSegmentsCopy(ctx, opts, newSegments); err != nil {
		_, errCleanup := adapter.deleteObjectExactVersion(ctx,
			DeleteObjectExactVersion{
				Version:        newObject.Version,
				ObjectLocation: newObject.Location(),
			})
		return Object{}, errors.Join(err, errCleanup)
	}

	var metrics commitMetrics
	err = db.ChooseAdapter(opts.ProjectID).WithTx(ctx, TransactionOptions{
		TransactionTag: "finish-copy-object",
		TransmitEvent:  opts.TransmitEvent,
	}, func(ctx context.Context, adapter TransactionAdapter) error {
		query, err := db.PrecommitQuery(ctx, PrecommitQuery{
			ObjectStream:   newObject.ObjectStream,
			Pending:        false, // the pending object is already created
			Unversioned:    !opts.NewVersioned,
			HighestVisible: opts.IfNoneMatch.All(),
		}, adapter)
		if err != nil {
			return err
		}

		// We should only commit when an object already doesn't exist.
		if opts.IfNoneMatch.All() {
			if query.HighestVisible.IsCommitted() {
				return ErrFailedPrecondition.New("object already exists")
			}
		}

		// When committing unversioned objects we need to delete any previous unversioned objects.
		if !opts.NewVersioned {
			if err := db.precommitDeleteUnversioned(ctx, adapter, query, &metrics, precommitDeleteUnversioned{
				DisallowDelete:     opts.NewDisallowDelete,
				BypassGovernance:   false,
				DeleteOnlySegments: false,
			}); err != nil {
				return err
			}
		}

		initial := newObject.ObjectStream
		newObject.Version = db.nextVersion(newObject.Version, query.HighestVersion, query.TimestampVersion)

		return adapter.commitPendingCopyObject2(ctx, commitPendingCopyObject{
			Initial: initial,
			Object:  &newObject,
		})
	})
	if err != nil {
		_, errCleanup := adapter.deleteObjectExactVersion(ctx,
			DeleteObjectExactVersion{
				Version:        newObject.Version,
				ObjectLocation: newObject.Location(),
			})
		return Object{}, errors.Join(err, errCleanup)
	}

	metrics.submit()
	mon.Meter("finish_copy_object").Mark(1)

	return newObject, nil
}

func (p *PostgresAdapter) getSegmentsForCopy(ctx context.Context, sourceObject Object) (segments transposedSegmentList, err error) {
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

	err = withRows(p.db.QueryContext(ctx, `
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

func (s *SpannerAdapter) getSegmentsForCopy(ctx context.Context, sourceObject Object) (segments transposedSegmentList, err error) {
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
	err = s.client.Single().QueryWithOptions(ctx, spanner.Statement{
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
		Params: map[string]any{
			"stream_id":     sourceObject.StreamID,
			"segment_count": int64(sourceObject.SegmentCount),
		},
	}, spanner.QueryOptions{RequestTag: "get-segments-for-copy"}).Do(func(row *spanner.Row) error {
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

func (p *PostgresAdapter) finalizeSegmentsCopy(ctx context.Context, opts FinishCopyObject, newSegments transposedSegmentList) (err error) {
	_, err = p.db.ExecContext(ctx, `
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
		return Error.New("unable to copy segments: %w", err)
	}
	return nil
}

func (p *PostgresAdapter) insertPendingCopyObject(ctx context.Context, opts FinishCopyObject, sourceObject Object, encryptedUserData EncryptedUserData) (newObject Object, err error) {
	// TODO we need to handle metadata correctly (copy from original object or replace)

	zombieDeletionDeadline := time.Now().Add(defaultZombieDeletionCopyObjectPeriod)

	row := p.db.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				status, expires_at, segment_count,
				encryption,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				zombie_deletion_deadline,
				retention_mode, retain_until
			) VALUES (
				$1, $2, $3,`+p.generateVersion()+`, $4,
				$5, $6, $7,
				$8,
				$9, $10, $11, $12,
				$13, $14, $15,
				$16,
				$17, $18
			)
			RETURNING
				version, created_at`,
		opts.ProjectID, opts.NewBucket, opts.NewEncryptedObjectKey, opts.NewStreamID,
		Pending, sourceObject.ExpiresAt, sourceObject.SegmentCount,
		&sourceObject.Encryption,
		encryptedUserData.EncryptedMetadata, encryptedUserData.EncryptedMetadataNonce, encryptedUserData.EncryptedMetadataEncryptedKey, encryptedUserData.EncryptedETag,
		sourceObject.TotalPlainSize, sourceObject.TotalEncryptedSize, sourceObject.FixedSegmentSize,
		&zombieDeletionDeadline,
		lockModeWrapper{retentionMode: &opts.Retention.Mode, legalHold: &opts.LegalHold},
		timeWrapper{&opts.Retention.RetainUntil},
	)

	newObject = sourceObject

	err = row.Scan(&newObject.Version, &newObject.CreatedAt)
	if err != nil {
		return Object{}, Error.New("unable to copy object: %w", err)
	}

	return newObject, nil
}

func (s *SpannerAdapter) finalizeSegmentsCopy(ctx context.Context, opts FinishCopyObject, newSegments transposedSegmentList) (err error) {
	// we need to batch inserts to avoid Spanner maximum mutation number limit
	const batchSize = 1000 // TODO make batchSize configurable
	inserts := make([]*spanner.Mutation, 0, batchSize)

	for i := range newSegments.Positions {
		inserts = append(inserts, spanner.Insert("segments",
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
		))

		if len(inserts) >= batchSize {
			if _, err := s.client.Apply(ctx, inserts); err != nil {
				return Error.New("unable to copy segments: %w", err)
			}
			inserts = inserts[:0]
		}
	}

	if len(inserts) > 0 {
		if _, err := s.client.Apply(ctx, inserts); err != nil {
			return Error.New("unable to copy segments: %w", err)
		}
	}

	return nil
}

func (s *SpannerAdapter) insertPendingCopyObject(ctx context.Context, opts FinishCopyObject, sourceObject Object, encryptedUserData EncryptedUserData) (newObject Object, err error) {
	// TODO we need to handle metadata correctly (copy from original object or replace)

	newObject = sourceObject

	zombieDeletionDeadline := time.Now().Add(defaultZombieDeletionCopyObjectPeriod)

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		return tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `
				INSERT INTO objects (
					project_id, bucket_name, object_key, version, stream_id,
					status, expires_at, segment_count,
					encryption,
					encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
					total_plain_size, total_encrypted_size, fixed_segment_size,
					zombie_deletion_deadline,
					retention_mode, retain_until
				) VALUES (
					@project_id, @bucket_name, @object_key, ` + s.generateVersion() + `, @stream_id,
					@status, @expires_at, @segment_count,
					@encryption,
					@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key, @encrypted_etag,
					@total_plain_size, @total_encrypted_size, @fixed_segment_size,
					@zombie_deletion_deadline,
					@retention_mode, @retain_until
				)
				THEN RETURN
					version, created_at
			`,
			Params: map[string]interface{}{
				"project_id":                       opts.ProjectID,
				"bucket_name":                      opts.NewBucket,
				"object_key":                       opts.NewEncryptedObjectKey,
				"stream_id":                        opts.NewStreamID,
				"status":                           Pending,
				"expires_at":                       sourceObject.ExpiresAt,
				"segment_count":                    int64(sourceObject.SegmentCount),
				"encryption":                       sourceObject.Encryption,
				"encrypted_metadata":               encryptedUserData.EncryptedMetadata,
				"encrypted_metadata_nonce":         encryptedUserData.EncryptedMetadataNonce,
				"encrypted_metadata_encrypted_key": encryptedUserData.EncryptedMetadataEncryptedKey,
				"encrypted_etag":                   encryptedUserData.EncryptedETag,
				"total_plain_size":                 sourceObject.TotalPlainSize,
				"total_encrypted_size":             sourceObject.TotalEncryptedSize,
				"fixed_segment_size":               int64(sourceObject.FixedSegmentSize),
				"zombie_deletion_deadline":         &zombieDeletionDeadline,
				"retention_mode":                   lockModeWrapper{retentionMode: &opts.Retention.Mode, legalHold: &opts.LegalHold},
				"retain_until":                     timeWrapper{&opts.Retention.RetainUntil},
			},
		}, spanner.QueryOptions{RequestTag: "object-copy-insert-pending"}).Do(func(row *spanner.Row) error {
			err := row.Columns(&newObject.Version, &newObject.CreatedAt)
			if err != nil {
				return Error.New("unable to scan created_at: %w", err)
			}
			return nil
		})
	}, spanner.TransactionOptions{
		TransactionTag:              "object-copy-insert-pending",
		ExcludeTxnFromChangeStreams: true,
	})
	if err != nil {
		return Object{}, Error.New("unable to copy object: %w", err)
	}

	return newObject, nil
}

func (ptx *postgresTransactionAdapter) commitPendingCopyObject(ctx context.Context, object *Object, highestVersion Version) (err error) {
	if object.Version == highestVersion {
		_, err = ptx.tx.ExecContext(ctx, `
			UPDATE objects SET
				status = $6,
				zombie_deletion_deadline = NULL
			WHERE
				(project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
				status       = `+statusPending,
			object.ProjectID, object.BucketName, object.ObjectKey, object.Version, object.StreamID,
			object.Status,
		)
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}
		return nil
	}

	// When there was an insert during finish copy object we need to also update the version.
	if !ptx.postgresAdapter.testingTimestampVersioning {
		oldVersion := object.Version
		object.Version = highestVersion + 1
		_, err = ptx.tx.ExecContext(ctx, `
				UPDATE objects SET
					status = $6,
					version = $7,
					zombie_deletion_deadline = NULL
				WHERE
					(project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
					status       = `+statusPending,
			object.ProjectID, object.BucketName, object.ObjectKey, oldVersion, object.StreamID,
			object.Status,
			object.Version,
		)
	} else {
		err = ptx.tx.QueryRowContext(ctx, `
				UPDATE objects SET
					status = $6,
					version = `+postgresGenerateTimestampVersion+`,
					zombie_deletion_deadline = NULL
				WHERE
					(project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
					status       = `+statusPending+`
				RETURNING version
			`,
			object.ProjectID, object.BucketName, object.ObjectKey, object.Version, object.StreamID,
			object.Status,
		).Scan(&object.Version)
	}
	if err != nil {
		return Error.New("unable to copy object: %w", err)
	}

	return nil
}

func (stx *spannerTransactionAdapter) commitPendingCopyObject(ctx context.Context, object *Object, highestVersion Version) (err error) {
	if object.Version == highestVersion {
		err = stx.tx.BufferWrite([]*spanner.Mutation{
			spanner.Update("objects", []string{
				"project_id", "bucket_name", "object_key", "version", "stream_id",
				"status", "zombie_deletion_deadline",
			}, []any{
				object.ProjectID, object.BucketName, object.ObjectKey, object.Version, object.StreamID,
				object.Status, nil,
			}),
		})
		if err != nil {
			return Error.New("unable to finish copy object: %w", err)
		}
		return nil
	}

	// When there was an insert during finish copy object we need to also update the version.
	//
	// We can not simply UPDATE the row, because we are changing the 'version' column,
	// which is part of the primary key. Spanner does not allow changing a primary key
	// column on an existing row. We must DELETE then INSERT a new row.

	oldVersion := object.Version
	if !stx.spannerAdapter.testingTimestampVersioning {
		object.Version = highestVersion + 1
	} else {
		// TODO: should we generate the timestamp version on satellite side and use it instead?
		// This would reduce the communication to the database.
		//
		// Alternatively, we could do the use "highestVersion + 1" even in the timestamp
		// versioning mode. The primary guarantee would still hold -- although, we may not be able to
		// get rid of querying the highest version.
		err = stx.tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `SELECT ` + spannerGenerateTimestampVersion + "AS version",
		}, spanner.QueryOptions{RequestTag: "copy-object-request-version"}).Do(func(row *spanner.Row) error {
			return Error.Wrap(row.Columns(&object.Version))
		})
		if err != nil {
			return Error.New("failed to request timestamp: %w", err)
		}
	}

	err = stx.tx.BufferWrite([]*spanner.Mutation{
		spanner.Delete("objects", spanner.Key{
			object.ProjectID, object.BucketName, object.ObjectKey, int64(oldVersion),
		}),
		spannerInsertObject(RawObject(*object)),
	})
	if err != nil {
		return Error.New("unable to finish copy object: %w", err)
	}

	return nil
}

type commitPendingCopyObject struct {
	Initial ObjectStream
	Object  *Object
}

func (ptx *postgresTransactionAdapter) commitPendingCopyObject2(ctx context.Context, opts commitPendingCopyObject) (err error) {
	initial := opts.Initial
	object := opts.Object

	result, err := ptx.tx.ExecContext(ctx, `
		UPDATE objects SET
			version = $6,
			status = $7,
			zombie_deletion_deadline = NULL
		WHERE
			(project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
			status       = `+statusPending,
		initial.ProjectID, initial.BucketName, initial.ObjectKey, initial.Version, initial.StreamID,
		object.Version, object.Status,
	)
	if err != nil {
		return Error.New("failed to update object: %w", err)
	}
	if count, err := result.RowsAffected(); count != 1 || err != nil {
		// This may happen when:
		//
		// 1. user starts copy object
		// 2. user calls list pending objects
		// 3. user invokes commit or abort pending object
		// 4. the 1. copy object arrives here in commitPendingCopyObject2.
		return Error.New("failed to update object %#v (changed %d rows): %w", initial, count, err)
	}
	return nil
}

func (stx *spannerTransactionAdapter) commitPendingCopyObject2(ctx context.Context, opts commitPendingCopyObject) (err error) {
	initial := opts.Initial
	object := opts.Object

	if object.Version == initial.Version {
		updateMap := map[string]any{
			"project_id":               initial.ProjectID,
			"bucket_name":              initial.BucketName,
			"object_key":               initial.ObjectKey,
			"version":                  initial.Version,
			"status":                   object.Status,
			"zombie_deletion_deadline": nil,
		}

		err = stx.tx.BufferWrite([]*spanner.Mutation{
			spanner.UpdateMap("objects", updateMap),
		})
		if err != nil {
			return Error.New("failed to update object: %w", err)
		}

		return nil
	}

	err = stx.tx.BufferWrite([]*spanner.Mutation{
		spanner.Delete("objects", spanner.Key{
			initial.ProjectID,
			initial.BucketName,
			initial.ObjectKey,
			int64(initial.Version),
		}),
		spannerInsertObject(RawObject(*object)),
	})
	if err != nil {
		return Error.New("failed to update object: %w", err)
	}

	return nil
}

// getObjectNonPendingExactVersion returns object information for exact version.
//
// Note: this returns both committed objects and delete markers.
func (p *PostgresAdapter) getObjectNonPendingExactVersion(ctx context.Context, opts FinishCopyObject) (_ Object, err error) {
	defer mon.Task()(&ctx)(&err)

	object := Object{}
	err = p.db.QueryRowContext(ctx, `
		SELECT
			stream_id, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
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
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			&object.Encryption,
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

func (s *SpannerAdapter) getObjectNonPendingExactVersion(ctx context.Context, opts FinishCopyObject) (_ Object, err error) {
	defer mon.Task()(&ctx)(&err)

	found := false
	object := Object{}
	err = s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT
				stream_id, status,
				created_at, expires_at,
				segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
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
	}, spanner.QueryOptions{RequestTag: "get-object-non-pending-exact-version"}).Do(func(row *spanner.Row) error {
		found = true
		err := row.Columns(
			&object.StreamID, &object.Status,
			&object.CreatedAt, &object.ExpiresAt,
			spannerutil.Int(&object.SegmentCount),
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
			&object.TotalPlainSize, &object.TotalEncryptedSize, spannerutil.Int(&object.FixedSegmentSize),
			&object.Encryption,
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
