// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// transitionSegmentsSeedInline inserts a committed object together with a single
// inline segment directly into the given backend, bypassing the transition
// routing. Inline segments avoid node-alias bookkeeping, so they are convenient
// for exercising the read-routing paths. The segment carries an EncryptedKey so
// that GetSegmentPositionsAndKeys returns an entry for it.
func transitionSegmentsSeedInline(ctx *testcontext.Context, t *testing.T, db *metabase.DB, adapter metabase.Adapter, projectID uuid.UUID, key string) metabase.ObjectStream {
	t.Helper()

	stream := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: "bucket",
		ObjectKey:  metabase.ObjectKey(key),
		Version:    1,
		StreamID:   testrand.UUID(),
	}

	require.NoError(t, adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.CommittedUnversioned,
		Encryption:   metabasetest.DefaultEncryption,
		SegmentCount: 1,
	}}))

	cache := metabase.NewNodeAliasCache(db, false)
	require.NoError(t, adapter.TestingBatchInsertSegments(ctx, cache, []metabase.RawSegment{{
		StreamID:          stream.StreamID,
		Position:          metabase.SegmentPosition{Index: 0},
		RootPieceID:       testrand.PieceID(),
		EncryptedKeyNonce: testrand.Bytes(16),
		EncryptedKey:      testrand.Bytes(16),
		EncryptedSize:     1024,
		PlainSize:         1024,
		InlineData:        testrand.Bytes(1024),
	}}))

	return stream
}

// transitionSegmentsSeedRemote inserts a committed object together with a single
// remote segment (one piece) directly into the given backend, bypassing the
// transition routing. It returns the stream and the pieces stored, so callers
// can build the matching old/new alias pieces for piece-update tests.
func transitionSegmentsSeedRemote(ctx *testcontext.Context, t *testing.T, db *metabase.DB, adapter metabase.Adapter, projectID uuid.UUID, key string) (metabase.ObjectStream, metabase.Pieces) {
	t.Helper()

	stream := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: "bucket",
		ObjectKey:  metabase.ObjectKey(key),
		Version:    1,
		StreamID:   testrand.UUID(),
	}

	require.NoError(t, adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.CommittedUnversioned,
		Encryption:   metabasetest.DefaultEncryption,
		SegmentCount: 1,
	}}))

	pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}

	cache := metabase.NewNodeAliasCache(db, false)
	require.NoError(t, adapter.TestingBatchInsertSegments(ctx, cache, []metabase.RawSegment{{
		StreamID:          stream.StreamID,
		Position:          metabase.SegmentPosition{Index: 0},
		RootPieceID:       testrand.PieceID(),
		EncryptedKeyNonce: testrand.Bytes(16),
		EncryptedKey:      testrand.Bytes(16),
		EncryptedSize:     1024,
		PlainSize:         1024,
		Redundancy:        metabasetest.DefaultRedundancy,
		Pieces:            pieces,
	}}))

	return stream, pieces
}

// transitionSegmentsAliases converts pieces into the alias-piece representation
// shared by both backends. Node aliases live in the primary backend, so a single
// shared cache built on db resolves consistently for either backend.
func transitionSegmentsAliases(ctx *testcontext.Context, t *testing.T, db *metabase.DB, pieces metabase.Pieces) metabase.AliasPieces {
	t.Helper()
	cache := metabase.NewNodeAliasCache(db, false)
	aliases, err := cache.EnsurePiecesToAliases(ctx, pieces)
	require.NoError(t, err)
	return aliases
}

func TestTransitionSegments_GetSegmentByPosition(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)
		pos := metabase.SegmentPosition{Index: 0}

		t.Run("in primary", func(t *testing.T) {
			stream := transitionSegmentsSeedInline(ctx, t, db, primary, projectID, "p")
			seg, _, err := adapter.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1024, seg.EncryptedSize)
		})

		t.Run("in secondary (fallback)", func(t *testing.T) {
			stream := transitionSegmentsSeedInline(ctx, t, db, secondary, projectID, "s")
			seg, _, err := adapter.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1024, seg.EncryptedSize)
		})

		t.Run("in both (prefer primary)", func(t *testing.T) {
			// same stream id seeded into both backends during a relocation window;
			// the primary copy must win.
			stream := metabase.ObjectStream{
				ProjectID:  projectID,
				BucketName: "bucket",
				ObjectKey:  "b",
				Version:    1,
				StreamID:   testrand.UUID(),
			}
			for i, a := range []metabase.Adapter{primary, secondary} {
				require.NoError(t, a.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
					ObjectStream: stream,
					Status:       metabase.CommittedUnversioned,
					Encryption:   metabasetest.DefaultEncryption,
					SegmentCount: 1,
				}}))
				cache := metabase.NewNodeAliasCache(db, false)
				// distinct encrypted sizes let us prove which backend answered.
				size := int32(100 + i)
				require.NoError(t, a.TestingBatchInsertSegments(ctx, cache, []metabase.RawSegment{{
					StreamID:          stream.StreamID,
					Position:          pos,
					RootPieceID:       testrand.PieceID(),
					EncryptedKeyNonce: testrand.Bytes(16),
					EncryptedKey:      testrand.Bytes(16),
					EncryptedSize:     size,
					PlainSize:         size,
					InlineData:        testrand.Bytes(memory.Size(size)),
				}}))
			}

			seg, _, err := adapter.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.NoError(t, err)
			require.EqualValues(t, 100, seg.EncryptedSize, "primary copy must win")
		})

		t.Run("in neither", func(t *testing.T) {
			_, _, err := adapter.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: testrand.UUID(),
				Position: pos,
			})
			require.True(t, metabase.ErrSegmentNotFound.Has(err))
		})
	})
}

