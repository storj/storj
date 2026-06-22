// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// transitionListStream builds a committed object stream for the listing tests.
func transitionListStream(projectID uuid.UUID, key string) metabase.ObjectStream {
	return metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: "bucket",
		ObjectKey:  metabase.ObjectKey(key),
		Version:    1,
		StreamID:   testrand.UUID(),
	}
}

// transitionListSeedSegments inserts a committed object plus inline segments at
// the given positions directly into the chosen backend, bypassing the
// transition routing. Inline segments are used to avoid node-alias complexity.
func transitionListSeedSegments(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter, stream metabase.ObjectStream, positions ...uint32) {
	require.NoError(t, adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.CommittedUnversioned,
		SegmentCount: int32(len(positions)),
		Encryption:   metabasetest.DefaultEncryption,
	}}))

	segments := make([]metabase.RawSegment, 0, len(positions))
	for _, idx := range positions {
		segments = append(segments, metabase.RawSegment{
			StreamID:          stream.StreamID,
			Position:          metabase.SegmentPosition{Index: idx},
			RootPieceID:       testrand.PieceID(),
			EncryptedKey:      []byte{3},
			EncryptedKeyNonce: []byte{4},
			EncryptedSize:     int32(len(positions)),
			PlainSize:         512,
			PlainOffset:       int64(idx) * 512,
			InlineData:        testrand.Bytes(16),
		})
	}

	// inline segments carry no pieces, so the alias cache is never consulted;
	// the adapter satisfies NodeAliasDB.
	cache := metabase.NewNodeAliasCache(adapter, false)
	require.NoError(t, adapter.TestingBatchInsertSegments(ctx, cache, segments))
}

// transitionListSeedRemoteSegments inserts a committed object plus remote
// segments (with pieces) directly into the given backend. Remote segments are
// required to exercise ListVerifySegments, which skips inline segments.
func transitionListSeedRemoteSegments(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter, stream metabase.ObjectStream, positions ...uint32) {
	require.NoError(t, adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.CommittedUnversioned,
		SegmentCount: int32(len(positions)),
		Encryption:   metabasetest.DefaultEncryption,
	}}))

	segments := make([]metabase.RawSegment, 0, len(positions))
	for _, idx := range positions {
		segments = append(segments, metabasetest.DefaultRawSegment(stream, metabase.SegmentPosition{Index: idx}))
	}

	cache := metabase.NewNodeAliasCache(adapter, false)
	require.NoError(t, adapter.TestingBatchInsertSegments(ctx, cache, segments))
}

// transitionListSegmentPositions extracts and sorts the positions present in a
// slice of segments for order-independent comparison.
func transitionListSegmentPositions(segments []metabase.Segment) []uint32 {
	out := make([]uint32, 0, len(segments))
	for _, s := range segments {
		out = append(out, s.Position.Index)
	}
	slices.Sort(out)
	return out
}

func transitionListStreamPositions(segments []metabase.SegmentPositionInfo) []uint32 {
	out := make([]uint32, 0, len(segments))
	for _, s := range segments {
		out = append(out, s.Position.Index)
	}
	slices.Sort(out)
	return out
}

func transitionListKeys(objects []metabase.ObjectEntry) []string {
	out := make([]string, 0, len(objects))
	for _, o := range objects {
		out = append(out, string(o.ObjectKey))
	}
	return out
}

