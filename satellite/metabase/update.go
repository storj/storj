// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/dbutil/tidbutil"
	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

const (
	noLockWithExpirationErrMsg         = "Object Lock settings must not be placed on an object with an expiration date"
	noLockWithExpirationSegmentsErrMsg = "Object Lock settings must not be placed on an object with segments having an expiration date"
	noLockOnUncommittedErrMsg          = "Object Lock settings must only be placed on committed objects"
	noLockFromUncommittedErrMsg        = "Object Lock settings must only be retrieved from committed objects"
	noShortenRetentionErrMsg           = "retention period cannot be shortened"
	noRemoveRetentionErrMsg            = "an active retention configuration cannot be removed"
	noChangeRetentionModeErrMsg        = "retention mode cannot be changed"
	noLockOnDeleteMarkerErrMsg         = "Object Lock settings must not be placed on delete markers"
)

var (
	// ErrValueChanged is returned when the current value of the key does not match the oldValue in UpdateSegmentPieces.
	ErrValueChanged = errs.Class("value changed")
	// ErrObjectExpiration is used when an object's expiration prevents an operation from succeeding.
	ErrObjectExpiration = errs.Class("object expiration")
	// ErrObjectStatus is used when an object's status prevents an operation from succeeding.
	ErrObjectStatus = errs.Class("object status")
)

// UpdateSegmentPieces contains arguments necessary for updating segment pieces.
type UpdateSegmentPieces struct {
	// Name of the database adapter to use for this segment. If empty (""), check all adapters
	// until the segment is found.
	DBAdapterName string

	StreamID uuid.UUID
	Position SegmentPosition

	OldPieces Pieces

	// OldPiecesHash, when set, uses SHA256(remote_alias_pieces) for the CAS condition
	// instead of exact match on OldPieces. This allows callers that only have a hash
	// (e.g. from a prior loop scan) to perform an atomic compare-and-swap without
	// needing the exact old pieces. When set, OldPieces is ignored.
	OldPiecesHash []byte

	NewRedundancy storj.RedundancyScheme
	NewPieces     Pieces

	NewRepairedAt time.Time // sets new time of last segment repair (optional).
}

// UpdateSegmentPieces updates pieces for specified segment. If provided old pieces
// won't match current database state update will fail.
func (db *DB) UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.StreamID.IsZero() {
		return ErrInvalidRequest.New("StreamID missing")
	}

	useHashCAS := len(opts.OldPiecesHash) > 0

	if !useHashCAS {
		if err := opts.OldPieces.Verify(); err != nil {
			if ErrInvalidRequest.Has(err) {
				return ErrInvalidRequest.New("OldPieces: %v", errors.Unwrap(err))
			}
			return err
		}
	}

	if opts.NewRedundancy.IsZero() {
		return ErrInvalidRequest.New("NewRedundancy zero")
	}

	// its possible that in this method we will have less pieces
	// than optimal shares (e.g. after repair)
	if len(opts.NewPieces) < int(opts.NewRedundancy.RepairShares) {
		db.log.Warn("number of new pieces is less than new redundancy repair shares value (segment will return to repair queue)",
			zap.Int("new_pieces", len(opts.NewPieces)),
			zap.Int("new_redundancy_repair_shares", int(opts.NewRedundancy.RepairShares)))
	}
	if len(opts.NewPieces) < int(opts.NewRedundancy.RequiredShares) {
		return ErrInvalidRequest.New("number of pieces is less than redundancy required shares")
	}

	if err := opts.NewPieces.Verify(); err != nil {
		if ErrInvalidRequest.Has(err) {
			return ErrInvalidRequest.New("NewPieces: %v", errors.Unwrap(err))
		}
		return err
	}

	var oldPieces AliasPieces
	if !useHashCAS {
		oldPieces, err = db.aliasCache.EnsurePiecesToAliases(ctx, opts.OldPieces)
		if err != nil {
			return Error.New("unable to convert pieces to aliases: %w", err)
		}
	}

	newPieces, err := db.aliasCache.EnsurePiecesToAliases(ctx, opts.NewPieces)
	if err != nil {
		return Error.New("unable to convert pieces to aliases: %w", err)
	}

	var resultPieces AliasPieces
	for _, adapter := range db.adapters {
		if opts.DBAdapterName == "" || opts.DBAdapterName == adapter.Name() {
			resultPieces, err = adapter.UpdateSegmentPieces(ctx, opts, oldPieces, newPieces)
			if err != nil {
				if ErrSegmentNotFound.Has(err) {
					continue
				}
				return err
			}
			// segment was found
			break
		}
	}
	if resultPieces == nil {
		return ErrSegmentNotFound.New("segment missing")
	}

	if !EqualAliasPieces(newPieces, resultPieces) {
		return ErrValueChanged.New("segment remote_alias_pieces field was changed")
	}

	mon.Meter("segment_update").Mark(1)

	return nil
}

// UpdateSegmentPieces updates pieces for specified segment, if pieces matches oldPieces.
func (p *PostgresAdapter) UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces, oldPieces, newPieces AliasPieces) (resultPieces AliasPieces, err error) {
	updateRepairAt := !opts.NewRepairedAt.IsZero()

	if len(opts.OldPiecesHash) > 0 {
		err = p.db.QueryRowContext(ctx, `
			UPDATE segments SET
				remote_alias_pieces = CASE
					WHEN sha256(remote_alias_pieces) = $3 THEN $4
					ELSE remote_alias_pieces
				END,
				redundancy = CASE
					WHEN sha256(remote_alias_pieces) = $3 THEN $5
					ELSE redundancy
				END,
				repaired_at = CASE
					WHEN sha256(remote_alias_pieces) = $3 AND $7 = true THEN $6
					ELSE repaired_at
				END
			WHERE
				stream_id     = $1 AND
				position      = $2
			RETURNING remote_alias_pieces
			`, opts.StreamID, opts.Position, opts.OldPiecesHash, newPieces, &opts.NewRedundancy, opts.NewRepairedAt, updateRepairAt).
			Scan(&resultPieces)
	} else {
		err = p.db.QueryRowContext(ctx, `
			UPDATE segments SET
				remote_alias_pieces = CASE
					WHEN remote_alias_pieces = $3 THEN $4
					ELSE remote_alias_pieces
				END,
				redundancy = CASE
					WHEN remote_alias_pieces = $3 THEN $5
					ELSE redundancy
				END,
				repaired_at = CASE
					WHEN remote_alias_pieces = $3 AND $7 = true THEN $6
					ELSE repaired_at
				END
			WHERE
				stream_id     = $1 AND
				position      = $2
			RETURNING remote_alias_pieces
			`, opts.StreamID, opts.Position, oldPieces, newPieces, &opts.NewRedundancy, opts.NewRepairedAt, updateRepairAt).
			Scan(&resultPieces)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSegmentNotFound.New("segment missing")
		}
		return nil, Error.New("unable to update segment pieces: %w", err)
	}
	return resultPieces, nil
}

