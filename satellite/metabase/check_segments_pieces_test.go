// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestCheckSegmentPiecesAlteration(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		// Create a segment with some pieces
		originalPieces := metabase.Pieces{
			{Number: 0, StorageNode: testrand.NodeID()},
			{Number: 1, StorageNode: testrand.NodeID()},
		}

		// Begin and commit the segment
		metabasetest.BeginObjectExactVersion{
			Opts: metabase.BeginObjectExactVersion{
				ObjectStream: obj,
				Encryption:   metabasetest.DefaultEncryption,
			},
		}.Check(ctx, t, db)

		metabasetest.BeginSegment{
			Opts: metabase.BeginSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: 0},
				RootPieceID:  storj.PieceID{1},
				Pieces:       originalPieces,
			},
		}.Check(ctx, t, db)

		metabasetest.CommitSegment{
			Opts: metabase.CommitSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: 0},
				RootPieceID:  storj.PieceID{1},
				Pieces:       originalPieces,

				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},

				EncryptedSize: 1024,
				PlainSize:     512,
				PlainOffset:   0,
				Redundancy:    metabasetest.DefaultRedundancy,
			},
		}.Check(ctx, t, db)

		metabasetest.CommitInlineSegment{
			Opts: metabase.CommitInlineSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: 1},

				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},

				PlainSize:   512,
				PlainOffset: 0,
			},
		}.Check(ctx, t, db)

		// Create different pieces for testing
		differentPieces := metabase.Pieces{
			{Number: 0, StorageNode: testrand.NodeID()},
			{Number: 1, StorageNode: testrand.NodeID()},
		}

		nonExistentStreamID := testrand.UUID()
		require.NotEqual(t, nonExistentStreamID, obj.StreamID, "random UUID clash")

		testCases := []struct {
			name          string
			streamID      uuid.UUID
			position      metabase.SegmentPosition
			pieces        metabase.Pieces
			expectAltered bool
			expectedErr   *errs.Class
		}{
			{
				name:          "same pieces - not altered",
				streamID:      obj.StreamID,
				position:      metabase.SegmentPosition{Part: 0, Index: 0},
				pieces:        originalPieces,
				expectAltered: false,
			},
			{
				name:          "different pieces - altered",
				streamID:      obj.StreamID,
				position:      metabase.SegmentPosition{Part: 0, Index: 0},
				pieces:        differentPieces,
				expectAltered: true,
			},
			{
				name:        "non-existent stream ID",
				streamID:    nonExistentStreamID,
				position:    metabase.SegmentPosition{Part: 0, Index: 0},
				pieces:      originalPieces,
				expectedErr: &metabase.ErrSegmentNotFound,
			},
			{
				name:        "non-existent segment position part",
				streamID:    obj.StreamID,
				position:    metabase.SegmentPosition{Part: 1, Index: 0},
				pieces:      originalPieces,
				expectedErr: &metabase.ErrSegmentNotFound,
			},
			{
				name:        "non-existent segment position index",
				streamID:    obj.StreamID,
				position:    metabase.SegmentPosition{Part: 0, Index: 10},
				pieces:      originalPieces,
				expectedErr: &metabase.ErrSegmentNotFound,
			},
			{
				name:        "zero stream ID validation error",
				streamID:    uuid.UUID{},
				position:    metabase.SegmentPosition{Part: 0, Index: 0},
				pieces:      originalPieces,
				expectedErr: &metabase.ErrInvalidRequest,
			},
			{
				name:        "no passed pieces (NULL)",
				streamID:    obj.StreamID,
				position:    metabase.SegmentPosition{Part: 0, Index: 0},
				expectedErr: &metabase.ErrInvalidRequest,
			},
			{
				name:        "no passed pieces (0 length)",
				streamID:    obj.StreamID,
				position:    metabase.SegmentPosition{Part: 0, Index: 0},
				expectedErr: &metabase.ErrInvalidRequest,
			},
			{
				name:        "inline segment",
				streamID:    obj.StreamID,
				position:    metabase.SegmentPosition{Part: 0, Index: 1},
				pieces:      originalPieces,
				expectedErr: &metabase.ErrInvalidRequest,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				altered, err := db.CheckSegmentPiecesAlteration(ctx, tc.streamID, tc.position, tc.pieces)

				if tc.expectedErr != nil {
					require.Error(t, err, "expected error for test case: %s", tc.name)
					require.True(t, tc.expectedErr.Has(err),
						"expected error class '%v' but got %v for test case: %s", *tc.expectedErr, err, tc.name,
					)
				} else {
					require.NoError(t, err, "unexpected error for test case: %s", tc.name)
					require.Equal(t, tc.expectAltered, altered,
						"unexpected alteration result for test case: %s", tc.name,
					)
				}
			})
		}
	})
}