func TestTransitionList_ListObjects(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		list := func(limit int, cursor string) metabase.ListObjectsResult {
			result, err := db.ListObjects(ctx, metabase.ListObjects{
				ProjectID:  projectID,
				BucketName: "bucket",
				Recursive:  true,
				Limit:      limit,
				Cursor:     metabase.ListObjectsCursor{Key: metabase.ObjectKey(cursor)},
			})
			require.NoError(t, err)
			return result
		}

		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "p1")
			transitionSeedCommitted(ctx, t, primary, projectID, "p2")

			result := list(10, "")
			require.False(t, result.More)
			require.Equal(t, []string{"p1", "p2"}, transitionListKeys(result.Objects))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, secondary, projectID, "s1")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s2")

			result := list(10, "")
			require.False(t, result.More)
			require.Equal(t, []string{"s1", "s2"}, transitionListKeys(result.Objects))
		})

		t.Run("split across both, merged and sorted", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// interleave so the merge has to sort across backends.
			transitionSeedCommitted(ctx, t, primary, projectID, "a")
			transitionSeedCommitted(ctx, t, secondary, projectID, "b")
			transitionSeedCommitted(ctx, t, primary, projectID, "c")
			transitionSeedCommitted(ctx, t, secondary, projectID, "d")

			result := list(10, "")
			require.False(t, result.More)
			require.Equal(t, []string{"a", "b", "c", "d"}, transitionListKeys(result.Objects))
		})

		t.Run("same key in both deduped, primary wins", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			primaryStream := transitionSeedCommitted(ctx, t, primary, projectID, "dup")
			secondaryStream := transitionSeedCommitted(ctx, t, secondary, projectID, "dup")
			require.NotEqual(t, primaryStream.StreamID, secondaryStream.StreamID)

			result := list(10, "")
			require.False(t, result.More)
			require.Len(t, result.Objects, 1)
			require.Equal(t, "dup", string(result.Objects[0].ObjectKey))
			require.Equal(t, primaryStream.StreamID, result.Objects[0].StreamID, "primary's stream must win")
		})

		t.Run("more results than limit truncates", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "k1")
			transitionSeedCommitted(ctx, t, secondary, projectID, "k2")
			transitionSeedCommitted(ctx, t, primary, projectID, "k3")
			transitionSeedCommitted(ctx, t, secondary, projectID, "k4")

			result := list(2, "")
			require.True(t, result.More)
			require.Equal(t, []string{"k1", "k2"}, transitionListKeys(result.Objects))
		})

		t.Run("cursor pagination covers all keys", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// keys spread across both backends.
			transitionSeedCommitted(ctx, t, primary, projectID, "c1")
			transitionSeedCommitted(ctx, t, secondary, projectID, "c2")
			transitionSeedCommitted(ctx, t, primary, projectID, "c3")
			transitionSeedCommitted(ctx, t, secondary, projectID, "c4")

			page1 := list(2, "")
			require.True(t, page1.More)
			require.Equal(t, []string{"c1", "c2"}, transitionListKeys(page1.Objects))

			// resume from the last key of page 1.
			lastKey := string(page1.Objects[len(page1.Objects)-1].ObjectKey)
			page2 := list(2, lastKey)
			require.False(t, page2.More)
			require.Equal(t, []string{"c3", "c4"}, transitionListKeys(page2.Objects))

			// the two pages together cover all keys with no gaps or duplicates.
			all := append(transitionListKeys(page1.Objects), transitionListKeys(page2.Objects)...)
			require.Equal(t, []string{"c1", "c2", "c3", "c4"}, all)
		})
	})
}