// UpdateSegmentPieces updates pieces for specified segment, if pieces matches oldPieces.
func (t *TiDBAdapter) UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces, oldPieces, newPieces AliasPieces) (resultPieces AliasPieces, err error) {
	defer mon.Task()(&ctx)(&err)

	updateRepairAt := !opts.NewRepairedAt.IsZero()

	// Match Postgres: when OldPiecesHash is set, compare SHA256 of the stored
	// remote_alias_pieces; otherwise compare the bytes directly. SHA2(x,256)
	// in MySQL/TiDB returns a hex string, so we hex-encode the expected hash
	// in Go to keep the comparison cheap (no UNHEX per row).
	cas := "remote_alias_pieces = ?"
	casArg := any(oldPieces)
	if len(opts.OldPiecesHash) > 0 {
		cas = "SHA2(remote_alias_pieces, 256) = ?"
		casArg = hex.EncodeToString(opts.OldPiecesHash)
	}

	// Wrap UPDATE + SELECT in one transaction so the SELECT sees this
	// transaction's own UPDATE, not a concurrent writer's commit landing
	// between the two statements. In autocommit mode TiDB would commit the
	// UPDATE on its own, leaving a race window where a racing writer could
	// flip remote_alias_pieces and cause the caller to report a false
	// ErrValueChanged for a CAS that actually succeeded. Within a
	// transaction, MVCC makes the SELECT observe the in-flight UPDATE,
	// matching the atomic RETURNING semantics of the Postgres adapter.
	//
	// CommitWithQuery folds BEGIN, the UPDATE + SELECT multi-statement, and
	// COMMIT into a single round trip: with multiStatements=true the MySQL
	// driver pipelines the `;`-separated statements into one COM_QUERY packet.
	err = tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) (err error) {
		// reset on retry
		resultPieces = nil

		return tx.CommitWithQuery(ctx, `
			UPDATE segments SET
				remote_alias_pieces = CASE
					WHEN `+cas+` THEN ?
					ELSE remote_alias_pieces
				END,
				redundancy = CASE
					WHEN `+cas+` THEN ?
					ELSE redundancy
				END,
				repaired_at = CASE
					WHEN `+cas+` AND ? = TRUE THEN ?
					ELSE repaired_at
				END
			WHERE (stream_id, position) = (?, ?);
			SELECT remote_alias_pieces FROM segments WHERE (stream_id, position) = (?, ?)
		`,
			[]any{
				casArg, newPieces, casArg, &opts.NewRedundancy, casArg, updateRepairAt, opts.NewRepairedAt,
				opts.StreamID, opts.Position,
				opts.StreamID, opts.Position,
			},
			tidbutil.ScanFirstRow(&resultPieces))
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSegmentNotFound.New("segment missing")
		}
		return nil, Error.New("unable to update segment pieces: %w", err)
	}
	return resultPieces, nil
}

// UpdateSegmentPieces updates pieces for specified segment, if pieces matches oldPieces.
func (s *SpannerAdapter) UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces, oldPieces, newPieces AliasPieces) (resultPieces AliasPieces, err error) {
	updateRepairAt := !opts.NewRepairedAt.IsZero()

	var casSQL string
	params := map[string]any{
		"stream_id":          opts.StreamID,
		"position":           opts.Position,
		"new_pieces":         newPieces,
		"redundancy":         opts.NewRedundancy,
		"new_repaired_at":    opts.NewRepairedAt,
		"update_repaired_at": updateRepairAt,
	}

	if len(opts.OldPiecesHash) > 0 {
		casSQL = `SHA256(remote_alias_pieces) = @old_pieces_hash`
		params["old_pieces_hash"] = opts.OldPiecesHash
	} else {
		casSQL = `remote_alias_pieces = @old_pieces`
		params["old_pieces"] = oldPieces
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		resultPieces, err = spannerutil.CollectRow(tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `
				UPDATE segments SET
					remote_alias_pieces = CASE
						WHEN ` + casSQL + ` THEN @new_pieces
						ELSE remote_alias_pieces
					END,
					redundancy = CASE
						WHEN ` + casSQL + ` THEN @redundancy
						ELSE redundancy
					END,
					repaired_at = CASE
						WHEN ` + casSQL + ` AND @update_repaired_at = true THEN @new_repaired_at
						ELSE repaired_at
					END
				WHERE
					stream_id     = @stream_id AND
					position      = @position
				THEN RETURN remote_alias_pieces
			`,
			Params: params,
		}, spanner.QueryOptions{RequestTag: "update-segment-pieces"}), func(row *spanner.Row, item *AliasPieces) error {
			err = row.Columns(item)
			if err != nil {
				return Error.New("unable to decode result pieces: %w", err)
			}
			return nil
		})

		if err != nil {
			if errors.Is(err, iterator.Done) {
				return ErrSegmentNotFound.New("segment missing")
			}
			return Error.New("unable to update segment pieces: %w", err)
		}

		return nil
	}, spanner.TransactionOptions{
		TransactionTag:              "update-segment-pieces",
		ExcludeTxnFromChangeStreams: true,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return resultPieces, nil
}

// BatchUpdateSegmentPiecesEntry contains the data needed to update one segment's pieces.
type BatchUpdateSegmentPiecesEntry struct {
	StreamID      uuid.UUID
	Position      SegmentPosition
	OldRepairedAt *time.Time
	NewPieces     Pieces
	NewRedundancy storj.RedundancyScheme
	NewRepairedAt time.Time

	// OldPiecesHash, when set, uses SHA256(remote_alias_pieces) for the CAS condition
	// instead of OldRepairedAt. When set, OldRepairedAt is ignored.
	OldPiecesHash []byte
}