func TestTransitionSegments_GetSegmentByPositionForAudit(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)
		pos := metabase.SegmentPosition{Index: 0}

		t.Run("in primary", func(t *testing.T) {
			stream, _ := transitionSegmentsSeedRemote(ctx, t, db, primary, projectID, "p")
			seg, _, err := adapter.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1024, seg.EncryptedSize)
		})

		t.Run("in secondary (fallback)", func(t *testing.T) {
			stream, _ := transitionSegmentsSeedRemote(ctx, t, db, secondary, projectID, "s")
			seg, _, err := adapter.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1024, seg.EncryptedSize)
		})

		t.Run("in neither", func(t *testing.T) {
			_, _, err := adapter.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
				StreamID: testrand.UUID(),
				Position: pos,
			})
			require.True(t, metabase.ErrSegmentNotFound.Has(err))
		})
	})
}

func TestTransitionSegments_GetSegmentByPositionForRepair(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)
		pos := metabase.SegmentPosition{Index: 0}

		t.Run("in primary", func(t *testing.T) {
			stream, _ := transitionSegmentsSeedRemote(ctx, t, db, primary, projectID, "p")
			seg, _, err := adapter.GetSegmentByPositionForRepair(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1024, seg.EncryptedSize)
		})

		t.Run("in secondary (fallback)", func(t *testing.T) {
			stream, _ := transitionSegmentsSeedRemote(ctx, t, db, secondary, projectID, "s")
			seg, _, err := adapter.GetSegmentByPositionForRepair(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1024, seg.EncryptedSize)
		})

		t.Run("in neither", func(t *testing.T) {
			_, _, err := adapter.GetSegmentByPositionForRepair(ctx, metabase.GetSegmentByPosition{
				StreamID: testrand.UUID(),
				Position: pos,
			})
			require.True(t, metabase.ErrSegmentNotFound.Has(err))
		})
	})
}

func TestTransitionSegments_GetSegmentsByPosition(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)
		pos := metabase.SegmentPosition{Index: 0}

		t.Run("in primary", func(t *testing.T) {
			stream := transitionSegmentsSeedInline(ctx, t, db, primary, projectID, "p")
			key := metabase.SegmentPositionKey{StreamID: stream.StreamID, Position: pos}
			segments, _, err := adapter.GetSegmentsByPosition(ctx, metabase.GetSegmentsByPosition{
				Keys: []metabase.SegmentPositionKey{key},
			})
			require.NoError(t, err)
			require.Contains(t, segments, key)
		})

		t.Run("in secondary (fallback on empty primary)", func(t *testing.T) {
			stream := transitionSegmentsSeedInline(ctx, t, db, secondary, projectID, "s")
			key := metabase.SegmentPositionKey{StreamID: stream.StreamID, Position: pos}
			segments, _, err := adapter.GetSegmentsByPosition(ctx, metabase.GetSegmentsByPosition{
				Keys: []metabase.SegmentPositionKey{key},
			})
			require.NoError(t, err)
			require.Contains(t, segments, key)
		})
	})
}

func TestTransitionSegments_GetSegmentPositionsAndKeys(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)

		t.Run("in primary", func(t *testing.T) {
			stream := transitionSegmentsSeedInline(ctx, t, db, primary, projectID, "p")
			keys, err := adapter.GetSegmentPositionsAndKeys(ctx, stream.StreamID)
			require.NoError(t, err)
			require.Len(t, keys, 1)
		})

		t.Run("in secondary (fallback on empty primary)", func(t *testing.T) {
			stream := transitionSegmentsSeedInline(ctx, t, db, secondary, projectID, "s")
			keys, err := adapter.GetSegmentPositionsAndKeys(ctx, stream.StreamID)
			require.NoError(t, err)
			require.Len(t, keys, 1)
		})
	})
}

func TestTransitionSegments_GetStreamPieceCountByAlias(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)

		t.Run("in primary", func(t *testing.T) {
			stream, _ := transitionSegmentsSeedRemote(ctx, t, db, primary, projectID, "p")
			counts, err := adapter.GetStreamPieceCountByAlias(ctx, metabase.GetStreamPieceCountByNodeID{
				ProjectID: projectID,
				StreamID:  stream.StreamID,
			})
			require.NoError(t, err)
			require.Len(t, counts, 1)
		})

		t.Run("in secondary (fallback on empty primary)", func(t *testing.T) {
			stream, _ := transitionSegmentsSeedRemote(ctx, t, db, secondary, projectID, "s")
			counts, err := adapter.GetStreamPieceCountByAlias(ctx, metabase.GetStreamPieceCountByNodeID{
				ProjectID: projectID,
				StreamID:  stream.StreamID,
			})
			require.NoError(t, err)
			require.Len(t, counts, 1)
		})
	})
}