func TestTransitionList_ListSegments(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		listSegments := func(streamID uuid.UUID, limit int) metabase.ListSegmentsResult {
			result, err := db.ListSegments(ctx, metabase.ListSegments{
				ProjectID: projectID,
				StreamID:  streamID,
				Limit:     limit,
			})
			require.NoError(t, err)
			return result
		}

		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionListStream(projectID, "seg-p")
			transitionListSeedSegments(ctx, t, primary, stream, 0, 1, 2)

			result := listSegments(stream.StreamID, 10)
			require.False(t, result.More)
			require.Equal(t, []uint32{0, 1, 2}, transitionListSegmentPositions(result.Segments))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionListStream(projectID, "seg-s")
			transitionListSeedSegments(ctx, t, secondary, stream, 0, 1, 2)

			result := listSegments(stream.StreamID, 10)
			require.False(t, result.More)
			require.Equal(t, []uint32{0, 1, 2}, transitionListSegmentPositions(result.Segments))
		})

		t.Run("both deduped by position and sorted", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// same stream lives in both backends with overlapping positions.
			stream := transitionListStream(projectID, "seg-b")
			transitionListSeedSegments(ctx, t, primary, stream, 0, 1)
			transitionListSeedSegments(ctx, t, secondary, stream, 1, 2)

			result := listSegments(stream.StreamID, 10)
			require.False(t, result.More)
			require.Equal(t, []uint32{0, 1, 2}, transitionListSegmentPositions(result.Segments), "position 1 deduped")
		})

		t.Run("More reflects either backend", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionListStream(projectID, "seg-more")
			transitionListSeedSegments(ctx, t, secondary, stream, 0, 1, 2)

			result := listSegments(stream.StreamID, 1)
			require.True(t, result.More, "secondary has more than the limit")
		})
	})
}

func TestTransitionList_ListStreamPositions(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		listPositions := func(streamID uuid.UUID, limit int) metabase.ListStreamPositionsResult {
			result, err := db.ListStreamPositions(ctx, metabase.ListStreamPositions{
				ProjectID: projectID,
				StreamID:  streamID,
				Limit:     limit,
			})
			require.NoError(t, err)
			return result
		}

		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionListStream(projectID, "pos-p")
			transitionListSeedSegments(ctx, t, primary, stream, 0, 1, 2)

			result := listPositions(stream.StreamID, 10)
			require.False(t, result.More)
			require.Equal(t, []uint32{0, 1, 2}, transitionListStreamPositions(result.Segments))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionListStream(projectID, "pos-s")
			transitionListSeedSegments(ctx, t, secondary, stream, 0, 1, 2)

			result := listPositions(stream.StreamID, 10)
			require.False(t, result.More)
			require.Equal(t, []uint32{0, 1, 2}, transitionListStreamPositions(result.Segments))
		})

		t.Run("both deduped by position and sorted", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionListStream(projectID, "pos-b")
			transitionListSeedSegments(ctx, t, primary, stream, 0, 1)
			transitionListSeedSegments(ctx, t, secondary, stream, 1, 2)

			result := listPositions(stream.StreamID, 10)
			require.False(t, result.More)
			require.Equal(t, []uint32{0, 1, 2}, transitionListStreamPositions(result.Segments), "position 1 deduped")
		})

		t.Run("More reflects either backend", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionListStream(projectID, "pos-more")
			transitionListSeedSegments(ctx, t, secondary, stream, 0, 1, 2)

			result := listPositions(stream.StreamID, 1)
			require.True(t, result.More, "secondary has more than the limit")
		})
	})
}