// BatchUpdateSegmentPieces contains arguments necessary for batch updating segment pieces.
type BatchUpdateSegmentPieces struct {
	Entries []BatchUpdateSegmentPiecesEntry
}

// BatchUpdateSegmentPieces updates pieces for multiple segments using a CAS on repaired_at.
// Returns which entries succeeded (true) and which had a CAS conflict or were missing (false).
func (db *DB) BatchUpdateSegmentPieces(ctx context.Context, opts BatchUpdateSegmentPieces) (results []bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(opts.Entries) == 0 {
		return nil, nil
	}

	aliasEntries := make([]AliasPieces, len(opts.Entries))
	for i, entry := range opts.Entries {
		aliasEntries[i], err = db.aliasCache.EnsurePiecesToAliases(ctx, entry.NewPieces)
		if err != nil {
			return nil, Error.New("unable to convert pieces to aliases: %w", err)
		}
	}

	results = make([]bool, len(opts.Entries))
	for _, adapter := range db.adapters {
		adapterResults, adapterErr := adapter.BatchUpdateSegmentPieces(ctx, opts, aliasEntries)
		if adapterErr != nil {
			return nil, adapterErr
		}
		for i, ok := range adapterResults {
			if ok {
				results[i] = true
			}
		}
	}

	mon.Meter("segment_update").Mark(len(opts.Entries))

	return results, nil
}

