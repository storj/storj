// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

func TestPieceCountHistogramProcess(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// 3 participating nodes and 1 node which is not participating anymore.
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()
	goneNode := testrand.NodeID()

	observer := &PieceCountHistogram{
		log:       zaptest.NewLogger(t),
		config:    PieceCountHistogramConfig{LogBuckets: true},
		histogram: make(map[PieceCountBucket]*PieceCountHistogramStats),
		participatingNodes: map[storj.NodeID]struct{}{
			node1: {},
			node2: {},
			node3: {},
			// goneNode intentionally absent
		},
		seenBuckets: make(map[PieceCountBucket]struct{}),
	}

	rs := func(required int16) storj.RedundancyScheme {
		return storj.RedundancyScheme{
			ShareSize:      10,
			RequiredShares: required,
			RepairShares:   required + 6,
			OptimalShares:  required + 10,
			TotalShares:    required + 20,
		}
	}

	// encryptedSize 290 with shareSize 10 gives pieceSize 20 for both rs_min 29 and
	// rs_min 16; encryptedSize 2900 with rs_min 29 gives pieceSize 110. Two different
	// piece sizes inside the same bucket keep the byte sums honest.
	newSegment := func(placement storj.PlacementConstraint, required int16, encryptedSize int32, nodes ...storj.NodeID) rangedloop.Segment {
		pieces := make(metabase.Pieces, len(nodes))
		for i, node := range nodes {
			pieces[i] = metabase.Piece{Number: uint16(i), StorageNode: node}
		}
		return rangedloop.Segment{
			StreamID:      testrand.UUID(),
			RootPieceID:   testrand.PieceID(),
			EncryptedSize: encryptedSize,
			Placement:     placement,
			Redundancy:    rs(required),
			Pieces:        pieces,
		}
	}

	expired := time.Now().Add(-time.Hour)
	expiredSegment := newSegment(0, 29, 290, node1, node2)
	expiredSegment.ExpiresAt = &expired

	segments := []rangedloop.Segment{
		// placement 0, rs_min 29, 3 participating pieces out of 4. pieceSize 20.
		newSegment(0, 29, 290, node1, node2, node3, goneNode),
		// same bucket as above, but all 3 pieces participating, and pieceSize 110.
		newSegment(0, 29, 2900, node1, node2, node3),
		// placement 0, rs_min 29, only 2 participating pieces: different bucket.
		newSegment(0, 29, 290, node1, node2),
		// placement 0, different rs_min: different bucket.
		newSegment(0, 16, 290, node1, node2, node3),
		// different placement: different bucket.
		newSegment(12, 29, 290, node1, node2, node3),
		// inline segment, ignored.
		{StreamID: testrand.UUID()},
		// expired segment, ignored.
		expiredSegment,
	}

	partial, err := observer.Fork(ctx)
	require.NoError(t, err)

	require.NoError(t, partial.Process(ctx, segments))
	require.NoError(t, observer.Join(ctx, partial))
	require.NoError(t, observer.Finish(ctx))

	// placement 0 / rs_min 29 / 3 participating pieces: two segments, 7 pieces
	// in total (4 + 3), 6 of them on participating nodes.
	stats := observer.histogram[PieceCountBucket{Placement: 0, RequiredShares: 29, ParticipatingPieces: 3}]
	require.NotNil(t, stats)
	assert.Equal(t, int64(2), stats.SegmentCount)
	assert.Equal(t, int64(7), stats.PieceCount)
	assert.Equal(t, int64(6), stats.ParticipatingPieceCount)
	// 4 pieces * 20 bytes + 3 pieces * 110 bytes
	assert.Equal(t, int64(410), stats.PieceBytes)
	// 3 pieces * 20 bytes + 3 pieces * 110 bytes
	assert.Equal(t, int64(390), stats.ParticipatingPieceBytes)

	stats = observer.histogram[PieceCountBucket{Placement: 0, RequiredShares: 29, ParticipatingPieces: 2}]
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.SegmentCount)
	assert.Equal(t, int64(2), stats.PieceCount)
	assert.Equal(t, int64(2), stats.ParticipatingPieceCount)
	assert.Equal(t, int64(40), stats.PieceBytes)
	assert.Equal(t, int64(40), stats.ParticipatingPieceBytes)

	stats = observer.histogram[PieceCountBucket{Placement: 0, RequiredShares: 16, ParticipatingPieces: 3}]
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.SegmentCount)
	assert.Equal(t, int64(60), stats.PieceBytes)

	stats = observer.histogram[PieceCountBucket{Placement: 12, RequiredShares: 29, ParticipatingPieces: 3}]
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.SegmentCount)
	assert.Equal(t, int64(60), stats.PieceBytes)

	// inline and expired segments must not create buckets.
	assert.Len(t, observer.histogram, 4)
}