func TestTransitionList_ObjectIterator(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// drain reads all entries from the merging iterator opened on the
		// transition adapter.
		drain := func() []metabase.ObjectEntry {
			it, err := db.ChooseAdapter(projectID).ObjectIterator(ctx, metabase.ObjectIteratorOptions{
				ProjectID:             projectID,
				BucketName:            "bucket",
				Recursive:             true,
				Delimiter:             metabase.Delimiter,
				BatchSize:             10,
				Mode:                  metabase.ObjectIteratorModeAllVersionsDescending,
				IncludeSystemMetadata: true,
			})
			require.NoError(t, err)
			defer func() { require.NoError(t, it.Close()) }()

			var entries []metabase.ObjectEntry
			var entry metabase.ObjectEntry
			for {
				ok, err := it.Next(ctx, &entry)
				require.NoError(t, err)
				if !ok {
					break
				}
				entries = append(entries, entry)
			}
			return entries
		}

		t.Run("empty both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			require.Empty(t, drain())
		})

		t.Run("interleaved keys emitted in sorted order", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "a")
			transitionSeedCommitted(ctx, t, secondary, projectID, "b")
			transitionSeedCommitted(ctx, t, primary, projectID, "c")
			transitionSeedCommitted(ctx, t, secondary, projectID, "d")

			require.Equal(t, []string{"a", "b", "c", "d"}, transitionListKeys(drain()))
		})

		t.Run("same key in both emitted once, primary wins", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			primaryStream := transitionSeedCommitted(ctx, t, primary, projectID, "dup")
			transitionSeedCommitted(ctx, t, secondary, projectID, "dup")

			entries := drain()
			require.Len(t, entries, 1)
			require.Equal(t, "dup", string(entries[0].ObjectKey))
			require.Equal(t, primaryStream.StreamID, entries[0].StreamID, "primary's stream must win")
		})

		t.Run("one side exhausts first, other drains", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// primary holds only the earliest key; secondary holds the rest,
			// so primary exhausts first and secondary must keep draining.
			transitionSeedCommitted(ctx, t, primary, projectID, "a")
			transitionSeedCommitted(ctx, t, secondary, projectID, "b")
			transitionSeedCommitted(ctx, t, secondary, projectID, "c")
			transitionSeedCommitted(ctx, t, secondary, projectID, "d")

			require.Equal(t, []string{"a", "b", "c", "d"}, transitionListKeys(drain()))
		})

		t.Run("only primary populated", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "x")
			transitionSeedCommitted(ctx, t, primary, projectID, "y")

			require.Equal(t, []string{"x", "y"}, transitionListKeys(drain()))
		})

		t.Run("only secondary populated", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, secondary, projectID, "x")
			transitionSeedCommitted(ctx, t, secondary, projectID, "y")

			require.Equal(t, []string{"x", "y"}, transitionListKeys(drain()))
		})
	})
}

func TestTransitionList_ListVerifySegments(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		defer metabasetest.DeleteAll{}.Check(ctx, t, db)

		// ListVerifySegments only returns remote segments (inline_data IS NULL AND
		// remote_alias_pieces IS NOT NULL), so seed remote segments here.
		primaryStream := transitionListStream(projectID, "verify-p")
		secondaryStream := transitionListStream(projectID, "verify-s")
		transitionListSeedRemoteSegments(ctx, t, primary, primaryStream, 0, 1)
		transitionListSeedRemoteSegments(ctx, t, secondary, secondaryStream, 0)

		// exercise the transition adapter's union directly (the DB-level method
		// fans out over all adapters and would double-count both backends).
		segments, err := db.ChooseAdapter(projectID).ListVerifySegments(ctx, metabase.ListVerifySegments{
			Limit: 100,
		})
		require.NoError(t, err)

		streamIDs := map[uuid.UUID]int{}
		for _, s := range segments {
			streamIDs[s.StreamID]++
		}
		require.Equal(t, 2, streamIDs[primaryStream.StreamID], "both primary segments delivered")
		require.Equal(t, 1, streamIDs[secondaryStream.StreamID], "secondary segment delivered")
	})
}

func TestTransitionList_ListBucketStreamIDs(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		defer metabasetest.DeleteAll{}.Check(ctx, t, db)

		primaryStream := transitionSeedCommitted(ctx, t, primary, projectID, "bsi-p")
		secondaryStream := transitionSeedCommitted(ctx, t, secondary, projectID, "bsi-s")

		var collected []uuid.UUID
		// exercise the transition adapter's union directly (the DB-level method
		// fans out over all adapters and would double-count both backends).
		err := db.ChooseAdapter(projectID).ListBucketStreamIDs(ctx, metabase.ListBucketStreamIDs{
			Bucket: metabase.BucketLocation{ProjectID: projectID, BucketName: "bucket"},
			Limit:  100,
		}, func(ctx context.Context, streamIDs []uuid.UUID) error {
			collected = append(collected, streamIDs...)
			return nil
		})
		require.NoError(t, err)

		require.Contains(t, collected, primaryStream.StreamID)
		require.Contains(t, collected, secondaryStream.StreamID)
	})
}