// BatchUpdateSegmentPieces updates pieces for multiple segments using a CAS on repaired_at or pieces hash.
func (p *PostgresAdapter) BatchUpdateSegmentPieces(ctx context.Context, opts BatchUpdateSegmentPieces, newAliasPieces []AliasPieces) (results []bool, err error) {
	defer mon.Task()(&ctx)(&err)

	results = make([]bool, len(opts.Entries))

	for i, entry := range opts.Entries {
		var result sql.Result
		if len(entry.OldPiecesHash) > 0 {
			result, err = p.db.ExecContext(ctx, `
				UPDATE segments SET
					remote_alias_pieces = $4,
					redundancy = $5,
					repaired_at = $6
				WHERE
					stream_id = $1 AND position = $2
					AND sha256(remote_alias_pieces) = $3
			`, entry.StreamID, entry.Position,
				entry.OldPiecesHash, newAliasPieces[i],
				&entry.NewRedundancy, entry.NewRepairedAt,
			)
		} else {
			result, err = p.db.ExecContext(ctx, `
				UPDATE segments SET
					remote_alias_pieces = $4,
					redundancy = $5,
					repaired_at = $6
				WHERE
					stream_id = $1 AND position = $2
					AND repaired_at IS NOT DISTINCT FROM $3
			`, entry.StreamID, entry.Position,
				entry.OldRepairedAt, newAliasPieces[i],
				&entry.NewRedundancy, entry.NewRepairedAt,
			)
		}
		if err != nil {
			return nil, Error.New("unable to batch update segment pieces: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, Error.New("unable to get rows affected: %w", err)
		}
		results[i] = affected > 0
	}
	return results, nil
}

// BatchUpdateSegmentPieces updates pieces for multiple segments using a CAS on repaired_at or pieces hash.
func (t *TiDBAdapter) BatchUpdateSegmentPieces(ctx context.Context, opts BatchUpdateSegmentPieces, newAliasPieces []AliasPieces) (results []bool, err error) {
	defer mon.Task()(&ctx)(&err)

	results = make([]bool, len(opts.Entries))

	// Mirror the Postgres adapter: one UPDATE per entry, switching the CAS
	// condition based on whether OldPiecesHash was provided. SHA2(x, 256) in
	// MySQL/TiDB returns a hex string, so we hex-encode the expected hash in
	// Go to keep the comparison cheap. For the no-hash variant, `<=>` is the
	// MySQL/TiDB null-safe equivalent of Postgres `IS NOT DISTINCT FROM`.
	for i, entry := range opts.Entries {
		var result sql.Result
		if len(entry.OldPiecesHash) > 0 {
			result, err = t.db.ExecContext(ctx, `
				UPDATE segments SET
					remote_alias_pieces = ?,
					redundancy = ?,
					repaired_at = ?
				WHERE
					stream_id = ? AND position = ?
					AND SHA2(remote_alias_pieces, 256) = ?
			`, newAliasPieces[i], &entry.NewRedundancy, entry.NewRepairedAt,
				entry.StreamID, entry.Position,
				hex.EncodeToString(entry.OldPiecesHash),
			)
		} else {
			result, err = t.db.ExecContext(ctx, `
				UPDATE segments SET
					remote_alias_pieces = ?,
					redundancy = ?,
					repaired_at = ?
				WHERE
					stream_id = ? AND position = ?
					AND repaired_at <=> ?
			`, newAliasPieces[i], &entry.NewRedundancy, entry.NewRepairedAt,
				entry.StreamID, entry.Position,
				entry.OldRepairedAt,
			)
		}
		if err != nil {
			return nil, Error.New("unable to batch update segment pieces: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, Error.New("unable to get rows affected: %w", err)
		}
		results[i] = affected > 0
	}
	return results, nil
}

// BatchUpdateSegmentPieces updates pieces for multiple segments using a CAS on repaired_at or pieces hash.
func (s *SpannerAdapter) BatchUpdateSegmentPieces(ctx context.Context, opts BatchUpdateSegmentPieces, newAliasPieces []AliasPieces) (results []bool, err error) {
	defer mon.Task()(&ctx)(&err)

	stmts := make([]spanner.Statement, len(opts.Entries))
	for i, entry := range opts.Entries {
		if len(entry.OldPiecesHash) > 0 {
			stmts[i] = spanner.Statement{
				SQL: `
					UPDATE segments SET
						remote_alias_pieces = @new_pieces,
						redundancy = @redundancy,
						repaired_at = @new_repaired_at
					WHERE
						stream_id = @stream_id AND position = @position
						AND SHA256(remote_alias_pieces) = @old_pieces_hash
				`,
				Params: map[string]any{
					"stream_id":       entry.StreamID,
					"position":        entry.Position,
					"old_pieces_hash": entry.OldPiecesHash,
					"new_pieces":      newAliasPieces[i],
					"redundancy":      entry.NewRedundancy,
					"new_repaired_at": entry.NewRepairedAt,
				},
			}
		} else {
			stmts[i] = spanner.Statement{
				SQL: `
					UPDATE segments SET
						remote_alias_pieces = @new_pieces,
						redundancy = @redundancy,
						repaired_at = @new_repaired_at
					WHERE
						stream_id = @stream_id AND position = @position
						AND (repaired_at = @old_repaired_at OR (repaired_at IS NULL AND @old_repaired_at IS NULL))
				`,
				Params: map[string]any{
					"stream_id":       entry.StreamID,
					"position":        entry.Position,
					"old_repaired_at": entry.OldRepairedAt,
					"new_pieces":      newAliasPieces[i],
					"redundancy":      entry.NewRedundancy,
					"new_repaired_at": entry.NewRepairedAt,
				},
			}
		}
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		results = make([]bool, len(opts.Entries))
		affecteds, err := tx.BatchUpdateWithOptions(ctx, stmts, spanner.QueryOptions{RequestTag: "batch-update-segment-pieces"})
		if err != nil {
			return Error.New("unable to batch update segment pieces: %w", err)
		}
		for i, affected := range affecteds {
			results[i] = affected > 0
		}
		return nil
	}, spanner.TransactionOptions{
		TransactionTag:              "batch-update-segment-pieces",
		ExcludeTxnFromChangeStreams: true,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return results, nil
}

// SetObjectExactVersionLegalHold contains arguments necessary for setting
// the legal hold configuration of an exact version of an object.
type SetObjectExactVersionLegalHold struct {
	ObjectLocation
	Version Version

	Enabled bool
}

// Verify verifies the request fields.
func (opts *SetObjectExactVersionLegalHold) Verify() error {
	return opts.ObjectLocation.Verify()
}

// SetObjectExactVersionLegalHold sets the legal hold configuration of an exact version of an object.
func (db *DB) SetObjectExactVersionLegalHold(ctx context.Context, opts SetObjectExactVersionLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = opts.Verify(); err != nil {
		return err
	}

	return db.ChooseAdapter(opts.ProjectID).SetObjectExactVersionLegalHold(ctx, opts)
}

// SetObjectExactVersionLegalHold sets the legal hold configuration of an exact version of an object.
func (p *PostgresAdapter) SetObjectExactVersionLegalHold(ctx context.Context, opts SetObjectExactVersionLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		status    ObjectStatus
		expiresAt *time.Time
		updated   bool
	)

	err = p.db.QueryRowContext(ctx, `
		WITH pre_update_info AS (
			SELECT status, expires_at
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
		), updated AS (
			UPDATE objects
			SET
				retention_mode = CASE
					WHEN $5 THEN COALESCE(retention_mode, 0) | `+retentionModeLegalHold+` -- Enable legal hold
					ELSE retention_mode & ~`+retentionModeLegalHold+` -- Disable legal hold
				END
			WHERE
				(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
				AND status IN `+statusesCommitted+`
				AND expires_at IS NULL
			RETURNING 1
		)
		SELECT status, expires_at, EXISTS(SELECT 1 FROM updated) FROM pre_update_info`,
		opts.ProjectID,
		opts.BucketName,
		opts.ObjectKey,
		opts.Version,
		opts.Enabled,
	).Scan(
		&status,
		&expiresAt,
		&updated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrObjectNotFound.New("")
		}
		return Error.New("unable to update object legal hold configuration: %w", err)
	}

	if !updated {
		switch {
		case status.IsDeleteMarker():
			return ErrObjectStatus.New(noLockOnDeleteMarkerErrMsg)
		case !status.IsCommitted():
			return ErrObjectStatus.New(noLockOnUncommittedErrMsg)
		case expiresAt != nil:
			return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
		default:
			return Error.New("unable to update object legal hold configuration")
		}
	}

	return nil
}

// SetObjectExactVersionLegalHold sets the legal hold configuration of an exact version of an object.
func (t *TiDBAdapter) SetObjectExactVersionLegalHold(ctx context.Context, opts SetObjectExactVersionLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	// The FOR UPDATE select folds BEGIN into its first statement and
	// CommitWithExec folds the UPDATE together with COMMIT, so this read then
	// dependent write costs two round trips instead of four.
	return tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
		var (
			status    ObjectStatus
			expiresAt *time.Time
		)
		err := tx.QueryRowContext(ctx, `
			SELECT status, expires_at
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)
			FOR UPDATE`,
			opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
		).Scan(&status, &expiresAt)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrObjectNotFound.New("")
			}
			return Error.New("unable to query object info before setting legal hold: %w", err)
		}

		switch {
		case status.IsDeleteMarker():
			return ErrObjectStatus.New(noLockOnDeleteMarkerErrMsg)
		case !status.IsCommitted():
			return ErrObjectStatus.New(noLockOnUncommittedErrMsg)
		case expiresAt != nil:
			return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
		}

		if err = tx.CommitWithExec(ctx, `
			UPDATE objects SET
				retention_mode = CASE
					WHEN ? THEN COALESCE(retention_mode, 0) | `+retentionModeLegalHold+`
					ELSE retention_mode & ~`+retentionModeLegalHold+`
				END
			WHERE
				(project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`,
			opts.Enabled, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
		); err != nil {
			return Error.New("unable to update object legal hold configuration: %w", err)
		}
		return nil
	})
}

// SetObjectExactVersionLegalHold sets the legal hold configuration of an exact version of an object.
func (s *SpannerAdapter) SetObjectExactVersionLegalHold(ctx context.Context, opts SetObjectExactVersionLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result, err := spannerutil.CollectRow(tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `
				SELECT status, expires_at, retention_mode
				FROM objects
				WHERE (project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
			`,
			Params: map[string]any{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     opts.Version,
			},
		}, spanner.QueryOptions{RequestTag: "set-object-exact-version-legal-hold-check"}),
			func(row *spanner.Row, item *preUpdateRetentionInfo) error {
				return errs.Wrap(row.Columns(
					&item.Status,
					&item.ExpiresAt,
					lockModeWrapper{retentionMode: &item.Retention.Mode},
				))
			})
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return ErrObjectNotFound.New("")
			}
			return errs.New("unable to query object info before setting legal hold: %w", err)
		}

		switch {
		case result.Status.IsDeleteMarker():
			return ErrObjectStatus.New(noLockOnDeleteMarkerErrMsg)
		case !result.Status.IsCommitted():
			return ErrObjectStatus.New(noLockOnUncommittedErrMsg)
		case result.ExpiresAt != nil:
			return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
		}

		return errs.Wrap(s.setObjectExactVersionLegalHold(ctx, tx, opts, result.Retention.Mode))
	}, spanner.TransactionOptions{
		TransactionTag:              "set-object-exact-version-legal-hold",
		ExcludeTxnFromChangeStreams: true,
	})

	if err != nil {
		if ErrObjectNotFound.Has(err) || ErrObjectExpiration.Has(err) || ErrObjectStatus.Has(err) {
			return errs.Wrap(err)
		}
		return Error.Wrap(err)
	}

	return nil
}

