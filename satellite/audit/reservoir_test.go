// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"encoding/binary"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
)

func TestReservoir(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	r := NewReservoir(3)

	seg := func(n byte) *segmentloop.Segment { return &segmentloop.Segment{StreamID: uuid.UUID{0: n}} }

	// if we sample 3 segments, we should record all 3
	r.Sample(rng, seg(1))
	r.Sample(rng, seg(2))
	r.Sample(rng, seg(3))

	require.Equal(t, r.Segments[:], []segmentloop.Segment{*seg(1), *seg(2), *seg(3)})
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

	segments := []*segmentloop.Segment{
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

		for _, segment := range r.Segments {
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
			seg := segmentloop.Segment{
				EncryptedSize: weight,
			}
			binary.BigEndian.PutUint64(seg.StreamID[0:8], uint64(n)<<(64-useBits))
			res.Sample(rng, &seg)
		}
		for i, seg := range res.Segments {
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
