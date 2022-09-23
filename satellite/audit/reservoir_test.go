// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"
	"testing"
	"time"

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

func TestReservoirBias(t *testing.T) {
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