// SetObjectLastCommittedLegalHold contains arguments necessary for setting
// the legal hold configuration of the most recently committed version of an object.
type SetObjectLastCommittedLegalHold struct {
	ObjectLocation

	Enabled bool
}

// Verify verifies the request fields.
func (opts SetObjectLastCommittedLegalHold) Verify() error {
	return opts.ObjectLocation.Verify()
}

// SetObjectLastCommittedLegalHold sets the legal hold configuration
// of the most recently committed version of an object.
func (db *DB) SetObjectLastCommittedLegalHold(ctx context.Context, opts SetObjectLastCommittedLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = opts.Verify(); err != nil {
		return err
	}

	return db.ChooseAdapter(opts.ProjectID).SetObjectLastCommittedLegalHold(ctx, opts)
}

// SetObjectLastCommittedLegalHold sets the legal hold configuration
// of the most recently committed version of an object.
func (p *PostgresAdapter) SetObjectLastCommittedLegalHold(ctx context.Context, opts SetObjectLastCommittedLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		status    ObjectStatus
		expiresAt *time.Time
		updated   bool
	)

	err = p.db.QueryRowContext(ctx, `
		WITH pre_update_info AS (
			SELECT status, version, expires_at
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND status <> `+statusPending+`
			ORDER BY version DESC
			LIMIT 1
		), updated AS (
			UPDATE objects
			SET
				retention_mode = CASE
					WHEN $4 THEN COALESCE(retention_mode, 0) | `+retentionModeLegalHold+` -- Enable legal hold
					ELSE retention_mode & ~`+retentionModeLegalHold+` -- Disable legal hold
				END
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version IN (SELECT version FROM pre_update_info)
				AND status IN `+statusesCommitted+`
				AND expires_at IS NULL
			RETURNING 1
		)
		SELECT status, expires_at, EXISTS(SELECT 1 FROM updated) FROM pre_update_info`,
		opts.ProjectID,
		opts.BucketName,
		opts.ObjectKey,
		opts.Enabled,
	).Scan(&status, &expiresAt, &updated)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrObjectNotFound.New("")
		}
		return Error.New("unable to update object legal hold configuration: %w", err)
	}

	if !updated {
		switch {
		case status.IsDeleteMarker():
			return ErrObjectStatus.New(noLockOnDeleteMarkerErrMsg)
		case expiresAt != nil:
			return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
		default:
			return Error.New("unable to update object legal hold configuration")
		}
	}

	return nil
}

// SetObjectLastCommittedLegalHold sets the legal hold configuration
// of the most recently committed version of an object.
func (t *TiDBAdapter) SetObjectLastCommittedLegalHold(ctx context.Context, opts SetObjectLastCommittedLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	// The FOR UPDATE select folds BEGIN into its first statement and
	// CommitWithExec folds the UPDATE together with COMMIT, so this read then
	// dependent write costs two round trips instead of four.
	return tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
		var (
			status    ObjectStatus
			version   Version
			expiresAt *time.Time
		)
		err := tx.QueryRowContext(ctx, `
			SELECT status, version, expires_at
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = (?, ?, ?)
				AND status <> `+statusPending+`
			ORDER BY version DESC
			LIMIT 1
			FOR UPDATE`,
			opts.ProjectID, opts.BucketName, opts.ObjectKey,
		).Scan(&status, &version, &expiresAt)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrObjectNotFound.New("")
			}
			return Error.New("unable to query object info before setting legal hold: %w", err)
		}

		switch {
		case status.IsDeleteMarker():
			return ErrObjectStatus.New(noLockOnDeleteMarkerErrMsg)
		case expiresAt != nil:
			return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
		}

		if err = tx.CommitWithExec(ctx, `
			UPDATE objects SET
				retention_mode = CASE
					WHEN ? THEN COALESCE(retention_mode, 0) | `+retentionModeLegalHold+`
					ELSE retention_mode & ~`+retentionModeLegalHold+`
				END
			WHERE
				(project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`,
			opts.Enabled, opts.ProjectID, opts.BucketName, opts.ObjectKey, version,
		); err != nil {
			return Error.New("unable to update object legal hold configuration: %w", err)
		}
		return nil
	})
}

// SetObjectLastCommittedLegalHold sets the legal hold configuration
// of the most recently committed version of an object.
func (s *SpannerAdapter) SetObjectLastCommittedLegalHold(ctx context.Context, opts SetObjectLastCommittedLegalHold) (err error) {
	defer mon.Task()(&ctx)(&err)

	type info struct {
		version Version
		preUpdateRetentionInfo
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result, err := spannerutil.CollectRow(tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `
				SELECT status, version, expires_at, retention_mode
				FROM objects
				WHERE
					(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					AND status <> ` + statusPending + `
				ORDER BY version DESC
				LIMIT 1
			`,
			Params: map[string]any{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
			},
		}, spanner.QueryOptions{RequestTag: "set-object-last-committed-legal-hold-check"}), func(row *spanner.Row, item *info) error {
			return errs.Wrap(row.Columns(
				&item.Status,
				&item.version,
				&item.ExpiresAt,
				lockModeWrapper{retentionMode: &item.Retention.Mode},
			))
		})
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return ErrObjectNotFound.New("")
			}
			return errs.New("unable to query object info before setting legal hold: %w", err)
		}

		switch {
		case result.Status.IsDeleteMarker():
			return ErrObjectStatus.New(noLockOnDeleteMarkerErrMsg)
		case result.ExpiresAt != nil:
			return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
		}

		return errs.Wrap(s.setObjectExactVersionLegalHold(ctx, tx, SetObjectExactVersionLegalHold{
			ObjectLocation: opts.ObjectLocation,
			Version:        result.version,
			Enabled:        opts.Enabled,
		}, result.Retention.Mode))
	}, spanner.TransactionOptions{
		TransactionTag:              "set-object-last-committed-legal-hold",
		ExcludeTxnFromChangeStreams: true,
	})

	if err != nil {
		if ErrObjectNotFound.Has(err) || ErrObjectExpiration.Has(err) || ErrObjectStatus.Has(err) {
			return errs.Wrap(err)
		}
		return Error.Wrap(err)
	}

	return nil
}

