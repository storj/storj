// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

func TestReservoir(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().Unix()))

	for size := range 4 {
		t.Run(fmt.Sprintf("size %d", size), func(t *testing.T) {
			samples := []rangedloop.Segment{}
			for i := range size {
				samples = append(samples, makeSegment(i))
			}

			// If we sample N segments, less than the max, we should record all N
			r := NewReservoir(size)
			for _, sample := range samples {
				r.Sample(rng, sample)
			}
			require.Equal(t, toAuditSegments(samples...), r.Segments())
			require.Len(t, r.Keys(), len(samples))
		})
	}
}

func TestReservoirMerge(t *testing.T) {
	t.Run("merge successful", func(t *testing.T) {
		// Use a fixed rng so we get deterministic sampling results.
		segments := []rangedloop.Segment{
			makeSegment(0), makeSegment(1), makeSegment(2),
			makeSegment(3), makeSegment(4), makeSegment(5),
		}
		rng := rand.New(rand.NewSource(999))
		r1 := NewReservoir(3)
		r1.Sample(rng, segments[0])
		r1.Sample(rng, segments[1])
		r1.Sample(rng, segments[2])

		r2 := NewReservoir(3)
		r2.Sample(rng, segments[3])
		r2.Sample(rng, segments[4])
		r2.Sample(rng, segments[5])

		err := r1.Merge(r2)
		require.NoError(t, err)

		// Segments should contain a cross section from r1 and r2. If the rng
		// changes, this result will likely change too since that will affect
		// the keys. and therefore how they are merged.
		require.Equal(t, toAuditSegments(
			segments[5],
			segments[1],
			segments[2],
		), r1.Segments())
	})

	t.Run("mismatched size", func(t *testing.T) {
		r1 := NewReservoir(2)
		r2 := NewReservoir(1)
		err := r1.Merge(r2)
		require.EqualError(t, err, "cannot merge: mismatched size: expected 2 but got 1")
	})

}

func TestReservoirWeights(t *testing.T) {
	var weight10StreamID = testrand.UUID()
	var weight5StreamID = testrand.UUID()
	var weight2StreamID = testrand.UUID()
	var weight1StreamID = testrand.UUID()
	streamIDCountsMap := map[uuid.UUID]int{
		weight10StreamID: 0,
		weight5StreamID:  0,
		weight2StreamID:  0,
		weight1StreamID:  0,
	}

	segments := []rangedloop.Segment{
		{
			StreamID:      weight10StreamID,
			Position:      metabase.SegmentPosition{},
			ExpiresAt:     nil,
			EncryptedSize: 10,
		},
		{
			StreamID:      weight5StreamID,
			Position:      metabase.SegmentPosition{},
			ExpiresAt:     nil,
			EncryptedSize: 5,
		},
		{
			StreamID:      weight2StreamID,
			Position:      metabase.SegmentPosition{},
			ExpiresAt:     nil,
			EncryptedSize: 2,
		},
		{
			StreamID:      weight1StreamID,
			Position:      metabase.SegmentPosition{},
			ExpiresAt:     nil,
			EncryptedSize: 1,
		},
	}

	// run a large number of times in loop for bias to show up
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 1; i < 100000; i++ {
		r := NewReservoir(3)

		for _, segment := range segments {
			r.Sample(rng, segment)
		}

		for _, segment := range r.Segments() {
			streamIDCountsMap[segment.StreamID]++
		}

		// shuffle the segments order after each result
		rng.Shuffle(len(segments),
			func(i, j int) {
				segments[i], segments[j] = segments[j], segments[i]
			})
	}
	require.Greater(t, streamIDCountsMap[weight10StreamID], streamIDCountsMap[weight5StreamID])
	require.Greater(t, streamIDCountsMap[weight5StreamID], streamIDCountsMap[weight2StreamID])
	require.Greater(t, streamIDCountsMap[weight2StreamID], streamIDCountsMap[weight1StreamID])
}

// Sample many segments, with equal weight, uniformly distributed, and in order,
// through the reservoir. Expect that elements show up in the result set with
// equal chance, whether they were inserted near the beginning of the list or
// near the end.
func TestReservoirBias(t *testing.T) {
	const (
		reservoirSize = 3
		useBits       = 14
		numSegments   = 1 << useBits
		weight        = 100000 // any number; same for all segments
		numRounds     = 1000
	)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	numsSelected := make([]uint64, numRounds*reservoirSize)

	for r := 0; r < numRounds; r++ {
		res := NewReservoir(reservoirSize)
		for n := 0; n < numSegments; n++ {
			seg := rangedloop.Segment{
				EncryptedSize: weight,
			}
			binary.BigEndian.PutUint64(seg.StreamID[0:8], uint64(n)<<(64-useBits))
			res.Sample(rng, seg)
		}
		for i, seg := range res.Segments() {
			num := binary.BigEndian.Uint64(seg.StreamID[0:8]) >> (64 - useBits)
			numsSelected[r*reservoirSize+i] = num
		}
	}

	sort.Sort(uint64Slice(numsSelected))

	// this delta is probably way too generous. but, the A-Chao
	// implementation failed the test with this value, so maybe it's fine.
	delta := float64(numSegments / 8)
	quartile0 := numsSelected[len(numsSelected)*0/4]
	assert.InDelta(t, numSegments*0/4, quartile0, delta)
	quartile1 := numsSelected[len(numsSelected)*1/4]
	assert.InDelta(t, numSegments*1/4, quartile1, delta)
	quartile2 := numsSelected[len(numsSelected)*2/4]
	assert.InDelta(t, numSegments*2/4, quartile2, delta)
	quartile3 := numsSelected[len(numsSelected)*3/4]
	assert.InDelta(t, numSegments*3/4, quartile3, delta)
	quartile4 := numsSelected[len(numsSelected)-1]
	assert.InDelta(t, numSegments*4/4, quartile4, delta)
}

type uint64Slice []uint64

func (us uint64Slice) Len() int           { return len(us) }
func (us uint64Slice) Swap(i, j int)      { us[i], us[j] = us[j], us[i] }
func (us uint64Slice) Less(i, j int) bool { return us[i] < us[j] }

func makeSegment(n int) rangedloop.Segment {
	return rangedloop.Segment{
		StreamID:      uuid.UUID{0: byte(n)},
		EncryptedSize: int32(n * 1000),
	}
}

func toAuditSegments(segments ...rangedloop.Segment) []Segment {
	auditSegments := make([]Segment, len(segments))
	for i, segment := range segments {
		auditSegments[i] = NewSegment(segment)
	}
	return auditSegments
}