func TestTransitionSegments_UpdateSegmentPieces(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)
		pos := metabase.SegmentPosition{Index: 0}

		// segmentPieces reads back the piece set stored on the segment in the
		// given backend, so we can assert which backend was mutated.
		segmentPieces := func(a metabase.Adapter, streamID uuid.UUID) metabase.Pieces {
			_, aliasPieces, err := a.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: streamID,
				Position: pos,
			})
			require.NoError(t, err)
			cache := metabase.NewNodeAliasCache(db, false)
			pieces, err := cache.ConvertAliasesToPieces(ctx, aliasPieces)
			require.NoError(t, err)
			return pieces
		}

		runUpdate := func(t *testing.T, stream metabase.ObjectStream, oldPieces metabase.Pieces) metabase.Pieces {
			newPieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			oldAlias := transitionSegmentsAliases(ctx, t, db, oldPieces)
			newAlias := transitionSegmentsAliases(ctx, t, db, newPieces)
			_, err := adapter.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
				StreamID:      stream.StreamID,
				Position:      pos,
				OldPieces:     oldPieces,
				NewRedundancy: metabasetest.DefaultRedundancy,
				NewPieces:     newPieces,
			}, oldAlias, newAlias)
			require.NoError(t, err)
			return newPieces
		}

		t.Run("in primary", func(t *testing.T) {
			stream, oldPieces := transitionSegmentsSeedRemote(ctx, t, db, primary, projectID, "p")
			newPieces := runUpdate(t, stream, oldPieces)
			require.Equal(t, newPieces, segmentPieces(primary, stream.StreamID), "primary must be updated")
		})

		t.Run("in secondary (fallback)", func(t *testing.T) {
			stream, oldPieces := transitionSegmentsSeedRemote(ctx, t, db, secondary, projectID, "s")
			newPieces := runUpdate(t, stream, oldPieces)
			require.Equal(t, newPieces, segmentPieces(secondary, stream.StreamID), "secondary must be updated")

			// the stream is absent from primary, so nothing leaked there.
			_, _, err := primary.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.True(t, metabase.ErrSegmentNotFound.Has(err))
		})
	})
}

func TestTransitionSegments_BatchUpdateSegmentPieces(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)
		pos := metabase.SegmentPosition{Index: 0}

		segmentPieceCount := func(a metabase.Adapter, streamID uuid.UUID) int {
			_, aliasPieces, err := a.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: streamID,
				Position: pos,
			})
			require.NoError(t, err)
			return len(aliasPieces)
		}

		runBatch := func(t *testing.T, stream metabase.ObjectStream) []bool {
			newPieces := metabase.Pieces{
				{Number: 0, StorageNode: testrand.NodeID()},
				{Number: 1, StorageNode: testrand.NodeID()},
			}
			newAlias := transitionSegmentsAliases(ctx, t, db, newPieces)
			results, err := adapter.BatchUpdateSegmentPieces(ctx, metabase.BatchUpdateSegmentPieces{
				Entries: []metabase.BatchUpdateSegmentPiecesEntry{{
					StreamID:      stream.StreamID,
					Position:      pos,
					NewPieces:     newPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
				}},
			}, []metabase.AliasPieces{newAlias})
			require.NoError(t, err)
			return results
		}

		t.Run("in primary", func(t *testing.T) {
			stream, _ := transitionSegmentsSeedRemote(ctx, t, db, primary, projectID, "p")
			require.Equal(t, []bool{true}, runBatch(t, stream))
			require.Equal(t, 2, segmentPieceCount(primary, stream.StreamID), "primary must be updated")
		})

		t.Run("in secondary (no fallback)", func(t *testing.T) {
			// NOTE: unlike UpdateSegmentPieces, the per-adapter
			// BatchUpdateSegmentPieces reports a missing segment as a false result
			// rather than an ErrSegmentNotFound error. The transition adapter only
			// falls back to secondary on a not-found *error*, so a batch update for
			// a stream that lives only in secondary does NOT fall back: the primary
			// reports false, the secondary is never consulted, and the segment is
			// left untouched. This documents the current behavior of the routing.
			stream, oldPieces := transitionSegmentsSeedRemote(ctx, t, db, secondary, projectID, "s")
			require.Equal(t, []bool{false}, runBatch(t, stream))

			// the secondary copy is unchanged (still one piece).
			require.Equal(t, len(oldPieces), segmentPieceCount(secondary, stream.StreamID))

			// the stream never existed in primary.
			_, _, err := primary.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: stream.StreamID,
				Position: pos,
			})
			require.True(t, metabase.ErrSegmentNotFound.Has(err))
		})
	})
}