func (s *SpannerAdapter) setObjectExactVersionLegalHold(ctx context.Context, tx *spanner.ReadWriteTransaction, opts SetObjectExactVersionLegalHold, existingRetMode storj.RetentionMode) (err error) {
	defer mon.Task()(&ctx)(&err)

	affected, err := tx.UpdateWithOptions(ctx, spanner.Statement{
		SQL: `
				UPDATE objects
				SET
					retention_mode = @retention_mode
				WHERE
					(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
			`,
		Params: map[string]any{
			"project_id":     opts.ProjectID,
			"bucket_name":    opts.BucketName,
			"object_key":     opts.ObjectKey,
			"version":        opts.Version,
			"retention_mode": lockModeWrapper{legalHold: &opts.Enabled, retentionMode: &existingRetMode},
		},
	}, spanner.QueryOptions{RequestTag: "set-object-last-committed-legal-hold"})
	if err != nil {
		return errs.New("unable to update object legal hold configuration: %w", err)
	}

	if affected == 0 {
		return ErrObjectNotFound.New("")
	}

	return nil
}

// SetObjectExactVersionRetention contains arguments necessary for setting
// the retention configuration of an exact version of an object.
type SetObjectExactVersionRetention struct {
	ObjectLocation
	Version Version

	Retention        Retention
	BypassGovernance bool
}

// Verify verifies the request fields.
func (opts *SetObjectExactVersionRetention) Verify() (err error) {
	if err = opts.ObjectLocation.Verify(); err != nil {
		return err
	}
	if err = opts.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}
	return nil
}

// SetObjectExactVersionRetention sets the retention configuration of an exact version of an object.
func (db *DB) SetObjectExactVersionRetention(ctx context.Context, opts SetObjectExactVersionRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	return db.ChooseAdapter(opts.ProjectID).SetObjectExactVersionRetention(ctx, opts)
}

// SetObjectExactVersionRetention sets the retention configuration of an exact version of an object.
func (p *PostgresAdapter) SetObjectExactVersionRetention(ctx context.Context, opts SetObjectExactVersionRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		info    preUpdateRetentionInfo
		updated bool
	)
	now := time.Now().Truncate(time.Microsecond)

	err = p.db.QueryRowContext(ctx, `
		WITH pre_update_info AS (
			SELECT status, expires_at, retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
		), updated AS (
			UPDATE objects
			SET
				retention_mode = CASE
					WHEN $6 != `+retentionModeNone+` THEN (COALESCE(retention_mode, `+retentionModeNone+`) & ~`+retentionModeComplianceAndGovernanceMask+`) | $6
					ELSE retention_mode & ~`+retentionModeComplianceAndGovernanceMask+`
				END,
				retain_until   = $7
			WHERE
				(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
				AND status IN `+statusesCommitted+`
				AND expires_at IS NULL
				AND CASE
					WHEN COALESCE(retention_mode, `+retentionModeNone+`) & `+retentionModeComplianceAndGovernanceMask+` = 0 THEN TRUE
					WHEN
						-- Invalid retention configuration
						retention_mode & `+retentionModeComplianceAndGovernanceMask+` NOT IN `+retentionModesComplianceAndGovernance+`
						OR retain_until IS NULL
						THEN FALSE
					ELSE
						retention_mode & `+retentionModeGovernance+` != 0 AND $5 -- Governance bypass
						OR CASE
							-- Removal is only allowed if the period has expired.
							WHEN $7::TIMESTAMPTZ IS NULL THEN retain_until <= $8
							-- Alteration is only allowed if the period isn't being shortened
							-- and the mode isn't being changed.
							ELSE retain_until <= $7 AND retention_mode & `+retentionModeComplianceAndGovernanceMask+` = $6
						END
				END
			RETURNING 1
		)
		SELECT *, EXISTS(SELECT 1 FROM updated) FROM pre_update_info`,
		opts.ProjectID,
		opts.BucketName,
		opts.ObjectKey,
		opts.Version,
		opts.BypassGovernance,
		lockModeWrapper{retentionMode: &opts.Retention.Mode},
		timeWrapper{&opts.Retention.RetainUntil},
		now,
	).Scan(
		&info.Status,
		&info.ExpiresAt,
		lockModeWrapper{retentionMode: &info.Retention.Mode},
		timeWrapper{&info.Retention.RetainUntil},
		&updated,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrObjectNotFound.New("")
		}
		return Error.New("unable to update object retention configuration: %w", err)
	}

	if !updated {
		if err = info.verify(opts.Retention, opts.BypassGovernance, now); err != nil {
			return errs.Wrap(err)
		}
		return Error.New("unable to update object retention configuration")
	}

	return nil
}

// SetObjectExactVersionRetention sets the retention configuration of an exact version of an object.
func (t *TiDBAdapter) SetObjectExactVersionRetention(ctx context.Context, opts SetObjectExactVersionRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().Truncate(time.Microsecond)

	return txutil.WithTx(ctx, t.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		var info preUpdateRetentionInfo
		err := tx.QueryRowContext(ctx, `
			SELECT status, expires_at, retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)
			FOR UPDATE`,
			opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
		).Scan(
			&info.Status,
			&info.ExpiresAt,
			lockModeWrapper{retentionMode: &info.Retention.Mode},
			timeWrapper{&info.Retention.RetainUntil},
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrObjectNotFound.New("")
			}
			return Error.New("unable to query object info before setting retention: %w", err)
		}

		if err = info.verify(opts.Retention, opts.BypassGovernance, now); err != nil {
			return errs.Wrap(err)
		}

		if _, err = tx.ExecContext(ctx, `
			UPDATE objects SET
				retention_mode = CASE
					WHEN ? != `+retentionModeNone+` THEN (COALESCE(retention_mode, `+retentionModeNone+`) & ~`+retentionModeComplianceAndGovernanceMask+`) | ?
					ELSE retention_mode & ~`+retentionModeComplianceAndGovernanceMask+`
				END,
				retain_until = ?
			WHERE
				(project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`,
			lockModeWrapper{retentionMode: &opts.Retention.Mode},
			lockModeWrapper{retentionMode: &opts.Retention.Mode},
			timeWrapper{&opts.Retention.RetainUntil},
			opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
		); err != nil {
			return Error.New("unable to update object retention configuration: %w", err)
		}
		return nil
	})
}

