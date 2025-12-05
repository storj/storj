// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/metabase/rangedloop"
)

type in struct {
	streamIDs []string
	nRanges   int
	batchSize int
}

type expected struct {
	nBatches  int
	nSegments int
}

func TestMetabaseSegementProvider(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		inouts := []struct {
			in       in
			expected expected
		}{
			{
				in: in{
					streamIDs: []string{},
					nRanges:   1,
					batchSize: 2,
				},
				expected: expected{
					nBatches:  0,
					nSegments: 0,
				},
			},
			{
				in: in{
					streamIDs: []string{
						"00000000-0000-0000-0000-000000000001",
						"00000000-0000-0000-0000-000000000002",
					},
					nRanges:   2,
					batchSize: 2,
				},
				expected: expected{
					nBatches:  1,
					nSegments: 2,
				},
			},
			{
				in: in{
					streamIDs: []string{
						"00000000-0000-0000-0000-000000000001",
						"f0000000-0000-0000-0000-000000000001",
					},
					nRanges:   2,
					batchSize: 2,
				},
				expected: expected{
					nBatches:  2,
					nSegments: 2,
				},
			},
			{
				in: in{
					streamIDs: []string{
						"00000000-0000-0000-0000-000000000001",
						"00000000-0000-0000-0000-000000000002",
						"f0000000-0000-0000-0000-000000000001",
						"f0000000-0000-0000-0000-000000000002",
					},
					nRanges:   2,
					batchSize: 1,
				},
				expected: expected{
					nBatches:  4,
					nSegments: 4,
				},
			},
		}

		for _, inout := range inouts {
			runTest(ctx, t, db, inout.in, inout.expected)
		}
	})
}

func runTest(ctx *testcontext.Context, t *testing.T, db *metabase.DB, in in, expected expected) {
	defer metabasetest.DeleteAll{}.Check(ctx, t, db)
	for _, streamID := range in.streamIDs {
		u, err := uuid.FromString(streamID)
		require.NoError(t, err)
		createSegment(ctx, t, db, u)
	}

	provider := rangedloop.NewMetabaseRangeSplitter(zap.NewNop(), db, rangedloop.Config{
		AsOfSystemInterval:   -1 * time.Microsecond,
		SpannerStaleInterval: -1 * time.Microsecond,
		BatchSize:            in.batchSize,
	})
	ranges, err := provider.CreateRanges(ctx, in.nRanges, in.batchSize)
	require.NoError(t, err)

	nBatches := 0
	nSegments := 0
	for _, r := range ranges {
		err = r.Iterate(ctx, func(segments []rangedloop.Segment) error {
			nBatches++
			nSegments += len(segments)
			return nil
		})
		require.NoError(t, err)
	}

	require.Equal(t, expected.nSegments, nSegments)
	require.Equal(t, expected.nBatches, nBatches)
}

func createSegment(ctx *testcontext.Context, t testing.TB, db *metabase.DB, streamID uuid.UUID) {
	obj := metabasetest.RandObjectStream()
	obj.StreamID = streamID

	pos := metabase.SegmentPosition{Part: 0, Index: 0}
	data := testrand.Bytes(32)
	encryptedKey := testrand.Bytes(32)
	encryptedKeyNonce := testrand.Bytes(32)

	metabasetest.BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   metabasetest.DefaultEncryption,
		},
	}.Check(ctx, t, db)

	metabasetest.CommitInlineSegment{
		Opts: metabase.CommitInlineSegment{
			ObjectStream: obj,
			Position:     pos,

			EncryptedKey:      encryptedKey,
			EncryptedKeyNonce: encryptedKeyNonce,

			PlainSize:   512,
			PlainOffset: 0,

			InlineData: data,
		},
	}.Check(ctx, t, db)

	metabasetest.CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
		},
	}.Check(ctx, t, db)
}