func TestPieceCountHistogramJoinMergesForks(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	node := testrand.NodeID()

	observer := &PieceCountHistogram{
		log:                zaptest.NewLogger(t),
		histogram:          make(map[PieceCountBucket]*PieceCountHistogramStats),
		participatingNodes: map[storj.NodeID]struct{}{node: {}},
		seenBuckets:        make(map[PieceCountBucket]struct{}),
	}

	segment := rangedloop.Segment{
		StreamID:      testrand.UUID(),
		RootPieceID:   testrand.PieceID(),
		EncryptedSize: 290,
		Redundancy: storj.RedundancyScheme{
			ShareSize:      10,
			RequiredShares: 29,
			RepairShares:   35,
			OptimalShares:  80,
			TotalShares:    110,
		},
		Pieces: metabase.Pieces{{Number: 0, StorageNode: node}},
	}

	for range 3 {
		partial, err := observer.Fork(ctx)
		require.NoError(t, err)
		require.NoError(t, partial.Process(ctx, []rangedloop.Segment{segment}))
		require.NoError(t, observer.Join(ctx, partial))
	}

	stats := observer.histogram[PieceCountBucket{Placement: 0, RequiredShares: 29, ParticipatingPieces: 1}]
	require.NotNil(t, stats)
	assert.Equal(t, int64(3), stats.SegmentCount)
	assert.Equal(t, int64(3), stats.PieceCount)
	assert.Equal(t, int64(3), stats.ParticipatingPieceCount)
	assert.Equal(t, int64(60), stats.PieceBytes)
	assert.Equal(t, int64(60), stats.ParticipatingPieceBytes)
}

// TestPieceCountHistogramFinishZeroesStaleBuckets checks that a bucket present
// in one run and missing in the next is still emitted (with zeroes) so its
// previous non-zero value does not keep being reported forever.
func TestPieceCountHistogramFinishZeroesStaleBuckets(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	observer := &PieceCountHistogram{
		log:                zaptest.NewLogger(t),
		histogram:          make(map[PieceCountBucket]*PieceCountHistogramStats),
		participatingNodes: map[storj.NodeID]struct{}{},
		seenBuckets:        make(map[PieceCountBucket]struct{}),
	}

	first := PieceCountBucket{Placement: 0, RequiredShares: 29, ParticipatingPieces: 5}
	second := PieceCountBucket{Placement: 0, RequiredShares: 29, ParticipatingPieces: 4}

	observer.histogram[first] = &PieceCountHistogramStats{SegmentCount: 7}
	require.NoError(t, observer.Finish(ctx))
	assert.Contains(t, observer.seenBuckets, first)

	// Simulate a fresh loop with a different bucket populated. Finish must
	// zero out the previously seen `first` bucket as well.
	observer.histogram = map[PieceCountBucket]*PieceCountHistogramStats{
		second: {SegmentCount: 3},
	}
	require.NoError(t, observer.Finish(ctx))
	assert.Contains(t, observer.seenBuckets, first)
	assert.Contains(t, observer.seenBuckets, second)
}