// SetObjectExactVersionRetention sets the retention configuration of an exact version of an object.
func (s *SpannerAdapter) SetObjectExactVersionRetention(ctx context.Context, opts SetObjectExactVersionRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now()

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result, err := spannerutil.CollectRow(tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `
				SELECT status, expires_at, retention_mode, retain_until
				FROM objects
				WHERE (project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
			`,
			Params: map[string]any{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     opts.Version,
			},
		}, spanner.QueryOptions{RequestTag: "set-object-exact-version-retention-check"}),
			func(row *spanner.Row, item *preUpdateRetentionInfo) error {
				return errs.Wrap(row.Columns(
					&item.Status,
					&item.ExpiresAt,
					lockModeWrapper{retentionMode: &item.Retention.Mode},
					timeWrapper{&item.Retention.RetainUntil},
				))
			})
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return ErrObjectNotFound.New("")
			}
			return errs.New("unable to query object info before setting retention: %w", err)
		}

		if err = result.verify(opts.Retention, opts.BypassGovernance, now); err != nil {
			return errs.Wrap(err)
		}

		return errs.Wrap(s.setObjectExactVersionRetention(ctx, tx, opts))
	}, spanner.TransactionOptions{
		TransactionTag:              "set-object-exact-version-retention",
		ExcludeTxnFromChangeStreams: true,
	})

	if err != nil {
		if ErrObjectNotFound.Has(err) || ErrObjectExpiration.Has(err) || ErrObjectLock.Has(err) || ErrObjectStatus.Has(err) {
			return errs.Wrap(err)
		}
		return Error.Wrap(err)
	}

	return nil
}

func (s *SpannerAdapter) setObjectExactVersionRetention(ctx context.Context, tx *spanner.ReadWriteTransaction, opts SetObjectExactVersionRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	affected, err := tx.UpdateWithOptions(ctx, spanner.Statement{
		SQL: `
			UPDATE objects
			SET
				retention_mode = CASE
					WHEN @retention_mode != ` + retentionModeNone + ` THEN (COALESCE(retention_mode, ` + retentionModeNone + `) & ~` + retentionModeComplianceAndGovernanceMask + `) | @retention_mode
					ELSE retention_mode & ~` + retentionModeComplianceAndGovernanceMask + `
				END,
				retain_until   = @retain_until
			WHERE
				(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
		`,
		Params: map[string]any{
			"project_id":     opts.ProjectID,
			"bucket_name":    opts.BucketName,
			"object_key":     opts.ObjectKey,
			"version":        opts.Version,
			"retention_mode": lockModeWrapper{retentionMode: &opts.Retention.Mode},
			"retain_until":   timeWrapper{&opts.Retention.RetainUntil},
		},
	}, spanner.QueryOptions{RequestTag: "set-object-exact-version-retention"})
	if err != nil {
		return errs.New("unable to update object retention configuration: %w", err)
	}

	if affected == 0 {
		return ErrObjectNotFound.New("")
	}

	return nil
}

// SetObjectLastCommittedRetention contains arguments necessary for setting
// the retention configuration of the most recently committed version of an object.
type SetObjectLastCommittedRetention struct {
	ObjectLocation

	Retention        Retention
	BypassGovernance bool
}

// Verify verifies the request fields.
func (opts SetObjectLastCommittedRetention) Verify() (err error) {
	if err = opts.ObjectLocation.Verify(); err != nil {
		return err
	}
	if err = opts.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}
	return nil
}

// SetObjectLastCommittedRetention sets the retention configuration
// of the most recently committed version of an object.
func (db *DB) SetObjectLastCommittedRetention(ctx context.Context, opts SetObjectLastCommittedRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	return db.ChooseAdapter(opts.ProjectID).SetObjectLastCommittedRetention(ctx, opts)
}

// SetObjectLastCommittedRetention sets the retention configuration
// of the most recently committed version of an object.
func (p *PostgresAdapter) SetObjectLastCommittedRetention(ctx context.Context, opts SetObjectLastCommittedRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		info    preUpdateRetentionInfo
		updated bool
	)
	now := time.Now().Truncate(time.Microsecond)

	err = p.db.QueryRowContext(ctx, `
		WITH pre_update_info AS (
			SELECT status, version, expires_at, retention_mode, retain_until
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND status <> `+statusPending+`
			ORDER BY version DESC
			LIMIT 1
		), updated AS (
			UPDATE objects
			SET
				retention_mode = CASE
					WHEN $5 != `+retentionModeNone+` THEN (COALESCE(retention_mode, `+retentionModeNone+`) & ~`+retentionModeComplianceAndGovernanceMask+`) | $5
					ELSE retention_mode & ~`+retentionModeComplianceAndGovernanceMask+`
				END,
				retain_until   = $6
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version IN (SELECT version FROM pre_update_info)
				AND status IN `+statusesCommitted+`
				AND expires_at IS NULL
				AND CASE
					WHEN COALESCE(retention_mode, `+retentionModeNone+`) & `+retentionModeComplianceAndGovernanceMask+` = 0 THEN TRUE
					WHEN
						-- Invalid retention configuration
						retention_mode & `+retentionModeComplianceAndGovernanceMask+` NOT IN `+retentionModesComplianceAndGovernance+`
						OR retain_until IS NULL
						THEN FALSE
					ELSE
						retention_mode & `+retentionModeGovernance+` != 0 AND $4 -- Governance bypass
						OR CASE
							-- Removal is only allowed if the period has expired.
							WHEN $6::TIMESTAMPTZ IS NULL THEN retain_until <= $7
							-- Alteration is only allowed if the period isn't being shortened
							-- and the mode isn't being changed.
							ELSE retain_until <= $6 AND retention_mode & `+retentionModeComplianceAndGovernanceMask+` = $5
						END
				END
			RETURNING 1
		)
		SELECT status, expires_at, retention_mode, retain_until, EXISTS(SELECT * FROM updated) from pre_update_info`,
		opts.ProjectID,
		opts.BucketName,
		opts.ObjectKey,
		opts.BypassGovernance,
		lockModeWrapper{retentionMode: &opts.Retention.Mode},
		timeWrapper{&opts.Retention.RetainUntil},
		now,
	).Scan(
		&info.Status,
		&info.ExpiresAt,
		lockModeWrapper{retentionMode: &info.Retention.Mode},
		timeWrapper{&info.Retention.RetainUntil},
		&updated,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrObjectNotFound.New("")
		}
		return Error.New("unable to update object retention configuration: %w", err)
	}

	if !updated {
		if err = info.verify(opts.Retention, opts.BypassGovernance, now); err != nil {
			return errs.Wrap(err)
		}
		return Error.New("unable to update object retention configuration")
	}

	return nil
}

