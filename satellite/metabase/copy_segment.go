// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// FinishCopySegments holds all data needed to finish copy segments.
type FinishCopySegments struct {
	// target pending object to copy segments to
	ObjectStream

	// StreamID of the source object to copy segments from.
	SourceStreamID uuid.UUID

	StartOffset int64
	EndOffset   int64
	PartNumber  uint32

	NewSegmentKeys []EncryptedKeyAndNonce

	// supported only by Spanner.
	MaxCommitDelay *time.Duration
}

// Verify checks if the FinishCopySegments options are valid.
func (opts FinishCopySegments) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	switch {
	case opts.SourceStreamID.IsZero():
		return Error.New("SourceStreamID must be specified")
	case opts.StartOffset < 0:
		return Error.New("StartOffset must be non-negative")
	case opts.EndOffset <= opts.StartOffset:
		return Error.New("EndOffset must be greater than StartOffset")
	case opts.PartNumber == 0:
		return Error.New("PartNumber must be greater than zero")
	case len(opts.NewSegmentKeys) == 0:
		return Error.New("NewSegmentKeys must not be empty")
	}

	// TODO add more

	return nil
}

// FinishCopySegments collects all data needed to finish copy segments procedure.
func (db *DB) FinishCopySegments(ctx context.Context, opts FinishCopySegments) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	err = db.ChooseAdapter(opts.ProjectID).FinishCopySegments(ctx, opts)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// FinishCopySegments collects all data needed to finish copy segments procedure.
func (p *PostgresAdapter) FinishCopySegments(ctx context.Context, opts FinishCopySegments) (err error) {
	defer mon.Task()(&ctx)(&err)

	return errs.New("unimplemented")
}

// FinishCopySegments collects all data needed to finish copy segments procedure.
func (s *SpannerAdapter) FinishCopySegments(ctx context.Context, opts FinishCopySegments) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO add limitation on number of segments to copy to 1000

	if err := opts.Verify(); err != nil {
		return Error.Wrap(err)
	}

	// TODO should we sort opts.NewSegmentKeys by position?

	firstSegmentPosition := opts.NewSegmentKeys[0].Position
	lastSegmentPosition := opts.NewSegmentKeys[len(opts.NewSegmentKeys)-1].Position

	segments := make([]RawSegment, 0, len(opts.NewSegmentKeys))
	aliasPieces := make([][]byte, 0, len(opts.NewSegmentKeys))

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		// check if the pending object exists and has the correct status
		row, err := tx.ReadRow(ctx,
			"objects",
			spanner.Key{opts.ProjectID, opts.BucketName, opts.ObjectKey, int64(opts.Version)},
			[]string{"stream_id", "status"},
		)
		if err != nil {
			if errors.Is(err, spanner.ErrRowNotFound) {
				return ErrPendingObjectMissing.New("")
			}
			return ErrFailedPrecondition.Wrap(err)
		}

		var streamID uuid.UUID
		var status int64
		err = row.Columns(&streamID, &status)
		if err != nil {
			return Error.Wrap(err)
		}

		if streamID != opts.StreamID || status != int64(Pending) {
			return ErrPendingObjectMissing.New("")
		}

		err = tx.Read(ctx, "segments", spanner.KeyRange{
			Start: spanner.Key{
				opts.SourceStreamID,
				firstSegmentPosition,
			},
			End: spanner.Key{
				opts.SourceStreamID,
				lastSegmentPosition,
			},
			Kind: spanner.ClosedClosed,
		},
			[]string{"position", "expires_at", "root_piece_id", "encrypted_size", "plain_offset",
				"plain_size", "redundancy", "remote_alias_pieces", "placement", "inline_data"}).Do(func(row *spanner.Row) error {
			var segment RawSegment
			var aliasPiecesRaw []byte
			err := row.Columns(
				&segment.Position, &segment.ExpiresAt, &segment.RootPieceID,
				spannerutil.Int(&segment.EncryptedSize), &segment.PlainOffset, spannerutil.Int(&segment.PlainSize),
				&segment.Redundancy, &aliasPiecesRaw, &segment.Placement,
				&segment.InlineData,
			)
			if err != nil {
				return err
			}
			segments = append(segments, segment)
			aliasPieces = append(aliasPieces, aliasPiecesRaw)
			return nil
		})
		if err != nil {
			return Error.New("could not read segments for copy: %w", err)
		}

		if len(segments) != len(opts.NewSegmentKeys) {
			return Error.New("number of provided segment keys doesn't match number of read segments: %d != %d", len(segments), len(opts.NewSegmentKeys))
		}

		firstSegment := segments[0]
		lastSegment := segments[len(segments)-1]
		switch {
		case firstSegment.PlainOffset != opts.StartOffset:
			return Error.New("first segment offset %v does not match expected start offset %v", firstSegment.PlainOffset, opts.StartOffset)
		case lastSegment.PlainOffset+int64(lastSegment.PlainSize) != opts.EndOffset:
			return Error.New("last segment end offset %v does not match expected end offset %v", lastSegment.PlainOffset+int64(lastSegment.PlainSize), opts.EndOffset)
		}

		mutations := make([]*spanner.Mutation, 0, len(segments))
		for i, segment := range segments {
			if segment.Position != opts.NewSegmentKeys[i].Position {
				return Error.New("segment position %v does not match expected position %v",
					segment.Position, opts.NewSegmentKeys[i].Position)
			}

			segment.Position.Part = opts.PartNumber

			mutations = append(mutations, spanner.Insert("segments",
				[]string{"stream_id", "position", "expires_at", "root_piece_id", "encrypted_size", "plain_offset",
					"plain_size", "redundancy", "remote_alias_pieces", "placement", "inline_data",
					"encrypted_key_nonce", "encrypted_key", "encrypted_etag"},
				[]any{opts.StreamID, segment.Position, segment.ExpiresAt, segment.RootPieceID,
					int64(segment.EncryptedSize), segment.PlainOffset, int64(segment.PlainSize),
					segment.Redundancy, aliasPieces[i], int64(segment.Placement), segment.InlineData,
					opts.NewSegmentKeys[i].EncryptedKeyNonce, opts.NewSegmentKeys[i].EncryptedKey, opts.NewSegmentKeys[i].EncryptedETag,
				},
			))
		}

		return tx.BufferWrite(mutations)
	}, spanner.TransactionOptions{
		TransactionTag: "finish-copy-segments",
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
	})

	return Error.Wrap(err)
}