// SetObjectLastCommittedRetention sets the retention configuration
// of the most recently committed version of an object.
func (t *TiDBAdapter) SetObjectLastCommittedRetention(ctx context.Context, opts SetObjectLastCommittedRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().Truncate(time.Microsecond)

	return txutil.WithTx(ctx, t.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		var (
			info    preUpdateRetentionInfo
			version Version
		)
		err := tx.QueryRowContext(ctx, `
			SELECT status, version, expires_at, retention_mode, retain_until
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = (?, ?, ?)
				AND status <> `+statusPending+`
			ORDER BY version DESC
			LIMIT 1
			FOR UPDATE`,
			opts.ProjectID, opts.BucketName, opts.ObjectKey,
		).Scan(
			&info.Status,
			&version,
			&info.ExpiresAt,
			lockModeWrapper{retentionMode: &info.Retention.Mode},
			timeWrapper{&info.Retention.RetainUntil},
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrObjectNotFound.New("")
			}
			return Error.New("unable to query object info before setting retention: %w", err)
		}

		if err = info.verify(opts.Retention, opts.BypassGovernance, now); err != nil {
			return errs.Wrap(err)
		}

		if _, err = tx.ExecContext(ctx, `
			UPDATE objects SET
				retention_mode = CASE
					WHEN ? != `+retentionModeNone+` THEN (COALESCE(retention_mode, `+retentionModeNone+`) & ~`+retentionModeComplianceAndGovernanceMask+`) | ?
					ELSE retention_mode & ~`+retentionModeComplianceAndGovernanceMask+`
				END,
				retain_until = ?
			WHERE
				(project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`,
			lockModeWrapper{retentionMode: &opts.Retention.Mode},
			lockModeWrapper{retentionMode: &opts.Retention.Mode},
			timeWrapper{&opts.Retention.RetainUntil},
			opts.ProjectID, opts.BucketName, opts.ObjectKey, version,
		); err != nil {
			return Error.New("unable to update object retention configuration: %w", err)
		}
		return nil
	})
}

// SetObjectLastCommittedRetention sets the retention configuration
// of the most recently committed version of an object.
func (s *SpannerAdapter) SetObjectLastCommittedRetention(ctx context.Context, opts SetObjectLastCommittedRetention) (err error) {
	defer mon.Task()(&ctx)(&err)

	type info struct {
		version Version
		preUpdateRetentionInfo
	}

	now := time.Now()

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result, err := spannerutil.CollectRow(tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `
				SELECT status, version, expires_at, retention_mode, retain_until
				FROM objects
				WHERE
					(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					AND status <> ` + statusPending + `
				ORDER BY version DESC
				LIMIT 1
			`,
			Params: map[string]any{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
			},
		}, spanner.QueryOptions{RequestTag: "set-object-last-committed-retention-check"}), func(row *spanner.Row, item *info) error {
			return errs.Wrap(row.Columns(
				&item.Status,
				&item.version,
				&item.ExpiresAt,
				lockModeWrapper{retentionMode: &item.Retention.Mode},
				timeWrapper{&item.Retention.RetainUntil},
			))
		})
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return ErrObjectNotFound.New("")
			}
			return errs.New("unable to query object info before setting retention: %w", err)
		}

		if err = result.verify(opts.Retention, opts.BypassGovernance, now); err != nil {
			return errs.Wrap(err)
		}

		return errs.Wrap(s.setObjectExactVersionRetention(ctx, tx, SetObjectExactVersionRetention{
			ObjectLocation: opts.ObjectLocation,
			Version:        result.version,
			Retention:      opts.Retention,
		}))
	}, spanner.TransactionOptions{
		TransactionTag:              "set-object-last-committed-retention",
		ExcludeTxnFromChangeStreams: true,
	})

	if err != nil {
		if ErrObjectNotFound.Has(err) || ErrObjectExpiration.Has(err) || ErrObjectLock.Has(err) || ErrObjectStatus.Has(err) {
			return errs.Wrap(err)
		}
		return Error.Wrap(err)
	}

	return nil
}

// preUpdateRetentionInfo contains information about an object that is collected
// before the object's retention configuration is updated.
type preUpdateRetentionInfo struct {
	Status    ObjectStatus
	ExpiresAt *time.Time
	Retention Retention
}

// verify returns an error if the object's retention shouldn't be updated.
func (info *preUpdateRetentionInfo) verify(newRetention Retention, bypassGovernance bool, now time.Time) error {
	switch {
	case info.Status.IsDeleteMarker():
		return ErrObjectStatus.New(noLockOnDeleteMarkerErrMsg)
	case !info.Status.IsCommitted():
		return ErrObjectStatus.New(noLockOnUncommittedErrMsg)
	case info.ExpiresAt != nil:
		return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
	}

	if err := info.Retention.Verify(); err != nil {
		return errs.Wrap(err)
	}

	if info.Retention.Active(now) && !(info.Retention.Mode == storj.GovernanceMode && bypassGovernance) {
		switch {
		case !newRetention.Enabled():
			return ErrObjectLock.New(noRemoveRetentionErrMsg)
		case info.Retention.Mode != newRetention.Mode:
			return ErrObjectLock.New(noChangeRetentionModeErrMsg)
		case newRetention.RetainUntil.Before(info.Retention.RetainUntil):
			return ErrObjectLock.New(noShortenRetentionErrMsg)
		}
	}

	return nil
}
